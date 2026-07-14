// Package judge implements the regression harness's QUALITY judge — the second half
// of the golden-set check (VERIFY covers honesty; judge covers quality/aesthetic).
//
// When a prompt or model changes, we re-run every approved golden room and ask: is the
// new CANDIDATE still at least as good as the previously-APPROVED reference, in the same
// intended aesthetic, with no NEW defects? Image generation is non-deterministic, so this
// judges the *bar*, not a pixel match.
package judge

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

// model is the judge — the strongest model, same policy as the VERIFY gate.
const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You are the QUALITY JUDGE for Latent Frame's regression harness. Latent Frame
re-stages a dated property photo into an aspirational "after". As we tune prompts and swap models,
we must not silently REGRESS rooms that already looked good. You catch that.

You are shown up to three images of ONE room:
  BEFORE     — the original dated photo (context).
  REFERENCE  — a previously APPROVED "after" (the bar to hold).
  CANDIDATE  — a newly generated "after" to judge against that bar.

Image generation is non-deterministic, so CANDIDATE and REFERENCE will NOT be identical — do NOT
judge pixel similarity. Judge whether CANDIDATE still clears the bar:
  - Quality/wow: is it at least as attractive and aspirational as REFERENCE?
  - Aesthetic: does it hit the same intended look (warm Mediterranean — living/bedroom cozy-riad,
    bathroom clean-spa, kitchen refined warm-wood/cream)?
  - NEW defects it must NOT introduce vs REFERENCE: dated furniture kept instead of replaced, a wall
    left the old colour, furniture blocking a door, a washed-out/pale sky, obvious AI warping, or a
    markedly worse composition.

Set meets_bar = true only if CANDIDATE is a valid NON-regression: as good or better than REFERENCE,
in the right aesthetic, with no new defects. If it is clearly worse or introduces a new defect,
meets_bar = false. Look carefully, then call record_judgment exactly once.`

// Verdict is the judge's output for one candidate-vs-reference comparison.
type Verdict struct {
	MeetsBar            bool   `json:"meets_bar"`
	QualityVsReference  string `json:"quality_vs_reference"` // "better" | "same" | "worse"
	Reason              string `json:"reason"`
}

// OK reports whether the candidate is a non-regression.
func (v Verdict) OK() bool { return v.MeetsBar }

// Judge is a Claude-backed quality judge.
type Judge struct {
	client anthropic.Client
}

// NewJudge builds a Judge. The Anthropic client reads ANTHROPIC_API_KEY.
func NewJudge() Judge {
	return Judge{client: anthropic.NewClient()}
}

// JudgePair scores candidate against reference for the same room. beforePath is optional
// context (may be empty).
func (j Judge) JudgePair(ctx context.Context, beforePath, candidatePath, referencePath, roomLabel string) (Verdict, error) {
	blocks := []anthropic.ContentBlockParamUnion{}
	if strings.TrimSpace(beforePath) != "" {
		bt, bd, err := loadImage(beforePath)
		if err != nil {
			return Verdict{}, fmt.Errorf("load before image: %w", err)
		}
		blocks = append(blocks, anthropic.NewTextBlock("BEFORE (original dated photo, context):"), anthropic.NewImageBlockBase64(bt, bd))
	}
	rt, rd, err := loadImage(referencePath)
	if err != nil {
		return Verdict{}, fmt.Errorf("load reference image: %w", err)
	}
	ct, cd, err := loadImage(candidatePath)
	if err != nil {
		return Verdict{}, fmt.Errorf("load candidate image: %w", err)
	}

	intro := "Judge the CANDIDATE against the approved REFERENCE for this room."
	if strings.TrimSpace(roomLabel) != "" {
		intro = fmt.Sprintf("Judge the CANDIDATE against the approved REFERENCE for this room (%s).", roomLabel)
	}
	blocks = append(blocks,
		anthropic.NewTextBlock("REFERENCE (previously approved after — the bar to hold):"),
		anthropic.NewImageBlockBase64(rt, rd),
		anthropic.NewTextBlock("CANDIDATE (new after to judge):"),
		anthropic.NewImageBlockBase64(ct, cd),
		anthropic.NewTextBlock(intro+" Then call record_judgment."),
	)

	strProp := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	tool := anthropic.ToolParam{
		Name:        "record_judgment",
		Description: anthropic.String("Record whether the candidate holds the quality bar set by the reference."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"meets_bar":            map[string]any{"type": "boolean", "description": "True only if the candidate is a non-regression: as good or better than the reference, right aesthetic, no new defects."},
				"quality_vs_reference": map[string]any{"type": "string", "enum": []string{"better", "same", "worse"}, "description": "How the candidate's quality compares to the reference."},
				"reason":               strProp("One or two sentences justifying the judgment; if meets_bar is false, name the regression/new defect."),
			},
			Required:    []string{"meets_bar", "quality_vs_reference", "reason"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}

	resp, err := j.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      model,
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceParamOfTool("record_judgment"),
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("claude judge request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var v Verdict
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &v); err != nil {
			return Verdict{}, fmt.Errorf("parse judgment: %w", err)
		}
		return v, nil
	}
	return Verdict{}, fmt.Errorf("model returned no judgment (stop reason: %s)", resp.StopReason)
}

// loadImage reads an image and returns its media type (sniffed from bytes) + base64.
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
