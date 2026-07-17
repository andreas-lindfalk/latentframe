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
	"path/filepath"
	"strings"
	"sync"

	"github.com/andreas-lindfalk/latentframe/pkg/env"
	"github.com/andreas-lindfalk/latentframe/pkg/imageedit"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/judge"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/selectbest"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/understand"
	"github.com/andreas-lindfalk/latentframe/services/director/internal/verify"
)

func main() {
	log.SetFlags(0)
	env.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("usage: director <understand|verify|judge|select|beststage> ...")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Fatal("ANTHROPIC_API_KEY is not set (put it in .env or export it)")
	}

	switch os.Args[1] {
	case "understand":
		understandCmd(os.Args[2:])
	case "verify":
		verifyCmd(os.Args[2:])
	case "judge":
		judgeCmd(os.Args[2:])
	case "select":
		selectCmd(os.Args[2:])
	case "beststage":
		beststageCmd(os.Args[2:])
	default:
		log.Fatalf("unknown command %q (use: understand | verify | judge | select | beststage)", os.Args[1])
	}
}

// beststageCmd runs the production reliability path — BEST-OF-N: generate N restages with
// the chosen engine, keep the ones that pass the honesty gate (inspire bar by default),
// and pick the strongest. Turns a non-deterministic image engine into a reliable pipeline.
// Needs FAL_API_KEY (restage) + ANTHROPIC_API_KEY (verify + select).
func beststageCmd(args []string) {
	fs := flag.NewFlagSet("beststage", flag.ExitOnError)
	in := fs.String("in", "", "source BEFORE image (required)")
	out := fs.String("out", "", "path to write the selected AFTER (required)")
	prompt := fs.String("prompt", "", "target-look prompt (required)")
	room := fs.String("room", "", "room label, e.g. 'living room'")
	n := fs.Int("n", 3, "candidates to generate")
	mode := fs.String("mode", "inspire", "honesty bar: 'inspire' (potential) or 'strict' (shell-vs-contents)")
	engine := fs.String("engine", "nano-banana", "restage engine: 'nano-banana' (default), 'depth-t2i' or 'gemini'")
	keepDir := fs.String("keep-dir", "", "optional dir to keep all candidates (else a temp dir)")
	_ = fs.Parse(args)
	if *in == "" || *out == "" || *prompt == "" {
		fs.Usage()
		log.Fatal("\n--in, --out and --prompt are required")
	}

	editor, err := imageedit.NewEditor(*engine)
	if err != nil {
		log.Fatal(err)
	}
	gate := verify.NewGate()
	switch *mode {
	case "strict":
	case "inspire":
		gate = verify.NewInspireGate()
	default:
		log.Fatalf("unknown --mode %q (use 'inspire' or 'strict')", *mode)
	}

	ctx := context.Background()
	dir := *keepDir
	if dir == "" {
		dir, err = os.MkdirTemp("", "beststage-")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir)
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	// 1. generate N candidates concurrently (the engine is non-deterministic).
	log.Printf("generating %d candidate(s) with %s …", *n, *engine)
	paths := make([]string, *n)
	genErr := make([]error, *n)
	var wg sync.WaitGroup
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := filepath.Join(dir, fmt.Sprintf("cand%d.jpg", i+1))
			paths[i], genErr[i] = imageedit.EditFile(ctx, editor, *in, p, *prompt)
		}(i)
	}
	wg.Wait()

	// 2. keep the ones that pass the honesty gate (concurrent verify).
	honestCh := make([]string, *n)
	var wg2 sync.WaitGroup
	for i := 0; i < *n; i++ {
		if genErr[i] != nil {
			log.Printf("  cand %d: gen failed: %v", i+1, genErr[i])
			continue
		}
		wg2.Add(1)
		go func(i int) {
			defer wg2.Done()
			v, e := gate.VerifyPair(ctx, *in, paths[i], *room)
			if e != nil {
				log.Printf("  cand %d: verify failed: %v", i+1, e)
				return
			}
			if v.OK() {
				honestCh[i] = paths[i]
			}
			log.Printf("  cand %d: %s", i+1, honestTag(v.OK()))
		}(i)
	}
	wg2.Wait()

	var honest []string
	for _, p := range honestCh {
		if p != "" {
			honest = append(honest, p)
		}
	}
	if len(honest) == 0 {
		log.Fatalf("✗ 0/%d candidates passed the %s honesty bar — nothing to ship (strengthen the prompt or raise -n)", *n, *mode)
	}

	// 3. select the strongest honest candidate.
	winner, why := honest[0], "only honest candidate"
	if len(honest) > 1 {
		sel, e := selectbest.NewSelector().SelectBest(ctx, *in, honest, *room)
		if e != nil {
			log.Fatal(e)
		}
		winner, why = honest[sel.BestIndex-1], sel.Reason
	}

	// 4. ship the winner.
	data, err := os.ReadFile(winner)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ shipped %d/%d honest → %s\n", len(honest), *n, *out)
	fmt.Printf("  reason: %s\n", why)
}

