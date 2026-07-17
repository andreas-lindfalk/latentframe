// nanobanana.go — the Nano-Banana (Gemini 3 image edit) RESTAGE backend, the bake-off
// winner (2026-07-17) and the DEFAULT engine.
//
// An in-context edit: image + instruction → restaged image. Unlike FLUX control-LoRA (which
// needs a control map + a strength dial and trades wow against honesty), Nano-Banana
// preserves the architecture NATIVELY — windows exact, real views, correct fixture identity
// (toilet vs bidet) — while fully gutting the dated furniture. No preprocessor, no strength.
//
// The `instruction` must be an EDIT INSTRUCTION, e.g. "Restage this <room> …; KEEP the exact
// architecture — same walls, windows, doors, openings; REMOVE all dated furniture/fixtures
// and replace them with <target look>." See playbook/prompts/nano-banana/.
package imageedit

import (
	"context"
	"fmt"
	"os"

	"github.com/andreas-lindfalk/latentframe/pkg/fal"
)

// nanoBananaDefault is the cost-effective model (NB-2, ~$0.08/img) — matched NB-Pro on the
// bake-off. Override with LATENTFRAME_NANOBANANA_MODEL (e.g. fal-ai/nano-banana-pro/edit).
const nanoBananaDefault = "fal-ai/nano-banana-2/edit"

// NanoBanana is an Editor backed by Google's Nano-Banana (Gemini 3) image-edit model on fal.
type NanoBanana struct {
	fal   *fal.Client
	model string
}

// NewNanoBanana builds the backend. Reads FAL_API_KEY; model from LATENTFRAME_NANOBANANA_MODEL.
func NewNanoBanana() (NanoBanana, error) {
	c, err := fal.New()
	if err != nil {
		return NanoBanana{}, err
	}
	model := os.Getenv("LATENTFRAME_NANOBANANA_MODEL")
	if model == "" {
		model = nanoBananaDefault
	}
	return NanoBanana{fal: c, model: model}, nil
}

var _ Editor = NanoBanana{}

// Edit restages the image in-context: upload → nano-banana edit → download → bytes.
func (n NanoBanana) Edit(ctx context.Context, img []byte, mimeType, instruction string) ([]byte, string, error) {
	url, err := n.fal.Upload(ctx, img, mimeType, "source"+extFor(mimeType))
	if err != nil {
		return nil, "", fmt.Errorf("upload source: %w", err)
	}
	res, err := n.fal.Run(ctx, n.model, map[string]any{
		"image_urls": []string{url}, // nano-banana takes an array
		"prompt":     instruction,
	})
	if err != nil {
		return nil, "", fmt.Errorf("nano-banana edit: %w", err)
	}
	out := fal.FirstImageURL(res)
	if out == "" {
		return nil, "", fmt.Errorf("nano-banana returned no image")
	}
	b, err := n.fal.Download(ctx, out)
	if err != nil {
		return nil, "", fmt.Errorf("download result: %w", err)
	}
	return b, "image/jpeg", nil
}
