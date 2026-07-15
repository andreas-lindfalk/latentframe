// Package imageedit is the RENDER stage's provider layer — pipeline stage 3
// (RESTAGE): take a room's hero frame and produce the aspirational "after" still.
//
// Image generation is a commodity (this is why the provider is behind an interface),
// so the value is not here — it's in the prompt (below) and the VERIFY gate. The
// prompt encodes the re-stage-never-restructure rule for the generator as a first
// line of defense; VERIFY is the enforced second line.
package imageedit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreas-lindfalk/latentframe/pkg/pipeline"
)

// Editor applies an instruction to an image and returns the edited image. Concrete
// implementations wrap an image-generation provider (see gemini.go, fluxdepth.go).
type Editor interface {
	Edit(ctx context.Context, image []byte, mimeType, instruction string) (out []byte, outMime string, err error)
}

// NewEditor selects a RESTAGE engine by name: "depth-t2i" (default — the depth-locked
// FLUX aesthetic engine, needs FAL_API_KEY) or "gemini" (in-context edit, needs
// GEMINI_API_KEY). Shared by the render CLI and the best-of-N orchestrator.
func NewEditor(engine string) (Editor, error) {
	switch engine {
	case "depth-t2i", "":
		return NewFluxDepth()
	case "gemini":
		return NewGemini()
	default:
		return nil, fmt.Errorf("unknown restage engine %q (use 'depth-t2i' or 'gemini')", engine)
	}
}

// RestagePrompt builds the stage-3 instruction from the room and the property's one
// global design vision. It is deliberately strict about architecture — the generator
// should not even attempt structural changes.
func RestagePrompt(roomLabel, style string) string {
	if strings.TrimSpace(style) == "" {
		style = "warm, modern minimalism with natural materials and soft daylight"
	}
	room := strings.TrimSpace(roomLabel)
	if room == "" {
		room = "room"
	}
	return fmt.Sprintf(`Re-stage this %s to show a buyer its potential.

KEEP THE ARCHITECTURE EXACTLY THE SAME. Do not move, add, remove, resize or reshape
any wall, window, door or opening. Do not change the room's shape, dimensions, ceiling
or the camera viewpoint. The result must read as the SAME physical room, only cosmetically
renovated.

Remove the existing furniture, clutter and dated decor. Furnish and finish the space
beautifully in this style: %s. Update furniture, soft furnishings, wall colour, flooring,
lighting and decor. Keep it tasteful and realistic.

Output a single photorealistic image that looks like a professional real-estate photograph
of this exact room after staging.`, room, style)
}

// RestageFile reads inPath, produces the "after" via ed, and writes it. The output
// extension is corrected to match the returned image type (the provider may return
// PNG for a .jpg request), so the returned path may differ from outPath.
func RestageFile(ctx context.Context, ed Editor, inPath, outPath, roomLabel, style string) (string, error) {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return "", fmt.Errorf("read input image: %w", err)
	}
	mime, err := mimeFromExt(inPath)
	if err != nil {
		return "", err
	}
	out, outMime, err := ed.Edit(ctx, raw, mime, RestagePrompt(roomLabel, style))
	if err != nil {
		return "", err
	}
	written := retargetExt(outPath, outMime)
	if err := os.WriteFile(written, out, 0o644); err != nil {
		return "", fmt.Errorf("write output image: %w", err)
	}
	return written, nil
}

// EditFile is RestageFile with a caller-supplied full prompt (bypassing RestagePrompt)
// — e.g. UNDERSTAND's transform_prompt fed directly. Returns the written path.
func EditFile(ctx context.Context, ed Editor, inPath, outPath, prompt string) (string, error) {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return "", fmt.Errorf("read input image: %w", err)
	}
	mime, err := mimeFromExt(inPath)
	if err != nil {
		return "", err
	}
	out, outMime, err := ed.Edit(ctx, raw, mime, prompt)
	if err != nil {
		return "", err
	}
	written := retargetExt(outPath, outMime)
	if err := os.WriteFile(written, out, 0o644); err != nil {
		return "", fmt.Errorf("write output image: %w", err)
	}
	return written, nil
}

// retargetExt rewrites path's extension to match the returned media type.
func retargetExt(path, mime string) string {
	want := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[mime]
	if want == "" {
		return path
	}
	cur := strings.ToLower(filepath.Ext(path))
	if cur == want || (want == ".jpg" && cur == ".jpeg") {
		return path
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + want
}

// Restager adapts an Editor to the pipeline.Restager contract (stage 3 in the
// RESTAGE→VERIFY loop).
type Restager struct {
	Editor Editor
}

var _ pipeline.Restager = Restager{}

// Restage produces room.AfterStill from room.Hero using the property's design vision.
func (r Restager) Restage(ctx context.Context, room *pipeline.Room, vision pipeline.GlobalVision) error {
	raw, err := os.ReadFile(room.Hero.Path)
	if err != nil {
		return fmt.Errorf("read hero frame: %w", err)
	}
	mime, err := mimeFromExt(room.Hero.Path)
	if err != nil {
		return err
	}
	out, outMime, err := r.Editor.Edit(ctx, raw, mime, RestagePrompt(room.Label, vision.Style))
	if err != nil {
		return err
	}
	outPath := afterPath(room.Hero.Path, outMime)
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		return fmt.Errorf("write after still: %w", err)
	}
	room.AfterStill = outPath
	return nil
}

func mimeFromExt(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".webp":
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unsupported input image type %q", filepath.Ext(path))
	}
}

func afterPath(heroPath, mime string) string {
	ext := ".png"
	if mime == "image/jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(heroPath, filepath.Ext(heroPath))
	return base + "_after" + ext
}
