// Package understand implements pipeline stage 2 (UNDERSTAND) — the art-director half
// of the director's moat, and the taste engine of the whole product. It looks at a room
// (crowded with the previous owner's dated furniture) plus the owner's spoken vision, and
// imagines the space at its full potential: what to strip out, what to bring in and where,
// in a coherent gorgeous style — emitting a transform prompt the RESTAGE / v2v stages run.
//
// This is the one place we spend the most capable model: Claude Fable 5. It's the
// creative-reasoning core, it runs only a handful of times per property (cost is cents),
// and per the product rule the core must be *wow*.
package understand

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// The taste engine runs on Anthropic's most capable model.
const model = anthropic.ModelClaudeFable5

const systemPrompt = `You are the art director for Latent Frame. Your job: look at a room — often
crowded with the previous owner's dated, mismatched furniture — and imagine it re-staged at its
FULL POTENTIAL, so a buyer falls in love with what the space could become.

These are INSPIRATIONAL visualisations. Be ambitious and tasteful: design like a top interior
stylist selling a dream, not a cautious realtor. Optimise for desire and wow.

The ONE hard rule is RE-STAGE, NEVER RESTRUCTURE — keep every wall, window, door, opening,
ceiling, built-in and the room's proportions exactly as they are. But everything the room
*contains* is yours to reimagine: you may (and usually should) REMOVE the existing furniture and
clutter entirely and furnish the space anew. Do NOT merely recolour what is already there.

If the owner/agent narration is present, treat it as THE VISION — realise it faithfully and
beautifully, filling in tasteful detail they did not specify. If there is no narration, infer the
most appealing, market-appropriate vision yourself.

Then call record_brief exactly once. Make transform_prompt a single, vivid, concrete instruction
an image/video model can execute directly — naming what to remove, what to add and where, plus
materials, palette and light — so the result reads as one coherent, gorgeous, photorealistic room
with the architecture unchanged.`

// Brief is stage 2's output for one room — the vision, decomposed, plus the ready-to-run
// transform prompt.
type Brief struct {
	CurrentState         string `json:"current_state"`
	VisionInterpretation string `json:"vision_interpretation"`
	Remove               string `json:"remove"`
	Add                  string `json:"add"`
	Style                string `json:"style"`
	TransformPrompt      string `json:"transform_prompt"`
}

// Director is a Claude-backed art director.
type Director struct {
	client anthropic.Client
}

// NewDirector builds a Director. The Anthropic client reads ANTHROPIC_API_KEY.
func NewDirector() Director {
	return Director{client: anthropic.NewClient()}
}

// UnderstandRoom produces a re-staging brief for a single room. narration (the owner's
// spoken vision) and marketContext may be empty.
func (d Director) UnderstandRoom(ctx context.Context, image []byte, mimeType, roomLabel, narration, marketContext string) (Brief, error) {
	intro := "Room to re-stage at its full potential."
	if strings.TrimSpace(roomLabel) != "" {
		intro = fmt.Sprintf("Room to re-stage at its full potential: %s.", roomLabel)
	}
	if strings.TrimSpace(marketContext) != "" {
		intro += " Market context: " + strings.TrimSpace(marketContext) + "."
	}

	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock(intro),
		anthropic.NewImageBlockBase64(mimeType, base64.StdEncoding.EncodeToString(image)),
	}
	if strings.TrimSpace(narration) != "" {
		blocks = append(blocks, anthropic.NewTextBlock("Owner/agent narration (THE VISION to realise):\n"+strings.TrimSpace(narration)))
	} else {
		blocks = append(blocks, anthropic.NewTextBlock("No narration — infer the most desirable, market-appropriate vision yourself."))
	}
	blocks = append(blocks, anthropic.NewTextBlock("Imagine the space at its full potential, then call record_brief exactly once."))

	strProp := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	tool := anthropic.ToolParam{
		Name:        "record_brief",
		Description: anthropic.String("Record the re-staging brief for this room."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"current_state":         strProp("What the room contains now — the dated furniture and clutter to be cleared."),
				"vision_interpretation": strProp("Your ambitious, tasteful reading of the desired end state (from the narration, or inferred)."),
				"remove":                strProp("What to strip out entirely."),
				"add":                   strProp("What to bring in and where it goes (furniture, decor, textiles, greenery, lighting), as a layout."),
				"style":                 strProp("Materials, palette, lighting and mood — the coherent design language."),
				"transform_prompt":      strProp("ONE vivid, concrete instruction an image/video model runs directly: what to remove, what to add & where, plus materials/palette/light. Architecture unchanged; photorealistic."),
			},
			Required:    []string{"current_state", "vision_interpretation", "remove", "add", "style", "transform_prompt"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	// Fable 5 has thinking always on, which is incompatible with a forced tool_choice —
	// so we leave tool_choice on auto and instruct the model to call the single tool.
	resp, err := d.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 8192,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:     []anthropic.ToolUnionParam{{OfTool: &tool}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return Brief{}, fmt.Errorf("claude understand request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue // skip thinking / text blocks
		}
		var b Brief
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &b); err != nil {
			return Brief{}, fmt.Errorf("parse brief: %w", err)
		}
		return b, nil
	}
	return Brief{}, fmt.Errorf("model returned no brief (stop reason: %s)", resp.StopReason)
}
