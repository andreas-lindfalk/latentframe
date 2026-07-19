package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/andreas-lindfalk/latentframe/pkg/fal"
)

// FalTranscriber transcribes narration via fal-ai/whisper — no local Whisper install
// needed (the deployed/CI path). Implements Transcriber, same as the CLI version.
type FalTranscriber struct {
	client   *fal.Client
	language string
	model    string
}

// NewFalTranscriber builds a fal-backed transcriber. Reads FAL_API_KEY. Empty language =
// Whisper auto-detect. Model overridable via LATENTFRAME_WHISPER_FAL_MODEL.
func NewFalTranscriber(language string) (*FalTranscriber, error) {
	c, err := fal.New()
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(os.Getenv("LATENTFRAME_WHISPER_FAL_MODEL"))
	if model == "" {
		model = "fal-ai/whisper"
	}
	return &FalTranscriber{client: c, language: strings.TrimSpace(language), model: model}, nil
}

var _ Transcriber = (*FalTranscriber)(nil)

// NewTranscriber returns the best available transcriber: fal (if FAL_API_KEY is set),
// else the local Whisper CLI. Lets ingest work in both dev and deployed environments.
func NewTranscriber(language string) Transcriber {
	if os.Getenv("FAL_API_KEY") != "" || os.Getenv("FAL_KEY") != "" {
		if t, err := NewFalTranscriber(language); err == nil {
			return t
		}
	}
	return NewWhisperCLITranscriber(language)
}

// Transcribe uploads the audio to fal and maps whisper "chunks" to timed Segments.
func (t *FalTranscriber) Transcribe(ctx context.Context, audioPath string) ([]Segment, error) {
	data, err := os.ReadFile(audioPath)
	if err != nil {
		return nil, fmt.Errorf("read audio: %w", err)
	}
	url, err := t.client.Upload(ctx, data, "audio/mpeg", "narration.mp3")
	if err != nil {
		return nil, fmt.Errorf("upload audio: %w", err)
	}
	in := map[string]any{"audio_url": url, "task": "transcribe", "chunk_level": "segment"}
	if t.language != "" {
		in["language"] = t.language
	}
	raw, err := t.client.Run(ctx, t.model, in)
	if err != nil {
		return nil, fmt.Errorf("fal whisper: %w", err)
	}

	var out struct {
		Text   string `json:"text"`
		Chunks []struct {
			Timestamp []float64 `json:"timestamp"`
			Text      string    `json:"text"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse whisper result: %w", err)
	}

	var segs []Segment
	for _, c := range out.Chunks {
		if len(c.Timestamp) < 2 {
			continue
		}
		segs = append(segs, Segment{
			StartMs: toMs(c.Timestamp[0]),
			EndMs:   toMs(c.Timestamp[1]),
			Text:    strings.TrimSpace(c.Text),
		})
	}
	// Fallback: no chunks but a full transcript — return it as one segment.
	if len(segs) == 0 && strings.TrimSpace(out.Text) != "" {
		segs = append(segs, Segment{StartMs: 0, EndMs: 0, Text: strings.TrimSpace(out.Text)})
	}
	return segs, nil
}

// toMs converts a whisper timestamp to milliseconds. Whisper emits seconds (floats);
// guard against an endpoint that already returns ms by not re-scaling large values.
func toMs(v float64) int64 {
	if v > 10000 { // already milliseconds
		return int64(v)
	}
	return int64(v * 1000)
}
