// The ingest service owns pipeline stage 1 (INGEST): a walkthrough video →
// per-room hero frames (+ optional narration). It samples frames densely, scores
// each for sharpness, segments the walk (scene cuts, or equal time-bins for
// continuous footage), and picks the sharpest frame per segment as that room's hero.
// The manifest it writes is the basis for the pipeline's Property.Rooms.
//
//	go run ./services/ingest --video walkthrough.mov --transcribe --lang es
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/andreas-lindfalk/latentframe/pkg/media"
)

func main() {
	log.SetFlags(0)

	video := flag.String("video", "", "path to the walkthrough video (required)")
	outDir := flag.String("out", "out", "output directory")
	fps := flag.Float64("fps", 2, "candidate frames sampled per second")
	rooms := flag.Int("rooms", 6, "max hero frames to select (and bins when there are no scene cuts)")
	sceneTh := flag.Float64("scene-threshold", 0.4, "scene-change sensitivity (0..1); lower = more cuts")
	transcribe := flag.Bool("transcribe", false, "also extract audio and run Whisper transcription")
	lang := flag.String("lang", "", "Whisper language hint (e.g. 'es', 'en'); empty = auto-detect")
	flag.Parse()

	if *video == "" {
		flag.Usage()
		log.Fatal("\n--video is required")
	}
	if _, err := os.Stat(*video); err != nil {
		log.Fatalf("video not found: %v", err)
	}

	if err := run(*video, *outDir, *fps, *rooms, *sceneTh, *transcribe, *lang); err != nil {
		log.Fatal(err)
	}
}

type scored struct {
	path  string
	t     float64 // seconds
	sharp float64
}

type roomEntry struct {
	Index     int     `json:"index"`
	Hero      string  `json:"hero"`
	TimeSec   float64 `json:"time_s"`
	Sharpness float64 `json:"sharpness"`
}

type manifest struct {
	Video      string          `json:"video"`
	FPSSampled float64         `json:"fps_sampled"`
	Rooms      []roomEntry     `json:"rooms"`
	Transcript []media.Segment `json:"transcript,omitempty"`
}

func run(video, outDir string, fps float64, maxRooms int, sceneTh float64, transcribe bool, lang string) error {
	ctx := context.Background()
	ff := media.ExecFFmpeg{}
	framesDir := filepath.Join(outDir, "frames")
	heroesDir := filepath.Join(outDir, "heroes")
	for _, d := range []string{framesDir, heroesDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	if fps <= 0 { // else t := i/fps and dur := len/fps below divide by zero; ExtractFrames would itself fall back to 2, so match it here
		log.Printf("  --fps %g is non-positive; using 2 fps", fps)
		fps = 2
	}
	log.Printf("stage 1 · sampling candidate frames at %g fps → %s", fps, framesDir)
	if err := ff.ExtractFrames(ctx, video, framesDir, fps, 1280); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(framesDir, "frame-*.jpg"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no frames extracted from %s", video)
	}

	log.Printf("  scoring sharpness of %d candidate frames…", len(files))
	frames := make([]scored, 0, len(files))
	for i, p := range files {
		s, err := media.SharpnessJPEG(p)
		if err != nil {
			return fmt.Errorf("score %s: %w", p, err)
		}
		frames = append(frames, scored{path: p, t: float64(i) / fps, sharp: s})
	}
	dur := float64(len(files)-1) / fps // frames sit at 0,1/fps,…,(len-1)/fps, so the covered span is (len-1)/fps, not len/fps (len>=1 checked above)

	cuts, err := ff.SceneTimestamps(ctx, video, sceneTh)
	if err != nil {
		return err
	}
	boundaries := buildBoundaries(cuts, dur, maxRooms)
	log.Printf("  segmentation: %d scene cut(s) → %d segment(s)", len(cuts), len(boundaries))

	heroes := pickHeroes(frames, boundaries, dur, maxRooms)
	log.Printf("  selected %d hero frame(s):", len(heroes))

	man := manifest{Video: video, FPSSampled: fps}
	for i, h := range heroes {
		dst := filepath.Join(heroesDir, fmt.Sprintf("room-%02d.jpg", i+1))
		if err := copyFile(h.path, dst); err != nil {
			return err
		}
		man.Rooms = append(man.Rooms, roomEntry{Index: i + 1, Hero: dst, TimeSec: h.t, Sharpness: h.sharp})
		log.Printf("    room %02d  t=%5.1fs  sharpness=%9.1f  → %s", i+1, h.t, h.sharp, dst)
	}

	if transcribe {
		audioPath := filepath.Join(outDir, "audio.mp3")
		log.Printf("stage 2 · transcribing narration…")
		if err := ff.ExtractAudio(ctx, video, audioPath); err != nil {
			return err
		}
		segs, err := media.NewWhisperCLITranscriber(lang).Transcribe(ctx, audioPath)
		if err != nil {
			return err
		}
		man.Transcript = segs
		log.Printf("  ✓ %d narration segments", len(segs))
	}

	manPath := filepath.Join(outDir, "manifest.json")
	if err := writeJSON(manPath, man); err != nil {
		return err
	}
	log.Printf("✓ wrote %s (feed each room's hero into `director understand` / `render restage`)", manPath)
	return nil
}

// buildBoundaries returns segment start times. Prefer scene cuts; fall back to
// maxRooms equal time-bins for continuous footage with no cuts.
func buildBoundaries(cuts []float64, dur float64, maxRooms int) []float64 {
	var valid []float64
	for _, c := range cuts {
		if c > 0.5 && c < dur-0.5 {
			valid = append(valid, c)
		}
	}
	sort.Float64s(valid)
	if len(valid) >= 1 {
		return append([]float64{0}, valid...)
	}
	k := maxRooms
	if k < 1 {
		k = 1
	}
	b := make([]float64, k)
	for i := 0; i < k; i++ {
		b[i] = float64(i) * dur / float64(k)
	}
	return b
}

// pickHeroes chooses one hero per segment: the sharpest frame within the segment's
// central 60% (avoiding transition frames at the edges, which are often ambiguous or
// motion-blurred). Capped to maxRooms by sharpness, returned in time order.
func pickHeroes(frames []scored, boundaries []float64, dur float64, maxRooms int) []scored {
	sharpestIn := func(lo, hi float64) *scored {
		var best *scored
		for i := range frames {
			f := frames[i]
			if f.t < lo || f.t > hi {
				continue
			}
			if best == nil || f.sharp > best.sharp {
				b := f
				best = &b
			}
		}
		return best
	}

	var heroes []scored
	for i := range boundaries {
		start := boundaries[i]
		end := dur
		if i+1 < len(boundaries) {
			end = boundaries[i+1]
		}
		span := end - start
		best := sharpestIn(start+0.2*span, end-0.2*span) // central 60%
		if best == nil {
			best = sharpestIn(start, end) // fallback: whole segment
		}
		if best != nil {
			heroes = append(heroes, *best)
		}
	}

	if maxRooms > 0 && len(heroes) > maxRooms {
		sort.Slice(heroes, func(i, j int) bool { return heroes[i].sharp > heroes[j].sharp })
		heroes = heroes[:maxRooms]
	}
	sort.Slice(heroes, func(i, j int) bool { return heroes[i].t < heroes[j].t })
	return heroes
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
