// Package verify implements the honesty gate — pipeline stage 4 (VERIFY).
//
// It shows Claude the BEFORE (original hero frame) and AFTER (proposed re-staged
// image) and asks one question: was the architecture preserved, and is the result
// believable? This is the enforcement point for the product's inviolable rule —
// RE-STAGE, NEVER RESTRUCTURE — and it fails closed (see the system prompt).
package verify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/andreas-lindfalk/latentframe/pkg/pipeline"
)

// model is the judge. Per project policy we use the strongest model for the gate —
// it's the moat, and it runs at most a handful of times per room.
const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You are the honesty gate for Latent Frame, a tool that shows the *potential* of a
property by digitally re-staging a room: removing old furniture and clutter and
furnishing it beautifully. You are shown two images of the SAME room:

  BEFORE — the real, current photo.
  AFTER  — a proposed AI re-staged version to show the buyer.

The product has ONE inviolable rule: RE-STAGE, NEVER RESTRUCTURE. The AFTER must be
the same physical room with IDENTICAL ARCHITECTURE. Only movable contents and
surfaces may change.

Allowed to change: furniture, rugs, curtains, decor, plants, freestanding
appliances, wall paint/colour, and surface finishes (flooring, tiles, worktops).

MUST be identical (architecture): the number, position, size and proportion of
windows, doors and openings; walls and their placement; room shape and dimensions;
ceiling height and shape; and permanent built-in structural elements. The camera
viewpoint should read as the same vantage of the same room.

Set architecture_preserved = false if the AFTER adds, removes, moves, resizes or
reshapes any window, door, wall or opening; changes the room's proportions or
ceiling; alters the structural layout; or depicts a different room. When you are
UNSURE whether the architecture changed, set it to false — we fail closed rather
than ship a misleading image to a buyer.

Set believable = false if the AFTER has obvious AI artifacts, warped or melted
geometry, impossible perspective, or otherwise would not pass as a real interior
photograph.

Look carefully, then call record_verdict exactly once.`

// Gate is a Claude-backed pipeline.Verifier.
type Gate struct {
	client anthropic.Client
}

// NewGate builds a Gate. The Anthropic client reads ANTHROPIC_API_KEY from the
// environment.
func NewGate() Gate {
	return Gate{client: anthropic.NewClient()}
}

var _ pipeline.Verifier = Gate{}

// Verify implements pipeline.Verifier using the room's hero frame as BEFORE and its
// re-staged still as AFTER.
func (g Gate) Verify(ctx context.Context, room *pipeline.Room) (pipeline.Verdict, error) {
	return g.VerifyPair(ctx, room.Hero.Path, room.AfterStill, room.Label)
}

// VerifyPair judges a single before/after image pair. roomLabel is optional context
// (e.g. "kitchen") and may be empty.
func (g Gate) VerifyPair(ctx context.Context, beforePath, afterPath, roomLabel string) (pipeline.Verdict, error) {
	beforeType, beforeData, err := loadImage(beforePath)
	if err != nil {
		return pipeline.Verdict{}, fmt.Errorf("load before image: %w", err)
	}
	afterType, afterData, err := loadImage(afterPath)
	if err != nil {
		return pipeline.Verdict{}, fmt.Errorf("load after image: %w", err)
	}

	intro := "Here are the BEFORE and AFTER images of the same room."
	if strings.TrimSpace(roomLabel) != "" {
		intro = fmt.Sprintf("Here are the BEFORE and AFTER images of the same room (%s).", roomLabel)
	}

	userMsg := anthropic.NewUserMessage(
		anthropic.NewTextBlock(intro),
		anthropic.NewTextBlock("BEFORE (real current photo):"),
		anthropic.NewImageBlockBase64(beforeType, beforeData),
		anthropic.NewTextBlock("AFTER (proposed AI re-staged image):"),
		anthropic.NewImageBlockBase64(afterType, afterData),
		anthropic.NewTextBlock("Judge whether the architecture was preserved and whether the AFTER is believable, then call record_verdict."),
	)

	tool := anthropic.ToolParam{
		Name:        "record_verdict",
		Description: anthropic.String("Record the honesty-gate verdict for the proposed re-staged image."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"architecture_preserved": map[string]any{
					"type":        "boolean",
					"description": "True only if walls, windows, doors, openings, room shape and ceiling are identical to the BEFORE.",
				},
				"believable": map[string]any{
					"type":        "boolean",
					"description": "True if the AFTER would pass as a real interior photograph (no warping, melting, or AI artifacts).",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "One or two sentences justifying the verdict.",
				},
				"drift_notes": map[string]any{
					"type":        "string",
					"description": "If architecture was NOT preserved, describe exactly what changed (e.g. 'a window was added on the left wall'). Empty otherwise.",
				},
			},
			Required:    []string{"architecture_preserved", "believable", "reason", "drift_notes"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	resp, err := g.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      model,
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("record_verdict"),
		Messages:   []anthropic.MessageParam{userMsg},
	})
	if err != nil {
		return pipeline.Verdict{}, fmt.Errorf("claude verify request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var payload struct {
			ArchitecturePreserved bool   `json:"architecture_preserved"`
			Believable            bool   `json:"believable"`
			Reason                string `json:"reason"`
			DriftNotes            string `json:"drift_notes"`
		}
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &payload); err != nil {
			return pipeline.Verdict{}, fmt.Errorf("parse verdict: %w", err)
		}

		reason := strings.TrimSpace(payload.Reason)
		if !payload.ArchitecturePreserved && strings.TrimSpace(payload.DriftNotes) != "" {
			reason = fmt.Sprintf("%s (drift: %s)", reason, strings.TrimSpace(payload.DriftNotes))
		}
		return pipeline.Verdict{
			ArchitecturePreserved: payload.ArchitecturePreserved,
			Believable:            payload.Believable,
			Reason:                reason,
		}, nil
	}

	return pipeline.Verdict{}, fmt.Errorf("model returned no verdict (stop reason: %s)", resp.StopReason)
}

// loadImage reads an image file and returns its media type + base64 data. It
// detects the type from the bytes, not the filename — generators often return PNG
// even when the output is named .jpg, and Anthropic rejects a mismatched media type.
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
