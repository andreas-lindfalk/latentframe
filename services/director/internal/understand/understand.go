// Package understand implements pipeline stage 2 (UNDERSTAND) — the art-director
// half of the director's moat. It looks at a room (and any spoken narration) and
// decides the single most appealing, market-appropriate way to re-stage it, emitting
// a design brief that feeds RESTAGE. This is what replaces a hand-written style prompt.
package understand

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You are the art director for Latent Frame, which shows buyers the *potential*
of a property by digitally re-staging its rooms. Given a photo of a room — and, when
available, the owner or agent's spoken narration and the market context — decide the
single most appealing, tasteful way to stage it.

The inviolable rule is RE-STAGE, NEVER RESTRUCTURE: you may propose new furniture,
decor, soft furnishings, wall colour, flooring, and lighting, but NEVER structural
changes (walls, windows, doors, layout stay exactly as they are).

Choose ONE coherent design direction appropriate to the property and its likely
buyer. If narration states the owner's intent, honour it. Otherwise infer a tasteful,
broadly appealing direction for the market. Keep it realistic and achievable as a
cosmetic refresh — not a fantasy.

Then call record_brief exactly once. global_style must be a vivid, concrete phrase an
image model can act on (materials, palette, mood, light), e.g. "warm Mediterranean
minimalism — light oak floors, off-white lime-washed walls, natural linen and rattan,
greenery, soft daylight".`

// Brief is stage 2's output for one room.
type Brief struct {
	Style         string `json:"style"`
	CurrentState  string `json:"current_state"`
	DesiredChange string `json:"desired_change"`
}

// Director is a Claude-backed art director.
type Director struct {
	client anthropic.Client
}

// NewDirector builds a Director. The Anthropic client reads ANTHROPIC_API_KEY.
func NewDirector() Director {
	return Director{client: anthropic.NewClient()}
}

// UnderstandRoom produces a design brief for a single room. narration and
// marketContext may be empty.
func (d Director) UnderstandRoom(ctx context.Context, image []byte, mimeType, roomLabel, narration, marketContext string) (Brief, error) {
	intro := "Room to stage."
	if strings.TrimSpace(roomLabel) != "" {
		intro = fmt.Sprintf("Room to stage: %s.", roomLabel)
	}
	if strings.TrimSpace(marketContext) != "" {
		intro += " Market context: " + strings.TrimSpace(marketContext) + "."
	}

	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock(intro),
		anthropic.NewImageBlockBase64(mimeType, base64.StdEncoding.EncodeToString(image)),
	}
	if strings.TrimSpace(narration) != "" {
		blocks = append(blocks, anthropic.NewTextBlock("Owner/agent narration:\n"+strings.TrimSpace(narration)))
	}
	blocks = append(blocks, anthropic.NewTextBlock("Decide the best staging direction and call record_brief."))

	tool := anthropic.ToolParam{
		Name:        "record_brief",
		Description: anthropic.String("Record the staging brief for this room."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"current_state": map[string]any{
					"type":        "string",
					"description": "One sentence on how the room looks now.",
				},
				"desired_change": map[string]any{
					"type":        "string",
					"description": "One sentence on what the re-staging should achieve (contents only, never structure).",
				},
				"style": map[string]any{
					"type":        "string",
					"description": "A vivid, concrete design direction an image model can act on (materials, palette, mood, light).",
				},
			},
			Required:    []string{"current_state", "desired_change", "style"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	resp, err := d.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      model,
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("record_brief"),
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return Brief{}, fmt.Errorf("claude understand request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var b Brief
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &b); err != nil {
			return Brief{}, fmt.Errorf("parse brief: %w", err)
		}
		return b, nil
	}
	return Brief{}, fmt.Errorf("model returned no brief (stop reason: %s)", resp.StopReason)
}
