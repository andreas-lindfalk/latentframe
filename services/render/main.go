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
	"path/filepath"
	"strings"
	"time"

	"github.com/andreas-lindfalk/latentframe/pkg/env"
	"github.com/andreas-lindfalk/latentframe/pkg/imageedit"
	"github.com/andreas-lindfalk/latentframe/pkg/propertymodel"
	"github.com/andreas-lindfalk/latentframe/pkg/videoedit"
	"github.com/andreas-lindfalk/latentframe/services/render/internal/showcase"
	"github.com/andreas-lindfalk/latentframe/services/render/internal/video"
)

func main() {
	log.SetFlags(0)
	env.Load(".env")

	if len(os.Args) < 2 {
		log.Fatal("usage: render <restage|showcase|animate|transform|upscale|add-sound> ...")
	}
	// Each subcommand validates its own provider key (Gemini for restage/animate,
	// fal for the video-track ops), so there's no global key check here.
	switch os.Args[1] {
	case "restage":
		restage(os.Args[2:])
	case "showcase":
		showcaseCmd(os.Args[2:])
	case "animate":
		animate(os.Args[2:])
	case "transform":
		transformCmd(os.Args[2:])
	case "upscale":
		upscaleCmd(os.Args[2:])
	case "add-sound":
		soundCmd(os.Args[2:])
	default:
		log.Fatalf("unknown command %q (use: restage | showcase | animate | transform | upscale | add-sound)", os.Args[1])
	}
}

// upscaleCmd runs the high-quality delivery step: raise the video's resolution.
func upscaleCmd(args []string) {
	fs := flag.NewFlagSet("upscale", flag.ExitOnError)
	url := fs.String("url", "", "source video URL (required)")
	model := fs.String("model", "", "fal upscale model id (default: topaz)")
	out := fs.String("out", "", "optional path to download the upscaled video")
	timeout := fs.Duration("timeout", 15*time.Minute, "max time to wait")
	_ = fs.Parse(args)
	if *url == "" {
		fs.Usage()
		log.Fatal("\n--url is required")
	}
	fal, err := videoedit.NewFal()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	log.Printf("upscaling %s …", *url)
	res, err := fal.Upscale(ctx, videoedit.UpscaleRequest{VideoURL: *url, Model: *model})
	if err != nil {
		log.Fatal(err)
	}
	finish(ctx, res.VideoURL, *out)
}

// soundCmd adds ambient sound / SFX to a silent reveal clip.
func soundCmd(args []string) {
	fs := flag.NewFlagSet("add-sound", flag.ExitOnError)
	url := fs.String("url", "", "source video URL (required)")
	prompt := fs.String("prompt", "", "optional description of the sound to add")
	model := fs.String("model", "", "fal sound model id (default: mirelo sfx)")
	out := fs.String("out", "", "optional path to download the result")
	timeout := fs.Duration("timeout", 15*time.Minute, "max time to wait")
	_ = fs.Parse(args)
	if *url == "" {
		fs.Usage()
		log.Fatal("\n--url is required")
	}
	fal, err := videoedit.NewFal()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	log.Printf("adding sound to %s …", *url)
	res, err := fal.AddSound(ctx, videoedit.SoundRequest{VideoURL: *url, Prompt: *prompt, Model: *model})
	if err != nil {
		log.Fatal(err)
	}
	finish(ctx, res.VideoURL, *out)
}

// finish prints the result URL and optionally downloads it.
func finish(ctx context.Context, videoURL, out string) {
	fmt.Printf("✓ done → %s\n", videoURL)
	if out != "" {
		if err := videoedit.Download(ctx, videoURL, out); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  downloaded → %s\n", out)
	}
}

// transformCmd runs the VIDEO track (stage 3+5 fused): an in-context v2v edit that
// transforms a real walkthrough in place — same camera, architecture preserved.
func transformCmd(args []string) {
	fs := flag.NewFlagSet("transform", flag.ExitOnError)
	url := fs.String("url", "", "source video URL, publicly reachable (required)")
	prompt := fs.String("prompt", "", "edit / re-stage prompt (required)")
	out := fs.String("out", "", "optional path to download the transformed video")
	keepAudio := fs.Bool("keep-audio", false, "keep the source audio")
	timeout := fs.Duration("timeout", 15*time.Minute, "max time to wait")
	_ = fs.Parse(args)
	if *url == "" || *prompt == "" {
		fs.Usage()
		log.Fatal("\n--url and --prompt are required")
	}

	fal, err := videoedit.NewFal()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("transforming (v2v) %s … (this takes a few minutes)", *url)
	res, err := fal.Transform(ctx, videoedit.Request{VideoURL: *url, Prompt: *prompt, KeepAudio: *keepAudio})
	if err != nil {
		log.Fatal(err)
	}
	finish(ctx, res.VideoURL, *out)
}

