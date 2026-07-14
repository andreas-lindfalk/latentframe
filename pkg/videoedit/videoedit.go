// Package videoedit is the VIDEO track's provider seam — in-context video-to-video
// editing that transforms a real walkthrough in place (same camera, architecture
// preserved) into the re-staged "potential" version. Mirrors pkg/imageedit (the
// PICTURE track): the provider is a commodity behind an interface, so we never build
// a hard dependency on any one vendor. Validated 2026-07-14 (see docs/03).
package videoedit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// maxVideoBytes caps a downloaded clip so a runaway response can't exhaust memory.
const maxVideoBytes = 512 << 20 // 512 MiB

// Request is a v2v edit: transform the source video per the prompt.
type Request struct {
	VideoURL  string // source video, publicly reachable (fal-hosted or GCS)
	Prompt    string // the edit / re-stage instruction (from UNDERSTAND)
	KeepAudio bool
}

// UpscaleRequest raises a video's resolution for high-quality delivery.
type UpscaleRequest struct {
	VideoURL string
	Model    string         // fal model id override (default: ModelUpscaleTopaz)
	Params   map[string]any // model-specific extras (e.g. upscale factor)
}

// SoundRequest adds ambient sound / SFX to a (silent) video.
type SoundRequest struct {
	VideoURL string
	Prompt   string // optional description of the sound to add
	Model    string // fal model id override (default: ModelSoundMirelo)
	Params   map[string]any
}

// Result is a produced video.
type Result struct {
	VideoURL string
}

// The video track's provider seams — small, single-purpose interfaces so a stage
// depends only on what it needs, and any of them can be swapped for another vendor.
// *Fal implements all three today. Hosting the source and storing the result are the
// storage layer's concern, so these deal only in URLs.
type (
	// Transformer performs in-context video-to-video editing (the core magic).
	Transformer interface {
		Transform(ctx context.Context, req Request) (Result, error)
	}
	// Upscaler raises resolution — the "high quality" delivery step.
	Upscaler interface {
		Upscale(ctx context.Context, req UpscaleRequest) (Result, error)
	}
	// SoundAdder adds ambient sound / SFX to the reveal clip.
	SoundAdder interface {
		AddSound(ctx context.Context, req SoundRequest) (Result, error)
	}
)

// Download fetches a (transformed) video URL to dst, bounded by maxVideoBytes.
func Download(ctx context.Context, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(b))
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, maxVideoBytes)); err != nil {
		return err
	}
	return f.Close()
}
