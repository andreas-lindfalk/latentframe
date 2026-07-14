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

const systemPrompt = `You are the honesty gate for Latent Frame. Latent Frame shows a property's *potential*
by digitally re-staging a room — stripping out dated furniture, fixtures and finishes and
rebuilding the space beautifully so a buyer sees what it could become. Re-staging is often
WHOLESALE: an entire dated kitchen or bathroom may be gutted and completely replaced. That
is expected and desirable — do NOT penalise it.

You are shown two images of the SAME room:

  BEFORE — the real, current photo.
  AFTER  — a proposed AI re-staged version to show the buyer.

Your ONE job: protect a single promise — the AFTER must be the SAME PHYSICAL SPACE (the same
SHELL), just refitted. It must never mislead a buyer about the property's structure, size,
light or layout. Think in terms of SHELL vs CONTENTS.

SHELL — the building itself. This MUST be preserved. Set architecture_preserved = false if the AFTER:
  • adds, removes, enlarges, shrinks or moves any window, exterior door or opening;
  • turns a solid wall into a window, a glass door or an outdoor view (or vice-versa);
  • moves/adds/removes a wall, or changes the room's shape, dimensions, proportions or ceiling;
  • removes or hides a functional area (e.g. makes a kitchen vanish into a blank wall);
  • relocates fixtures to a different wall or plumbing position (the toilet, sink or the kitchen
    run jumps to another wall);
  • otherwise depicts a different or structurally altered room.

CONTENTS — everything the room is fitted and furnished with. These may be REPLACED WHOLESALE,
IN PLACE, and doing so must NOT lower architecture_preserved. Expected, allowed changes include:
  • all furniture, rugs, curtains, decor, plants, lighting fixtures, mirrors;
  • all finishes — paint, wallpaper, wall and floor tiles, flooring, worktops;
  • all fixtures and fittings replaced in their EXISTING position — e.g. swapping a BATHTUB for a
    walk-in shower in the SAME wet zone, a pedestal sink for a vanity on the SAME wall, a close-
    coupled toilet for a wall-hung WC in the SAME corner, or dated kitchen cabinets and built-in
    appliances for new ones along the SAME wall.

Test for a fixture change: did it stay in roughly the same place (same wall / same plumbing zone)?
If yes → allowed staging, not a violation. If it jumped to a different wall or the layout was
reconfigured → restructure → false.

CAMERA / PERSPECTIVE: the AFTER is re-rendered, so it may be shot from a slightly different angle,
height or focal length. Do NOT treat a mere viewpoint difference as a structural change. Judge the
underlying geometry of the room, not the exact framing. Only mark a window or opening as changed if
it is genuinely added, removed, resized or moved ON THE WALL — not merely seen from a different angle.

When, after accounting for perspective, you are genuinely UNSURE whether the SHELL changed, fail
closed (architecture_preserved = false). But do NOT fail a render solely because contents/finishes
were replaced or the camera moved a little — that is the product working as intended.

Set believable = false only if the AFTER has obvious AI artifacts, warped or melted geometry,
impossible perspective, or otherwise would not pass as a real interior photograph.

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
					"description": "True if the building SHELL is unchanged — same walls, windows, exterior openings, room shape/proportions/ceiling, no functional area removed, and no fixtures relocated to a different wall. Replacing fixtures/finishes/furniture IN PLACE (e.g. tub→walk-in shower in the same wet zone, new kitchen along the same wall) does NOT make this false. A slightly different camera angle does NOT make this false.",
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
					"description": "If the SHELL was changed, describe exactly what structural change occurred (e.g. 'a window was added on the right wall where the BEFORE had a solid wall'). Empty if only contents/fixtures/finishes changed or the camera merely moved.",
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
