// The render service owns pipeline stages 3 (RESTAGE) and 5 (ANIMATE) — the
// commodity image-gen + image-to-video calls. Today it runs stage 3 one-shot from
// the CLI so we can produce a real "after" from a hero frame and feed it into the
// director VERIFY gate, closing the RESTAGE→VERIFY loop on real images.
//
//	go run ./services/render restage --in old-kitchen.jpg --out after.jpg \
//	    --room kitchen --style "warm Nordic minimalism, oak and off-white"
//
// Requires an image-gen key (GEMINI_API_KEY or GOOGLE_API_KEY) — Anthropic does not
// generate images, so this is a separate provider from the VERIFY gate.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andreas-lindfalk/latentframe/pkg/env"
	"github.com/andreas-lindfalk/latentframe/services/render/internal/imageedit"
	"github.com/andreas-lindfalk/latentframe/services/render/internal/video"
)

func main() {
	log.SetFlags(0)
	env.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("usage: render <restage|animate> ...")
	}
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" {
		log.Fatal("no key set. Put GEMINI_API_KEY=... in .env (get one at aistudio.google.com/apikey)")
	}

	switch os.Args[1] {
	case "restage":
		restage(os.Args[2:])
	case "animate":
		animate(os.Args[2:])
	default:
		log.Fatalf("unknown command %q (use: restage | animate)", os.Args[1])
	}
}

// restage runs stage 3: hero frame → re-staged "after" still.
func restage(args []string) {
	fs := flag.NewFlagSet("restage", flag.ExitOnError)
	in := fs.String("in", "", "path to the input hero frame (required)")
	out := fs.String("out", "", "path to write the re-staged 'after' image (required)")
	room := fs.String("room", "", "room label, e.g. 'kitchen'")
	style := fs.String("style", "", "target design style (defaults to a warm modern minimalism)")
	_ = fs.Parse(args)
	if *in == "" || *out == "" {
		fs.Usage()
		log.Fatal("\n--in and --out are required")
	}

	editor, err := imageedit.NewGemini()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("re-staging %s → %s (style: %q)", *in, *out, styleOrDefault(*style))
	written, err := imageedit.RestageFile(context.Background(), editor, *in, *out, *room, *style)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ wrote %s\n", written)
	fmt.Println("next: director verify --before", *in, "--after", written)
}

// animate runs stage 5: gate-approved still → short reveal clip (image-to-video).
func animate(args []string) {
	fs := flag.NewFlagSet("animate", flag.ExitOnError)
	in := fs.String("in", "", "path to the approved 'after' still (required)")
	out := fs.String("out", "", "path to write the .mp4 clip (required)")
	prompt := fs.String("prompt", "", "camera-motion prompt (defaults to a slow dolly-in)")
	timeout := fs.Duration("timeout", 10*time.Minute, "max time to wait for the clip")
	_ = fs.Parse(args)
	if *in == "" || *out == "" {
		fs.Usage()
		log.Fatal("\n--in and --out are required")
	}

	veo, err := video.NewVeo()
	if err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}
	motion := *prompt
	if motion == "" {
		motion = video.DefaultMotion
	}
	log.Printf("animating %s → %s (this takes ~1 min)…", *in, *out)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	mp4, err := veo.Animate(ctx, raw, http.DetectContentType(raw), motion)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, mp4, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ wrote %s (%d KB)\n", *out, len(mp4)/1024)
}

func styleOrDefault(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(default warm modern minimalism)"
	}
	return s
}