func honestTag(ok bool) string {
	if ok {
		return "HONEST ✓"
	}
	return "rejected ✗"
}

// selectCmd runs the best-of-N SELECTOR: given several honest candidates for the same
// room, print the 1-based index of the best one. This is the production reliability
// engine — generate N, keep the honest ones (verify), pick the strongest (this).
func selectCmd(args []string) {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	before := fs.String("before", "", "optional path to the original BEFORE (context)")
	room := fs.String("room", "", "optional room label, e.g. 'kitchen'")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: director select [--before b.jpg] [--room kitchen] cand1.png cand2.png ...")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	candidates := fs.Args()
	if len(candidates) == 0 {
		fs.Usage()
		log.Fatal("\nprovide one or more candidate image paths")
	}

	sel, err := selectbest.NewSelector().SelectBest(context.Background(), *before, candidates, *room)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("best index : %d\n", sel.BestIndex)
	fmt.Printf("best path  : %s\n", candidates[sel.BestIndex-1])
	fmt.Printf("reason     : %s\n", sel.Reason)
}

// judgeCmd runs the regression harness's quality judge: does a new candidate hold the
// bar set by a previously-approved reference for the same room?
func judgeCmd(args []string) {
	fs := flag.NewFlagSet("judge", flag.ExitOnError)
	candidate := fs.String("candidate", "", "path to the newly generated AFTER (required)")
	reference := fs.String("reference", "", "path to the previously-approved AFTER (required)")
	before := fs.String("before", "", "optional path to the original BEFORE (context)")
	room := fs.String("room", "", "optional room label, e.g. 'kitchen'")
	_ = fs.Parse(args)
	if *candidate == "" || *reference == "" {
		fs.Usage()
		log.Fatal("\n--candidate and --reference are required")
	}

	verdict, err := judge.NewJudge().JudgePair(context.Background(), *before, *candidate, *reference, *room)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("meets bar          : %v\n", verdict.MeetsBar)
	fmt.Printf("quality vs reference: %s\n", verdict.QualityVsReference)
	fmt.Printf("reason             : %s\n", verdict.Reason)
	fmt.Println(strings.Repeat("─", 40))
	if verdict.OK() {
		fmt.Println("✓ HOLDS — non-regression")
		return
	}
	fmt.Println("✗ REGRESSION — candidate is worse than the approved reference")
	os.Exit(1)
}

// verifyCmd runs stage 4: the honesty gate on a before/after pair.
func verifyCmd(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	before := fs.String("before", "", "path to the BEFORE image (real current photo) (required)")
	after := fs.String("after", "", "path to the AFTER image (proposed re-staged render) (required)")
	room := fs.String("room", "", "optional room label, e.g. 'kitchen'")
	mode := fs.String("mode", "strict", "honesty bar: 'strict' (shell-vs-contents) or 'inspire' (potential — relax decorative architecture, keep buyable facts: size/light/view/structure)")
	_ = fs.Parse(args)
	if *before == "" || *after == "" {
		fs.Usage()
		log.Fatal("\n--before and --after are required")
	}

	gate := verify.NewGate()
	switch *mode {
	case "strict":
	case "inspire":
		gate = verify.NewInspireGate()
	default:
		log.Fatalf("unknown --mode %q (use 'strict' or 'inspire')", *mode)
	}
	verdict, err := gate.VerifyPair(context.Background(), *before, *after, *room)
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
