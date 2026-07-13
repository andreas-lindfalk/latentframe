// Package video is the ANIMATE stage's provider layer — pipeline stage 5: turn a
// gate-approved "after" still into a short cinematic reveal clip via image-to-video.
//
// Like image generation, this is a commodity behind an interface (swap Veo for
// Runway/Kling/Luma). The taste is in the motion prompt — short, tasteful camera
// moves that wow without exposing the model's invention (see docs/02-refined-blueprint).
package video

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andreas-lindfalk/latentframe/pkg/pipeline"
)

const base = "https://generativelanguage.googleapis.com/v1beta"

// DefaultMotion keeps the camera move small — a slow dolly-in reads as flawless;
// big sweeps make the model invent occluded space and wobble.
const DefaultMotion = "Slow, smooth cinematic dolly-in through this room — a gentle, " +
	"subtle forward push at calm real-estate showcase pacing. Photorealistic; do not " +
	"change the room, furniture, or architecture; no people."

// maxVideoBytes caps a downloaded clip so a runaway response can't exhaust memory.
const maxVideoBytes = 256 << 20 // 256 MiB

// Veo is an Animator backed by Google's Veo model via the async predictLongRunning API.
type Veo struct {
	apiKey  string
	model   string
	http    *http.Client
	poll    time.Duration // interval between operation polls
	timeout time.Duration // overall deadline for one Animate call
}

// NewVeo reads the key from GEMINI_API_KEY (or GOOGLE_API_KEY) and the model from
// LATENTFRAME_VIDEO_MODEL (default: veo-3.1-fast-generate-preview).
func NewVeo() (Veo, error) {
	key := firstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY"))
	if key == "" {
		return Veo{}, fmt.Errorf("no video-gen key set: export GEMINI_API_KEY (or GOOGLE_API_KEY)")
	}
	model := os.Getenv("LATENTFRAME_VIDEO_MODEL")
	if model == "" {
		model = "veo-3.1-fast-generate-preview"
	}
	return Veo{
		apiKey:  key,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
		poll:    15 * time.Second,
		timeout: 10 * time.Minute,
	}, nil
}

// Animate submits the start image + motion prompt, polls the long-running operation
// to completion, and downloads the resulting MP4.
func (v Veo) Animate(ctx context.Context, image []byte, mimeType, prompt string) ([]byte, error) {
	reqBody := map[string]any{
		"instances": []any{map[string]any{
			"prompt": prompt,
			"image":  map[string]any{"bytesBase64Encoded": base64.StdEncoding.EncodeToString(image), "mimeType": mimeType},
		}},
		"parameters": map[string]any{"aspectRatio": "16:9"},
	}

	var op struct {
		Name string `json:"name"`
	}
	if err := v.do(ctx, http.MethodPost, fmt.Sprintf("%s/models/%s:predictLongRunning?key=%s", base, v.model, v.apiKey), reqBody, &op); err != nil {
		return nil, fmt.Errorf("veo submit: %w", err)
	}
	if op.Name == "" {
		return nil, fmt.Errorf("veo submit: no operation name returned")
	}

	// Bound the poll loop: apply a default deadline if the caller didn't set one,
	// and reuse a single ticker rather than allocating a timer each iteration.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, v.timeout)
		defer cancel()
	}
	ticker := time.NewTicker(v.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("veo: gave up waiting for the clip: %w", ctx.Err())
		case <-ticker.C:
		}

		var result map[string]any
		if err := v.do(ctx, http.MethodGet, fmt.Sprintf("%s/%s?key=%s", base, op.Name, v.apiKey), nil, &result); err != nil {
			return nil, fmt.Errorf("veo poll: %w", err)
		}
		if done, _ := result["done"].(bool); !done {
			continue
		}
		if e, ok := result["error"]; ok {
			return nil, fmt.Errorf("veo operation failed: %v", e)
		}
		uri := findVideoURI(result)
		if uri == "" {
			return nil, fmt.Errorf("veo returned no video in completed operation")
		}
		return v.download(ctx, uri)
	}
}

func (v Veo) download(ctx context.Context, uri string) ([]byte, error) {
	dl := uri + sep(uri) + "alt=media&key=" + v.apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoBytes+1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("veo download HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if int64(len(body)) > maxVideoBytes {
		return nil, fmt.Errorf("veo clip exceeds the %d MiB cap", maxVideoBytes>>20)
	}
	return body, nil
}

// do performs a JSON request and decodes into out (out may be nil for GET-as-bytes).
func (v Veo) do(ctx context.Context, method, url string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := v.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(raw), 400))
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Animator adapts Veo to the pipeline.Animator contract (stage 5).
type Animator struct {
	Veo    Veo
	Prompt string // motion prompt; DefaultMotion if empty
}

var _ pipeline.Animator = Animator{}

// Animate produces room.Clip from room.AfterStill.
func (a Animator) Animate(ctx context.Context, room *pipeline.Room) error {
	raw, err := os.ReadFile(room.AfterStill)
	if err != nil {
		return fmt.Errorf("read after still: %w", err)
	}
	prompt := a.Prompt
	if strings.TrimSpace(prompt) == "" {
		prompt = DefaultMotion
	}
	mp4, err := a.Veo.Animate(ctx, raw, http.DetectContentType(raw), prompt)
	if err != nil {
		return err
	}
	out := strings.TrimSuffix(room.AfterStill, filepathExt(room.AfterStill)) + ".mp4"
	if err := os.WriteFile(out, mp4, 0o644); err != nil {
		return fmt.Errorf("write clip: %w", err)
	}
	room.Clip = out
	return nil
}

func findVideoURI(v any) string {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "uri" || k == "fileUri" {
				if s, ok := val.(string); ok && s != "" {
					return s
				}
			}
			if r := findVideoURI(val); r != "" {
				return r
			}
		}
	case []any:
		for _, e := range t {
			if r := findVideoURI(e); r != "" {
				return r
			}
		}
	}
	return ""
}

func sep(u string) string {
	if strings.Contains(u, "?") {
		return "&"
	}
	return "?"
}

func filepathExt(p string) string {
	if i := strings.LastIndex(p, "."); i >= 0 {
		return p[i:]
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
