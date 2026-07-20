// Package describe writes the per-room, per-style captions shown above each image on
// the showcase page. For every selected restyle space it asks Claude for a short
// (<=2 sentence) caption of that room in each decoration style — concrete about
// materials/palette/mood — and stores them on the space (Descriptions).
package describe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/andreas-lindfalk/latentframe/pkg/propertymodel"
	"github.com/anthropics/anthropic-sdk-go"
)

const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You write short, evocative captions for a property restaging showcase.
For each room, in EACH decoration style, write a caption of AT MOST TWO sentences describing
that room in that style — concrete about materials, palette and mood, aspirational but
grounded and specific to the actual room. No clichés, no filler, do not start with "Imagine".
Then call record_descriptions exactly once.`

// Style carries what the caption writer needs to know about a style.
type Style struct{ ID, Name, Flavor string }

// Describe fills Descriptions (styleId -> caption) for every selected restyle space.
func Describe(ctx context.Context, m propertymodel.Model, styles []Style, logf func(string, ...any)) (propertymodel.Model, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var roomLines []string
	for _, s := range m.Spaces {
		if !s.Selected || s.RestageTier != propertymodel.TierRestyle {
			continue
		}
		roomLines = append(roomLines, fmt.Sprintf("- id=%q  name=%q  type=%q  now: %s  potential: %s", s.ID, s.Name, s.Type, s.Current, s.Potential))
	}
	if len(roomLines) == 0 {
		return m, nil
	}

	var styleLines []string
	var styleKeys []string
	for _, st := range styles {
		styleLines = append(styleLines, fmt.Sprintf("- %s (%s): %s", st.ID, st.Name, st.Flavor))
		styleKeys = append(styleKeys, st.ID)
	}

	user := fmt.Sprintf("Property: %s — %s.\n\nStyles:\n%s\n\nRooms:\n%s\n\nFor every room id, write a caption (<=2 sentences) in each style. Call record_descriptions once.",
		orDash(m.Property.Name), orDash(m.Property.Location), strings.Join(styleLines, "\n"), strings.Join(roomLines, "\n"))

	itemProps := map[string]any{"space_id": map[string]any{"type": "string", "description": "the room id"}}
	itemReq := []string{"space_id"}
	for _, k := range styleKeys {
		itemProps[k] = map[string]any{"type": "string", "description": "<=2 sentence caption of this room in the " + k + " style"}
		itemReq = append(itemReq, k)
	}
	tool := anthropic.ToolParam{
		Name:        "record_descriptions",
		Description: anthropic.String("Record per-room, per-style captions."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"descriptions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":                 "object",
						"properties":           itemProps,
						"required":             itemReq,
						"additionalProperties": false,
					},
				},
			},
			Required:    []string{"descriptions"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	logf("describing %d rooms × %d styles with %s …", len(roomLines), len(styles), model)
	client := anthropic.NewClient()
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 8192,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:     []anthropic.ToolUnionParam{{OfTool: &tool}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
	})
	if err != nil {
		return m, fmt.Errorf("describe request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var out struct {
			Descriptions []map[string]string `json:"descriptions"`
		}
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &out); err != nil {
			return m, fmt.Errorf("parse descriptions: %w", err)
		}
		byID := map[string]map[string]string{}
		for _, d := range out.Descriptions {
			id := d["space_id"]
			if id == "" {
				continue
			}
			caps := map[string]string{}
			for _, k := range styleKeys {
				if v := strings.TrimSpace(d[k]); v != "" {
					caps[k] = v
				}
			}
			byID[id] = caps
		}
		filled := 0
		for i := range m.Spaces {
			if c, ok := byID[m.Spaces[i].ID]; ok && len(c) > 0 {
				m.Spaces[i].Descriptions = c
				filled++
			}
		}
		logf("✓ captioned %d rooms", filled)
		return m, nil
	}
	return m, fmt.Errorf("no descriptions returned (stop reason: %s)", resp.StopReason)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "a property"
	}
	return s
}
