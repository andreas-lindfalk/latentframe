// fluxdepth.go — the depth-t2i RESTAGE backend (the aesthetic engine).
//
// Instead of an in-context edit (which anchors the dated furniture, as Gemini does),
// this locks only the room's STRUCTURE via a depth map and generates FRESH content from
// the prompt. That fully guts a dated room while keeping the geometry — and pairs with
// the VERIFY gate's INSPIRE bar (decorative drift is fine; size/light/view/structure are
// protected). See the branch's cold-run validation.
//
//	source → fal depth preprocessor → FLUX Control-LoRA Depth (text-to-image) → after
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

const (
	depthModel   = "fal-ai/imageutils/depth"          // Midas depth map
	fluxDepthT2I = "fal-ai/flux-control-lora-depth"    // depth structure, fresh content
)

// FluxDepth is an Editor that restages via depth-locked FLUX generation.
type FluxDepth struct {
	fal        *fal.Client
	depthScale float64 // control_lora_strength — structure fidelity (0.95 = sweet spot)
	steps      int
	guidance   float64
}

// NewFluxDepth builds the depth-t2i backend. Reads FAL_API_KEY; tunables from env
// (LATENTFRAME_DEPTH_SCALE / _FLUX_STEPS / _FLUX_GUIDANCE) with validated defaults.
func NewFluxDepth() (FluxDepth, error) {
	c, err := fal.New()
	if err != nil {
		return FluxDepth{}, err
	}
	return FluxDepth{
		fal:        c,
		depthScale: envFloat("LATENTFRAME_DEPTH_SCALE", 0.95),
		steps:      envInt("LATENTFRAME_FLUX_STEPS", 30),
		guidance:   envFloat("LATENTFRAME_FLUX_GUIDANCE", 3.5),
	}, nil
}

var _ Editor = FluxDepth{}

// Edit restages the image: upload → depth map → depth-locked FLUX generation → bytes.
// instruction is the full target-look prompt; the SOURCE only contributes its structure.
func (f FluxDepth) Edit(ctx context.Context, img []byte, mimeType, instruction string) ([]byte, string, error) {
	srcURL, err := f.fal.Upload(ctx, img, mimeType, "source"+extFor(mimeType))
	if err != nil {
		return nil, "", fmt.Errorf("upload source: %w", err)
	}

	depthRes, err := f.fal.Run(ctx, depthModel, map[string]any{"image_url": srcURL})
	if err != nil {
		return nil, "", fmt.Errorf("depth preprocess: %w", err)
	}
	depthURL := fal.FirstImageURL(depthRes)
	if depthURL == "" {
		return nil, "", fmt.Errorf("depth preprocess returned no image")
	}

	input := map[string]any{
		"control_lora_image_url": depthURL,
		"prompt":                 instruction,
		"control_lora_strength":  f.depthScale,
		"num_inference_steps":    f.steps,
		"guidance_scale":         f.guidance,
		"image_size":             matchedSize(img),
		"output_format":          "jpeg",
	}
	res, err := f.fal.Run(ctx, fluxDepthT2I, input)
	if err != nil {
		return nil, "", fmt.Errorf("flux depth restage: %w", err)
	}
	outURL := fal.FirstImageURL(res)
	if outURL == "" {
		return nil, "", fmt.Errorf("flux depth restage returned no image")
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
