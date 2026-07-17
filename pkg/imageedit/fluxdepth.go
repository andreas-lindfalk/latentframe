// fluxdepth.go — the depth-t2i RESTAGE backend (the aesthetic engine).
//
// Instead of an in-context edit (which anchors the dated furniture, as Gemini does),
// this locks only the room's STRUCTURE (via a control map) and generates FRESH content
// from the prompt. That fully guts a dated room while keeping the geometry — and pairs
// with the VERIFY gate's INSPIRE bar (decorative drift is fine; size/light/view/structure
// are protected).
//
// Two structural controls (LATENTFRAME_CONTROL):
//   - canny (DEFAULT): edge map — locks the LINES of windows/doors/openings (the "shell"),
//     while a fixture-aware prompt is free to upgrade fixtures (bidet→shower). The taming
//     win: keeps openings honest without over-mimicking the old fixtures.
//   - depth: 3D layout — frees fixtures but is blind to flat openings (drops windows/doors).
//
//	source → fal <control> preprocessor → FLUX Control-LoRA <control> (t2i) → after
package imageedit

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"github.com/andreas-lindfalk/latentframe/pkg/fal"
)

// control map: fal preprocessor + matching FLUX Control-LoRA (t2i) endpoint, per mode.
var controlModels = map[string]struct{ pre, ctrl string }{
	"canny": {"fal-ai/image-preprocessors/canny", "fal-ai/flux-control-lora-canny"},
	"depth": {"fal-ai/imageutils/depth", "fal-ai/flux-control-lora-depth"},
}

// FluxDepth is an Editor that restages via structure-locked FLUX generation.
type FluxDepth struct {
	fal      *fal.Client
	control  string  // "canny" (default) | "depth"
	scale    float64 // control_lora_strength — structure fidelity
	steps    int
	guidance float64
}

// NewFluxDepth builds the aesthetic engine. Reads FAL_API_KEY; tunables from env
// (LATENTFRAME_CONTROL / _DEPTH_SCALE / _FLUX_STEPS / _FLUX_GUIDANCE). Defaults: canny
// control at strength 0.8 — the taming sweet spot (shell locked, fixtures free).
func NewFluxDepth() (FluxDepth, error) {
	c, err := fal.New()
	if err != nil {
		return FluxDepth{}, err
	}
	control := os.Getenv("LATENTFRAME_CONTROL")
	if _, ok := controlModels[control]; !ok {
		control = "canny"
	}
	return FluxDepth{
		fal:      c,
		control:  control,
		scale:    envFloat("LATENTFRAME_DEPTH_SCALE", 0.8),
		steps:    envInt("LATENTFRAME_FLUX_STEPS", 30),
		guidance: envFloat("LATENTFRAME_FLUX_GUIDANCE", 3.5),
	}, nil
}

var _ Editor = FluxDepth{}

// Edit restages the image: upload → control map → structure-locked FLUX generation → bytes.
// instruction is the full target-look prompt; the SOURCE only contributes its structure.
func (f FluxDepth) Edit(ctx context.Context, img []byte, mimeType, instruction string) ([]byte, string, error) {
	m := controlModels[f.control]

	srcURL, err := f.fal.Upload(ctx, img, mimeType, "source"+extFor(mimeType))
	if err != nil {
		return nil, "", fmt.Errorf("upload source: %w", err)
	}

	preRes, err := f.fal.Run(ctx, m.pre, map[string]any{"image_url": srcURL})
	if err != nil {
		return nil, "", fmt.Errorf("%s preprocess: %w", f.control, err)
	}
	ctrlURL := fal.FirstImageURL(preRes)
	if ctrlURL == "" {
		return nil, "", fmt.Errorf("%s preprocess returned no image", f.control)
	}

	input := map[string]any{
		"control_lora_image_url": ctrlURL,
		"prompt":                 instruction,
		"control_lora_strength":  f.scale,
		"num_inference_steps":    f.steps,
		"guidance_scale":         f.guidance,
		"image_size":             matchedSize(img),
		"output_format":          "jpeg",
	}
	res, err := f.fal.Run(ctx, m.ctrl, input)
	if err != nil {
		return nil, "", fmt.Errorf("flux %s restage: %w", f.control, err)
	}
	outURL := fal.FirstImageURL(res)
	if outURL == "" {
		return nil, "", fmt.Errorf("flux %s restage returned no image", f.control)
	}
	out, err := f.fal.Download(ctx, outURL)
	if err != nil {
		return nil, "", fmt.Errorf("download result: %w", err)
	}
	return out, "image/jpeg", nil
}

// matchedSize returns an image_size that keeps the source's aspect ratio (long side
// ~1024, dims rounded to a multiple of 16 as FLUX prefers). Falls back to a landscape
// enum if the source can't be decoded.
func matchedSize(img []byte) any {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img))
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return "landscape_4_3"
	}
	const long = 1024.0
	scale := long / float64(max(cfg.Width, cfg.Height))
	round16 := func(v float64) int {
		n := int(v/16+0.5) * 16
		if n < 256 {
			n = 256
		}
		return n
	}
	return map[string]any{
		"width":  round16(float64(cfg.Width) * scale),
		"height": round16(float64(cfg.Height) * scale),
	}
}

func extFor(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
