// The walkthrough service is the owner-narrated pipeline, end to end:
//
//	walkthrough video + voice
//	  → INGEST      (heroes per room + Whisper transcript)      [services/ingest]
//	  → pair each room's hero with the narration at its time
//	  → UNDERSTAND  (frame + narration → the owner's vision)     [director understand]
//	  → RESTAGE     (Nano-Banana, gated shell/layout guardrails) [render restage]
//	  → per-room before/after
//
//	walkthrough --video cabin.mov --lang en --bindir ./bin
//
// It shells out to the ingest/director/render binaries (build them first, or pass --bindir),
// so the whole flow stays Go and reuses the real services. i2v + assembly are the next step.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// archClause is the honesty guardrail prepended to UNDERSTAND's vision so the restage keeps
// the real shell/layout even when the owner asks for a big change (their words carry the
// intent; this keeps windows/doors/orientation honest unless they explicitly authorise it).
const archClause = "KEEP THE EXACT ARCHITECTURE and the SAME LEFT-RIGHT LAYOUT — do not mirror, flip or reorganise the room; keep the windows, doors and layout on their original sides; do not invent an outdoor view; never treat a wall painting as a window. Then: "

type segment struct {
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
	Text    string `json:"text"`
}
type room struct {
	Index   int     `json:"index"`
	Hero    string  `json:"hero"`
	TimeSec float64 `json:"time_s"`
}
type manifest struct {
	Rooms      []room    `json:"rooms"`
	Transcript []segment `json:"transcript"`
}

func main() {
	log.SetFlags(0)
	video := flag.String("video", "", "walkthrough video (required)")
	out := flag.String("out", "walkthrough-out", "output dir")
	lang := flag.String("lang", "", "narration language hint (e.g. 'en','sv','es'); empty = auto")
	rooms := flag.Int("rooms", 6, "max rooms to pick from the walk")
	engine := flag.String("engine", "nano-banana", "restage engine")
	context := flag.String("context", "", "property/market context for UNDERSTAND (e.g. 'a cozy Swedish summer cabin')")
	bindir := flag.String("bindir", "", "dir with prebuilt ingest/director/render binaries (else `go run`)")
	flag.Parse()
	if *video == "" {
		flag.Usage()
		log.Fatal("\n--video is required")
	}
	if err := run(*video, *out, *lang, *rooms, *engine, *context, *bindir); err != nil {
		log.Fatal(err)
	}
}

func run(video, out, lang string, maxRooms int, engine, marketContext, bindir string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	bin := func(svc string) []string {
		if bindir != "" {
			return []string{filepath.Join(bindir, svc)}
		}
		return []string{"go", "run", "./services/" + svc}
	}

	// 1. INGEST → heroes + transcript
	log.Printf("① ingest %s …", filepath.Base(video))
	ingArgs := append(bin("ingest"), "--video", video, "--out", filepath.Join(out, "ingest"),
		"--rooms", fmt.Sprint(maxRooms), "--transcribe")
	if lang != "" {
		ingArgs = append(ingArgs, "--lang", lang)
	}
	if o, err := exec.Command(ingArgs[0], ingArgs[1:]...).CombinedOutput(); err != nil {
		return fmt.Errorf("ingest: %v\n%s", err, o)
	}
	var man manifest
	if b, err := os.ReadFile(filepath.Join(out, "ingest", "manifest.json")); err != nil {
		return err
	} else if err := json.Unmarshal(b, &man); err != nil {
		return err
	}
	log.Printf("   %d rooms, %d narration segments", len(man.Rooms), len(man.Transcript))

	// 2. per room: pair narration → UNDERSTAND → RESTAGE
	for _, rm := range man.Rooms {
		narr := pairNarration(man.Transcript, man.Rooms, rm)
		log.Printf("② room %02d (t=%.0fs) narration: %q", rm.Index, rm.TimeSec, truncate(narr, 80))

		// UNDERSTAND: frame + narration → the owner's vision (transform prompt)
		narrFile := filepath.Join(out, fmt.Sprintf("room-%02d.narration.txt", rm.Index))
		_ = os.WriteFile(narrFile, []byte(narr), 0o644)
		uArgs := append(bin("director"), "understand", "--image", rm.Hero, "--print", "prompt")
		if narr != "" {
			uArgs = append(uArgs, "--transcript", narrFile)
		}
		if marketContext != "" {
			uArgs = append(uArgs, "--context", marketContext)
		}
		vision, err := output(uArgs)
		if err != nil {
			return fmt.Errorf("understand room %d: %w", rm.Index, err)
		}
		instruction := archClause + strings.TrimSpace(vision)

		// RESTAGE: Nano-Banana, honest by the prepended guardrail
		after := filepath.Join(out, fmt.Sprintf("room-%02d_after.jpg", rm.Index))
		log.Printf("③ restage room %02d …", rm.Index)
		rArgs := append(bin("render"), "restage", "--engine", engine, "--in", rm.Hero,
			"--out", after, "--prompt", instruction)
		if o, err := exec.Command(rArgs[0], rArgs[1:]...).CombinedOutput(); err != nil {
			return fmt.Errorf("restage room %d: %v\n%s", rm.Index, err, o)
		}
	}
	log.Printf("✓ done → %s (per-room heroes in ingest/heroes, afters as room-NN_after.jpg)", out)
	return nil
}

// pairNarration gives a room the transcript spoken while the camera was on it — segments
// whose midpoint falls in the room's slice of the timeline (midpoint-to-midpoint between
// adjacent room hero times).
func pairNarration(segs []segment, rooms []room, rm room) string {
	times := make([]float64, len(rooms))
	idx := 0
	for i, r := range rooms {
		times[i] = r.TimeSec
		if r.Index == rm.Index {
			idx = i
		}
	}
	sort.Float64s(times)
	lo, hi := -1e9, 1e9
	for i, t := range times {
		if t == rm.TimeSec {
			if i > 0 {
				lo = (times[i-1] + t) / 2
			}
			if i+1 < len(times) {
				hi = (t + times[i+1]) / 2
			}
			break
		}
	}
	_ = idx
	var parts []string
	for _, s := range segs {
		mid := float64(s.StartMs+s.EndMs) / 2000.0
		if mid >= lo && mid < hi {
			parts = append(parts, strings.TrimSpace(s.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func output(args []string) (string, error) {
	o, err := exec.Command(args[0], args[1:]...).Output()
	return strings.TrimSpace(string(o)), err
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
