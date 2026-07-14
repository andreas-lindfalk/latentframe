// Package selectbest implements the best-of-N SELECTOR — the reliability engine.
//
// Image generation is non-deterministic (~78% of single draws hold the bar in testing),
// so production generates N candidates, keeps the ones that pass the VERIFY honesty gate,
// and asks this selector to pick the single best of them. N draws + gate-select turns an
// unreliable per-draw model into a reliable pipeline.
package selectbest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You are the SELECTOR for Latent Frame. You are shown a BEFORE image (a dated
room) and several CANDIDATE re-staged "after" images of that same room. They have already passed the
honesty gate, so judge purely on QUALITY and desirability, and pick the SINGLE BEST one.

Prefer the candidate that:
  - most fully TRANSFORMS the space — no dated furniture, tiles or finishes left behind, no clutter;
  - best hits the warm Mediterranean look (living/bedroom = cozy riad; bathroom = clean luxe spa;
    kitchen = refined warm-wood/cream — not green, not farmhouse);
  - is the most cohesive, beautifully staged and aspirational — reads like a premium real-estate
    photograph a buyer falls for;
  - has no awkward composition, odd artifacts or empty/dead zones.

Return the 1-based index of the best candidate. Call record_selection exactly once.`

// Selection is the selector's output.
type Selection struct {
	BestIndex int    `json:"best_index"` // 1-based index into the candidates
	Reason    string `json:"reason"`
}

// Selector is a Claude-backed best-of-N picker.
type Selector struct {
	client anthropic.Client
}

// NewSelector builds a Selector. The Anthropic client reads ANTHROPIC_API_KEY.
func NewSelector() Selector {
	return Selector{client: anthropic.NewClient()}
}

// SelectBest returns the 1-based index of the best candidate. candidatePaths must be
// non-empty; beforePath is optional context.
func (s Selector) SelectBest(ctx context.Context, beforePath string, candidatePaths []string, roomLabel string) (Selection, error) {
	if len(candidatePaths) == 0 {
		return Selection{}, fmt.Errorf("no candidates to select from")
	}
	if len(candidatePaths) == 1 {
		return Selection{BestIndex: 1, Reason: "only one honest candidate"}, nil
	}

	blocks := []anthropic.ContentBlockParamUnion{}
	intro := "Pick the single best re-staging."
	if strings.TrimSpace(roomLabel) != "" {
		intro = fmt.Sprintf("Pick the single best re-staging of this %s.", roomLabel)
	}
	blocks = append(blocks, anthropic.NewTextBlock(intro))
	if strings.TrimSpace(beforePath) != "" {
		bt, bd, err := loadImage(beforePath)
		if err != nil {
			return Selection{}, fmt.Errorf("load before image: %w", err)
		}
		blocks = append(blocks, anthropic.NewTextBlock("BEFORE (original dated room):"), anthropic.NewImageBlockBase64(bt, bd))
	}
	for i, p := range candidatePaths {
		mt, md, err := loadImage(p)
		if err != nil {
			return Selection{}, fmt.Errorf("load candidate %d: %w", i+1, err)
		}
		blocks = append(blocks, anthropic.NewTextBlock(fmt.Sprintf("Candidate %d:", i+1)), anthropic.NewImageBlockBase64(mt, md))
	}
	blocks = append(blocks, anthropic.NewTextBlock("Then call record_selection with the 1-based index of the best candidate."))

	tool := anthropic.ToolParam{
		Name:        "record_selection",
		Description: anthropic.String("Record which candidate is the best re-staging."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"best_index": map[string]any{"type": "integer", "description": fmt.Sprintf("1-based index of the best candidate (1..%d).", len(candidatePaths))},
				"reason":     map[string]any{"type": "string", "description": "One sentence on why it's the strongest."},
			},
			Required:    []string{"best_index", "reason"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	resp, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      model,
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("record_selection"),
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return Selection{}, fmt.Errorf("claude select request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var sel Selection
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &sel); err != nil {
			return Selection{}, fmt.Errorf("parse selection: %w", err)
		}
		if sel.BestIndex < 1 || sel.BestIndex > len(candidatePaths) {
			return Selection{}, fmt.Errorf("selector returned out-of-range index %d (have %d candidates)", sel.BestIndex, len(candidatePaths))
		}
		return sel, nil
	}
	return Selection{}, fmt.Errorf("model returned no selection (stop reason: %s)", resp.StopReason)
}

func loadImage(path string) (mediaType, encoded string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	mediaType = http.DetectContentType(raw)
	switch mediaType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return "", "", fmt.Errorf("unsupported or undetected image type %q for %s", mediaType, path)
	}
	return mediaType, base64.StdEncoding.EncodeToString(raw), nil
}
