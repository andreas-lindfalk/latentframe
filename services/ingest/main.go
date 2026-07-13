// The ingest service owns pipeline stage 1 (INGEST): video → keyframes, audio, and
// transcribed narration. It currently runs one-shot from the CLI to drive the one-room
// hand-made experiment; it will grow a `serve` mode that consumes the job queue.
//
//	go run ./services/ingest --video experiments/one-room/kitchen.mov --transcribe --lang es
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/andreas-lindfalk/latentframe/pkg/media"
)

func main() {
	log.SetFlags(0)

	video := flag.String("video", "", "path to the walkthrough video (required)")
	outDir := flag.String("out", "out", "output directory for extracted assets")
	interval := flag.Int("interval", 3, "keyframe sampling interval in seconds")
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

	if err := run(*video, *outDir, *interval, *transcribe, *lang); err != nil {
		log.Fatal(err)
	}
}

func run(video, outDir string, interval int, transcribe bool, lang string) error {
	ctx := context.Background()
	ff := media.ExecFFmpeg{}

	keyframeDir := filepath.Join(outDir, "keyframes")
	if err := os.MkdirAll(keyframeDir, 0o755); err != nil {
		return fmt.Errorf("create keyframe dir: %w", err)
	}

	log.Printf("stage 1/2 · extracting keyframes (every %ds) → %s", interval, keyframeDir)
	if err := ff.ExtractKeyframes(ctx, video, keyframeDir, interval); err != nil {
		return err
	}
	frames, err := filepath.Glob(filepath.Join(keyframeDir, "frame-*.jpg"))
	if err != nil {
		return err
	}
	log.Printf("  ✓ %d keyframes extracted", len(frames))

	if !transcribe {
		log.Print("\nTip: add --transcribe to also capture the spoken context.")
		return nil
	}

	audioPath := filepath.Join(outDir, "audio.mp3")
	log.Printf("stage 2/2 · extracting audio → %s", audioPath)
	if err := ff.ExtractAudio(ctx, video, audioPath); err != nil {
		return err
	}

	log.Print("  · transcribing with Whisper (this can take a minute) …")
	segments, err := media.NewWhisperCLITranscriber(lang).Transcribe(ctx, audioPath)
	if err != nil {
		return err
	}
	log.Printf("  ✓ %d narration segments\n", len(segments))
	for _, s := range segments {
		log.Printf("  [%6.1fs] %s", float64(s.StartMs)/1000, s.Text)
	}
	return nil
}
