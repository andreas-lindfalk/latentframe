package videoedit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// fal model ids for the video track (all the same async queue shape).
const (
	// v2v transform (re-staging the walk)
	ModelKlingO3Pro = "fal-ai/kling-video/o3/pro/video-to-video/edit" // 3–15s, ≤4K, ~$0.168/s
	ModelHappyHorse = "alibaba/happy-horse/video-edit"                // 3–60s, 720p/1080p, ~$0.14/s
	// upscale (high-quality delivery)
	ModelUpscaleTopaz  = "fal-ai/topaz/upscale/video"
	ModelUpscaleSeedVR = "fal-ai/seedvr/upscale/video"
	// sound (ambient / SFX for the reveal)
	ModelSoundMirelo = "mirelo-ai/sfx-v1.5/video-to-video"
)

// falBase is the fal queue endpoint; overridable in tests.
var falBase = "https://queue.fal.run"

// Fal is a Transformer backed by fal.ai's async queue API. The model is
// configurable (LATENTFRAME_V2V_MODEL), defaulting to Kling O3 Pro.
type Fal struct {
	apiKey  string
	model   string
	http    *http.Client
	poll    time.Duration
	timeout time.Duration
}

// NewFal reads the key from FAL_API_KEY (or FAL_KEY) and the model from
// LATENTFRAME_V2V_MODEL (default: Kling O3 Pro).
func NewFal() (*Fal, error) {
	key := firstNonEmpty(os.Getenv("FAL_API_KEY"), os.Getenv("FAL_KEY"))
	if key == "" {
		return nil, fmt.Errorf("no fal key set: export FAL_API_KEY (fal.ai/dashboard/keys)")
	}
	model := os.Getenv("LATENTFRAME_V2V_MODEL")
	if model == "" {
		model = ModelKlingO3Pro
	}
	return &Fal{
		apiKey:  key,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
		poll:    10 * time.Second,
		timeout: 15 * time.Minute,
	}, nil
}

var (
	_ Transformer = (*Fal)(nil)
	_ Upscaler    = (*Fal)(nil)
	_ SoundAdder  = (*Fal)(nil)
)

// Transform runs an in-context v2v edit — re-stage the real walk.
func (f *Fal) Transform(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.VideoURL) == "" || strings.TrimSpace(req.Prompt) == "" {
		return Result{}, fmt.Errorf("videoedit: VideoURL and Prompt are required")
	}
	url, err := f.runVideo(ctx, f.model, map[string]any{
		"video_url":  req.VideoURL,
		"prompt":     req.Prompt,
		"keep_audio": req.KeepAudio,
	})
	return Result{VideoURL: url}, err
}

// Upscale raises a video's resolution (the high-quality delivery step).
func (f *Fal) Upscale(ctx context.Context, req UpscaleRequest) (Result, error) {
	if strings.TrimSpace(req.VideoURL) == "" {
		return Result{}, fmt.Errorf("videoedit: VideoURL is required")
	}
	url, err := f.runVideo(ctx, orDefault(req.Model, ModelUpscaleTopaz), merge(map[string]any{"video_url": req.VideoURL}, req.Params))
	return Result{VideoURL: url}, err
}

// AddSound adds ambient sound / SFX to a silent clip.
func (f *Fal) AddSound(ctx context.Context, req SoundRequest) (Result, error) {
	if strings.TrimSpace(req.VideoURL) == "" {
		return Result{}, fmt.Errorf("videoedit: VideoURL is required")
	}
	in := map[string]any{"video_url": req.VideoURL}
	if strings.TrimSpace(req.Prompt) != "" {
		in["prompt"] = req.Prompt
	}
	url, err := f.runVideo(ctx, orDefault(req.Model, ModelSoundMirelo), merge(in, req.Params))
	return Result{VideoURL: url}, err
}

// runVideo submits input to a fal model, polls the async queue to completion, and
// returns the URL of the {"video":{"url":...}} in the result. Shared by every op.
func (f *Fal) runVideo(ctx context.Context, model string, input map[string]any) (string, error) {
	var sub struct {
		StatusURL   string `json:"status_url"`
		ResponseURL string `json:"response_url"`
		RequestID   string `json:"request_id"`
	}
	if err := f.do(ctx, http.MethodPost, falBase+"/"+model, input, &sub); err != nil {
		return "", fmt.Errorf("fal submit (%s): %w", model, err)
	}
	if sub.StatusURL == "" || sub.ResponseURL == "" {
		return "", fmt.Errorf("fal submit (%s): no status/response url", model)
	}

	// Bound the poll loop and reuse one ticker.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	ticker := time.NewTicker(f.poll)
	defer ticker.Stop()

	for {
		var st struct {
			Status string `json:"status"`
		}
		if err := f.doPoll(ctx, sub.StatusURL, &st); err != nil {
			return "", fmt.Errorf("fal status (%s): %w", model, err)
		}
		switch st.Status {
		case "COMPLETED":
			var out struct {
				Video struct {
					URL string `json:"url"`
				} `json:"video"`
			}
			if err := f.doPoll(ctx, sub.ResponseURL, &out); err != nil {
				return "", fmt.Errorf("fal result (%s): %w", model, err)
			}
			if out.Video.URL == "" {
				return "", fmt.Errorf("fal result (%s): no video url", model)
			}
			return out.Video.URL, nil
		case "IN_QUEUE", "IN_PROGRESS", "":
			// keep polling
		default:
			return "", fmt.Errorf("fal (%s): unexpected status %q", model, st.Status)
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("fal (%s): gave up waiting: %w", model, ctx.Err())
		case <-ticker.C:
		}
	}
}

// do performs a JSON request with fal's Key auth and decodes into out (nil to ignore).
func (f *Fal) do(ctx context.Context, method, url string, body, out any) error {
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
	req.Header.Set("Authorization", "Key "+f.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return &httpError{code: resp.StatusCode, body: truncate(string(raw), 300)}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// httpError carries the status code so poll retries can classify transient failures.
type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.code, e.body) }

// isTransient reports whether err is worth retrying: a 429/5xx from fal, or a transport
// error — but NOT a context cancel/deadline (that's the caller giving up).
func isTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var he *httpError
	if errors.As(err, &he) {
		return he.code == http.StatusTooManyRequests || he.code >= 500
	}
	return true // transport-level blip (connection reset, timeout, DNS…)
}

// doPoll is a GET with bounded retry-on-transient, for the idempotent status/result
// endpoints. A queued fal job survives a transient blip, so a single bad poll must not
// kill a minutes-long render. Backoff respects ctx.
func (f *Fal) doPoll(ctx context.Context, url string, out any) error {
	const maxAttempts = 5
	for attempt := 1; ; attempt++ {
		err := f.do(ctx, http.MethodGet, url, nil, out)
		if err == nil {
			return nil
		}
		if attempt >= maxAttempts || !isTransient(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// merge returns base with extra's keys applied on top (extra wins).
func merge(base, extra map[string]any) map[string]any {
	for k, v := range extra {
		base[k] = v
	}
	return base
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