// restage runs stage 3: hero frame → re-staged "after" still.
func restage(args []string) {
	fs := flag.NewFlagSet("restage", flag.ExitOnError)
	in := fs.String("in", "", "path to the input hero frame (required)")
	out := fs.String("out", "", "path to write the re-staged 'after' image (required)")
	room := fs.String("room", "", "room label, e.g. 'kitchen'")
	style := fs.String("style", "", "target design style (defaults to a warm modern minimalism)")
	prompt := fs.String("prompt", "", "full edit prompt (e.g. UNDERSTAND's transform_prompt); overrides --style")
	engine := fs.String("engine", "nano-banana", "restage engine: 'nano-banana' (Gemini 3 in-context edit, bake-off winner), 'depth-t2i' (FLUX Control-LoRA), or 'gemini' (legacy)")
	_ = fs.Parse(args)
	if *in == "" || *out == "" {
		fs.Usage()
		log.Fatal("\n--in and --out are required")
	}

	editor, err := imageedit.NewEditor(*engine)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("restage engine: %s", *engine)

	var written string
	if strings.TrimSpace(*prompt) != "" {
		log.Printf("re-staging %s → %s (using supplied prompt)", *in, *out)
		written, err = imageedit.EditFile(context.Background(), editor, *in, *out, *prompt)
	} else {
		log.Printf("re-staging %s → %s (style: %q)", *in, *out, styleOrDefault(*style))
		written, err = imageedit.RestageFile(context.Background(), editor, *in, *out, *room, *style)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ wrote %s\n", written)
	fmt.Println("next: director verify --before", *in, "--after", written)
}

// showcaseCmd runs the multi-style RESTAGE stage: a property spec → 3 style
// variants per room via nano-banana + the page's data manifest. It is the
// reproducible, in-repo path that feeds web/showcase.
//
//	render showcase --spec playbook/showcase/zeniamar.spec.json
//	render showcase --spec … --generate=false          # re-emit manifest only
//	render showcase --spec … --only-rooms living --only-style coastal
func showcaseCmd(args []string) {
	fs := flag.NewFlagSet("showcase", flag.ExitOnError)
	spec := fs.String("spec", "", "hand-authored property spec JSON (use this OR --model)")
	modelPath := fs.String("model", "", "curated property model from `director classify` (use this OR --spec)")
	photos := fs.String("photos", "", "source photo folder the model's hero filenames resolve against (required with --model)")
	app := fs.String("app", "web/showcase", "Next app dir; writes <app>/public/renders and <app>/data/*.json")
	engine := fs.String("engine", "nano-banana", "restage engine")
	generate := fs.Bool("generate", true, "run the model to (re)generate style renders; --generate=false emits the manifest only")
	onlyStyle := fs.String("only-style", "", "limit to one style id (e.g. mediterranean)")
	onlyRooms := fs.String("only-rooms", "", "comma-separated space ids to limit to (e.g. living,kitchen)")
	conc := fs.Int("concurrency", 3, "max concurrent restage calls")
	bestOf := fs.Int("best-of", 1, "candidates to generate per space×style (>1 = best-of-N: honesty-gate each, keep the honest ones)")
	keep := fs.Int("keep", 1, "how many honest, distinct variants to keep per space×style")
	gateVotes := fs.Int("gate-votes", 1, "independent honesty-gate passes per candidate (>1 = must pass ALL; raises recall on stubborn spaces like kitchens)")
	slug := fs.String("slug", "", "property slug — namespaces output to <app>/public/renders/<slug>/ and <app>/data/<slug>.full.json (empty = root, back-compat)")
	prompts := fs.String("prompts", "playbook/prompts/styles", "dir with per-style flavor prompts")
	_ = fs.Parse(args)
	if *spec == "" && *modelPath == "" {
		fs.Usage()
		log.Fatal("\n--spec or --model is required")
	}

	only := map[string]bool{}
	for _, r := range strings.Split(*onlyRooms, ",") {
		if r = strings.TrimSpace(r); r != "" {
			only[r] = true
		}
	}

	// --slug namespaces every output so multiple properties coexist in one app without
	// clobbering each other (villa "kitchen" vs Zeniamar "kitchen"). Empty = legacy root.
	rendersDir := filepath.Join(*app, "public", "renders")
	reelsDir := filepath.Join(*app, "public", "reels")
	webBase := "/renders"
	reelsWebBase := "/reels"
	manifestName := "property.full.json"
	if *slug != "" {
		rendersDir = filepath.Join(rendersDir, *slug)
		reelsDir = filepath.Join(reelsDir, *slug)
		webBase = "/renders/" + *slug
		reelsWebBase = "/reels/" + *slug
		manifestName = *slug + ".full.json"
	}

	opts := showcase.Options{
		RendersDir:   rendersDir,
		ReelsDir:     reelsDir,
		ReelsWebBase: reelsWebBase,
		WebBase:      webBase,
		PromptDir:    *prompts,
		Engine:       *engine,
		Generate:     *generate,
		OnlyStyle:    *onlyStyle,
		OnlyRooms:    only,
		Concurrency:  *conc,
		BestOf:       *bestOf,
		KeepK:        *keep,
		GateVotes:    *gateVotes,
		Logf:         log.Printf,
	}

	if *modelPath != "" {
		if *photos == "" {
			log.Fatal("--photos (source folder) is required with --model")
		}
		m, err := propertymodel.Load(*modelPath)
		if err != nil {
			log.Fatal(err)
		}
		opts.ManifestPath = filepath.Join(*app, "data", manifestName)
		if err := showcase.RunFromModel(context.Background(), m, *photos, opts); err != nil {
			log.Fatal(err)
		}
	} else {
		sp, err := showcase.LoadSpec(*spec)
		if err != nil {
			log.Fatal(err)
		}
		opts.ManifestPath = filepath.Join(*app, "data", "property.json")
		if err := showcase.Run(context.Background(), sp, opts); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("next: (cd web/showcase && pnpm dev) then open http://localhost:3000")
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
