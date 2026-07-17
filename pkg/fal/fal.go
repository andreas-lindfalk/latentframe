// Package fal is a small, generic client for the fal.ai async queue + storage APIs.
//
// It is provider plumbing shared by the image and (future) video backends: upload a
// local asset to fal storage, run any fal model through its queue, and download the
// result. The taste lives in the callers (prompts, the VERIFY gate); this is a pipe.
package fal

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

const (
	queueBase  = "https://queue.fal.run"
	uploadInit = "https://rest.alpha.fal.ai/storage/upload/initiate?storage_type=fal-cdn-v3"
)

// Client talks to fal.ai. Construct with New.
type Client struct {
	apiKey  string
	http    *http.Client
	poll    time.Duration
	timeout time.Duration
}

// New reads the key from FAL_API_KEY (or FAL_KEY).
func New() (*Client, error) {
	key := firstNonEmpty(os.Getenv("FAL_API_KEY"), os.Getenv("FAL_KEY"))
	if key == "" {
		return nil, fmt.Errorf("no fal key set: export FAL_API_KEY (fal.ai/dashboard/keys)")
	}
	return &Client{
		apiKey:  key,
		http:    &http.Client{Timeout: 120 * time.Second},
		poll:    5 * time.Second,
		timeout: 10 * time.Minute,
	}, nil
}

// Upload stores data in fal storage and returns its public URL.
func (c *Client) Upload(ctx context.Context, data []byte, contentType, fileName string) (string, error) {
	var init struct {
		FileURL   string `json:"file_url"`
		UploadURL string `json:"upload_url"`
	}
	if err := c.doJSON(ctx, http.MethodPost, uploadInit,
		map[string]any{"content_type": contentType, "file_name": fileName}, &init); err != nil {
		return "", fmt.Errorf("fal upload initiate: %w", err)
	}
	if init.UploadURL == "" || init.FileURL == "" {
		return "", fmt.Errorf("fal upload initiate: empty urls")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, init.UploadURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fal upload PUT: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 400))
		return "", fmt.Errorf("fal upload PUT HTTP %d: %s", resp.StatusCode, body)
	}
	return init.FileURL, nil
}

// Run submits input to a fal model, polls the queue to completion, and returns the raw
// result JSON. Use FirstImageURL/find helpers (or your own unmarshal) to read it.
func (c *Client) Run(ctx context.Context, model string, input map[string]any) (json.RawMessage, error) {
	var sub struct {
		StatusURL   string `json:"status_url"`
		ResponseURL string `json:"response_url"`
	}
	if err := c.doJSON(ctx, http.MethodPost, queueBase+"/"+model, input, &sub); err != nil {
		return nil, fmt.Errorf("fal submit (%s): %w", model, err)
	}
	if sub.StatusURL == "" || sub.ResponseURL == "" {
		return nil, fmt.Errorf("fal submit (%s): no status/response url", model)
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	ticker := time.NewTicker(c.poll)
	defer ticker.Stop()

	for {
		var st struct {
			Status string `json:"status"`
		}
		if err := c.pollJSON(ctx, sub.StatusURL, &st); err != nil {
			return nil, fmt.Errorf("fal status (%s): %w", model, err)
		}
		switch st.Status {
		case "COMPLETED":
			var out json.RawMessage
			if err := c.pollJSON(ctx, sub.ResponseURL, &out); err != nil {
				return nil, fmt.Errorf("fal result (%s): %w", model, err)
			}
			return out, nil
		case "IN_QUEUE", "IN_PROGRESS", "":
		default:
			return nil, fmt.Errorf("fal (%s): unexpected status %q", model, st.Status)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("fal (%s): gave up waiting: %w", model, ctx.Err())
		case <-ticker.C:
		}
	}
}

// Download fetches a URL's bytes (e.g. a result image).
func (c *Client) Download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("fal download HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// FirstImageURL pulls the first image URL out of a fal result: handles {"images":[{"url"}]}
// and {"image":{"url"}} shapes (the common image-endpoint outputs).
func FirstImageURL(raw json.RawMessage) string {
	var r struct {
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Image struct {
			URL string `json:"url"`
		} `json:"image"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return ""
	}
	if len(r.Images) > 0 && r.Images[0].URL != "" {
		return r.Images[0].URL
	}
	return r.Image.URL
}

// doJSON performs a JSON request with fal's Key auth and decodes into out (nil to ignore).
func (c *Client) doJSON(ctx context.Context, method, url string, body, out any) error {
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
	req.Header.Set("Authorization", "Key "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return &httpError{code: resp.StatusCode, body: truncate(string(raw), 400)}
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// pollJSON is a GET with bounded retry-on-transient for the idempotent status/result
// endpoints — a queued job survives a transient blip, so one bad poll must not kill it.
func (c *Client) pollJSON(ctx context.Context, url string, out any) error {
	const maxAttempts = 5
	for attempt := 1; ; attempt++ {
		err := c.doJSON(ctx, http.MethodGet, url, nil, out)
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

type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.code, e.body) }

func isTransient(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var he *httpError
	if errors.As(err, &he) {
		return he.code == http.StatusTooManyRequests || he.code >= 500
	}
	return true
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
