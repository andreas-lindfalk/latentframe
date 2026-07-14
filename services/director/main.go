// The director service owns pipeline stages 2 (UNDERSTAND) and 4 (VERIFY) — the
// Claude art-director + honesty gate that are Latent Frame's moat.
//
//	director understand --image kitchen.jpg --room kitchen        # stage 2: design brief
//	director verify --before old.jpg --after restaged.png         # stage 4: honesty gate
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/andreas-lindfalk/latentframe/pkg/env"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/understand"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/verify"
)

func main() {
	log.SetFlags(0)
	env.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("usage: director <understand|verify> ...")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set (put it in .env or export it)")
	}

	switch os.Args[1] {
	case "understand":
		understandCmd(os.Args[2:])
	case "verify":
		verifyCmd(os.Args[2:])
	default:
		log.Fatalf("unknown command %q (use: understand | verify)", os.Args[1])
	}
}

// verifyCmd runs stage 4: the honesty gate on a before/after pair.
func verifyCmd(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	before := fs.String("before", "", "path to the BEFORE image (real current photo) (required)")
	after := fs.String("after", "", "path to the AFTER image (proposed re-staged render) (required)")
	room := fs.String("room", "", "optional room label, e.g. 'kitchen'")
	_ = fs.Parse(args)
	if *before == "" || *after == "" {
		fs.Usage()
		log.Fatal("\n--before and --after are required")
	}

	verdict, err := verify.NewGate().VerifyPair(context.Background(), *before, *after, *room)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("architecture preserved : %v\n", verdict.ArchitecturePreserved)
	fmt.Printf("believable             : %v\n", verdict.Believable)
	fmt.Printf("reason                 : %s\n", verdict.Reason)
	fmt.Println(strings.Repeat("─", 40))
	if verdict.OK() {
		fmt.Println("✓ PASS — honest and believable, would ship")
		return
	}
	fmt.Println("✗ FAIL — would be rejected and regenerated")
	os.Exit(1)
}

// understandCmd runs stage 2: Claude writes the design brief for a room.
func understandCmd(args []string) {
	fs := flag.NewFlagSet("understand", flag.ExitOnError)
	image := fs.String("image", "", "path to the room's hero frame (required)")
	room := fs.String("room", "", "room label, e.g. 'kitchen'")
	transcript := fs.String("transcript", "", "optional path to spoken narration (.txt/.srt/.vtt)")
	marketCtx := fs.String("context", "premium Spanish Costa Blanca property marketed to international buyers", "market context")
	printOnly := fs.String("print", "brief", "output: 'brief' (full), 'prompt' (just the transform prompt, for piping), or 'style'")
	_ = fs.Parse(args)
	if *image == "" {
		fs.Usage()
		log.Fatal("\n--image is required")
	}

	raw, err := os.ReadFile(*image)
	if err != nil {
		log.Fatal(err)
	}
	var narration string
	if *transcript != "" {
		t, err := os.ReadFile(*transcript)
		if err != nil {
			log.Fatal(err)
		}
		narration = string(t)
	}

	brief, err := understand.NewDirector().UnderstandRoom(
		context.Background(), raw, http.DetectContentType(raw), *room, narration, *marketCtx)
	if err != nil {
		log.Fatal(err)
	}

	switch *printOnly {
	case "prompt":
		fmt.Println(brief.TransformPrompt)
		return
	case "style":
		fmt.Println(brief.Style)
		return
	}
	fmt.Printf("current state  : %s\n", brief.CurrentState)
	fmt.Printf("vision         : %s\n", brief.VisionInterpretation)
	fmt.Printf("remove         : %s\n", brief.Remove)
	fmt.Printf("add            : %s\n", brief.Add)
	fmt.Printf("style          : %s\n", brief.Style)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("transform prompt:\n%s\n", brief.TransformPrompt)
}
