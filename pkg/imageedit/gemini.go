package imageedit

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
)

// Gemini is an Editor backed by Google's Gemini image model via the REST API. It's
// one interchangeable provider — swap in OpenAI gpt-image-1, Flux, etc. behind the
// Editor interface without touching the pipeline.
//
// Untested against the live API until a key is available; request/response shapes
// follow the documented Gemini generateContent surface and may need minor tuning on
// first real call.
type Gemini struct {
	apiKey string
	model  string
	http   *http.Client
}

// NewGemini reads the key from GEMINI_API_KEY (or GOOGLE_API_KEY) and the model from
// LATENTFRAME_IMAGE_MODEL (default: gemini-2.5-flash-image).
func NewGemini() (Gemini, error) {
	key := firstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY"))
	if key == "" {
		return Gemini{}, fmt.Errorf("no image-gen key set: export GEMINI_API_KEY (or GOOGLE_API_KEY)")
	}
	model := os.Getenv("LATENTFRAME_IMAGE_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-image"
	}
	return Gemini{apiKey: key, model: model, http: &http.Client{Timeout: 120 * time.Second}}, nil
}

type geminiBlob struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *geminiBlob `json:"inlineData,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents         []geminiContent `json:"contents"`
	GenerationConfig struct {
		ResponseModalities []string `json:"responseModalities,omitempty"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Edit sends the image + instruction and returns the first image part of the response.
func (g Gemini) Edit(ctx context.Context, image []byte, mimeType, instruction string) ([]byte, string, error) {
	var req geminiRequest
	req.Contents = []geminiContent{{Parts: []geminiPart{
		{InlineData: &geminiBlob{MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(image)}},
		{Text: instruction},
	}}}
	req.GenerationConfig.ResponseModalities = []string{"IMAGE"}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", g.model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.http.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", fmt.Errorf("decode gemini response: %w", err)
	}
	if parsed.Error != nil {
		return nil, "", fmt.Errorf("gemini error: %s", parsed.Error.Message)
	}
	for _, cand := range parsed.Candidates {
		for _, part := range cand.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				out, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					return nil, "", fmt.Errorf("decode returned image: %w", err)
				}
				return out, part.InlineData.MimeType, nil
			}
		}
	}
	return nil, "", fmt.Errorf("gemini returned no image (response: %s)", truncate(string(raw), 300))
}

var _ Editor = Gemini{}

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
