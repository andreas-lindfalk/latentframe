// Package verify implements the honesty gate — pipeline stage 4 (VERIFY).
//
// It shows Claude the BEFORE (original hero frame) and AFTER (proposed re-staged
// image) and asks one question: was the architecture preserved, and is the result
// believable? This is the enforcement point for the product's inviolable rule —
// RE-STAGE, NEVER RESTRUCTURE — and it fails closed (see the system prompt).
package verify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	xdraw "golang.org/x/image/draw"

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
  • adds, removes, enlarges, shrinks or moves any window, door (INTERIOR or exterior), doorway
    or opening — the exact set, type and position of windows, doors and openings must match the
    BEFORE; also fails if it adds a closed door leaf where the BEFORE has an open passage, or
    opens a solid wall/passage where the BEFORE has none;
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

// inspireSystemPrompt is the relaxed "potential / inspire" bar (Andreas 2026-07-14): outputs
// are framed as an aspirational vision of what a dated room COULD become, not documentation.
// We give full creative freedom on style AND decorative architecture, and only stop a render
// that would mislead a visiting lead about the "buyable facts" — size, light, view, structure.
const inspireSystemPrompt = `You are the honesty gate for Latent Frame, in INSPIRE mode. Latent Frame shows
a property's *potential* — an aspirational vision of what a dated room could become. The goal is to INSPIRE
a buyer, not to document the current state, so generous creative re-imagining is EXPECTED and desirable.
Be deliberately LENIENT about style and decoration.

You are shown two images of the SAME room:
  BEFORE — the real, current photo.
  AFTER  — an aspirational AI "potential" version.

Protect ONE thing: the AFTER must still be a believable potential of THIS ACTUAL SPACE, so a lead who
visits recognises the place and isn't misled about the facts they'd buy on. Judge ONLY these "buyable
facts" — set architecture_preserved = false ONLY if the AFTER misrepresents one of them:
  • ROOM SIZE / FOOTPRINT — the space is made materially bigger or smaller, or rooms are merged, so the
    sense of scale is wrong;
  • NATURAL LIGHT — a clearly dim room is shown flooded with daylight it doesn't have, or windows are
    removed so a bright room looks dark. (Restyling or modestly reshaping a window is fine, as long as the
    room's overall daylight level stays honest.);
  • INVENTED VIEW — an outdoor view is fabricated that cannot exist (e.g. a sea/mountain/pool view where
    the real window looks onto a wall or street);
  • STRUCTURAL WALLS / LAYOUT — a solid, likely load-bearing wall is knocked through, or a whole functional
    area is removed, misrepresenting the real layout.

Everything else is ALLOWED and must NOT lower architecture_preserved — this is the product working:
  • all furniture, finishes, paint, flooring, tiles, worktops, fixtures, decor, plants, lighting;
  • DECORATIVE architecture — adding or removing beams, arches, niches, plaster texture, wainscoting,
    columns, mouldings or a feature fireplace; restyling window frames and reasonably reshaping openings;
  • warm styling, staging and a lifted, inviting mood.

CAMERA: the AFTER is re-rendered and may be shot from a slightly different angle — never treat a viewpoint
difference as a change. When UNSURE, LEAN TOWARD PASSING — in inspire mode we only stop a render that would
genuinely mislead a visiting lead about size, light, view or structure. Set believable = false only for
obvious AI artifacts or warped geometry. Call record_verdict exactly once.`

// architecture_preserved descriptions, per mode.
const (
	strictArchDesc = "True if the building SHELL is unchanged — same walls, windows, exterior openings, room shape/proportions/ceiling, no functional area removed, and no fixtures relocated to a different wall. Replacing fixtures/finishes/furniture IN PLACE (e.g. tub→walk-in shower in the same wet zone, new kitchen along the same wall) does NOT make this false. A slightly different camera angle does NOT make this false."
	inspireArchDesc = "True if the room's BUYABLE FACTS are preserved — same size/footprint, an honest natural-light level, no invented outdoor view, and no removed structural walls or functional areas. DECORATIVE changes (added/removed beams, arches, niches, plaster, mouldings, restyled or modestly reshaped window frames) and ALL finishes/furniture/fixtures/decor do NOT make this false. Lean toward true unless a visiting lead would be misled about size, light, view or structure."
)

// Gate is a Claude-backed pipeline.Verifier. It runs in one of two modes: the default
// STRICT shell-vs-contents bar, or the relaxed INSPIRE bar (see the two system prompts).
type Gate struct {
	client       anthropic.Client
	systemPrompt string
	archDesc     string
	votes        int // independent judge passes per pair; a pair must pass ALL (fail-closed). <=1 = single pass.
}

// NewGate builds a STRICT gate (shell-vs-contents). The Anthropic client reads
// ANTHROPIC_API_KEY from the environment. This is the default used by the pipeline,
// the golden harness and best-of-N.
func NewGate() Gate {
	return Gate{client: anthropic.NewClient(), systemPrompt: systemPrompt, archDesc: strictArchDesc, votes: 1}
}

// NewInspireGate builds a gate on the relaxed "potential / inspire" bar — full creative
// freedom on style and decorative architecture; fails only material misrepresentations of
// size, natural light, view or structure.
func NewInspireGate() Gate {
	return Gate{client: anthropic.NewClient(), systemPrompt: inspireSystemPrompt, archDesc: inspireArchDesc, votes: 1}
}

// Voted returns a copy of the gate that runs n INDEPENDENT judge passes on each pair and
// passes it ONLY if every pass agrees it is honest and believable (fail-closed on any
// dissent). Because a single VLM pass is stochastic and occasionally MISSES a subtle added
// window/door, requiring unanimous agreement across passes raises defect recall — use it for
// structurally stubborn spaces (kitchens, bathrooms). It trades a few extra Claude calls and
// some false rejections (harmless under best-of-N) for far fewer missed slips. n<=1 = single pass.
func (g Gate) Voted(n int) Gate {
	if n < 1 {
		n = 1
	}
	g.votes = n
	return g
}

var _ pipeline.Verifier = Gate{}

// Verify implements pipeline.Verifier using the room's hero frame as BEFORE and its
// re-staged still as AFTER.
func (g Gate) Verify(ctx context.Context, room *pipeline.Room) (pipeline.Verdict, error) {
	return g.VerifyPair(ctx, room.Hero.Path, room.AfterStill, room.Label)
}

// VerifyPair judges a single before/after image pair. roomLabel is optional context
// (e.g. "kitchen") and may be empty. If the gate was built with Voted(n>1), it runs up to
// n independent passes and fails closed on the FIRST dissent (short-circuit) — a pair passes
// only if every pass agrees. A pass error is returned immediately.
func (g Gate) VerifyPair(ctx context.Context, beforePath, afterPath, roomLabel string) (pipeline.Verdict, error) {
	votes := g.votes
	if votes < 1 {
		votes = 1
	}
	var last pipeline.Verdict
	for i := 0; i < votes; i++ {
		v, err := g.verifyOnce(ctx, beforePath, afterPath, roomLabel)
		if err != nil {
			return pipeline.Verdict{}, err
		}
		last = v
		if !v.OK() {
			return v, nil // any dissenting pass rejects the pair (fail-closed)
		}
	}
	return last, nil // unanimous pass
}

// verifyOnce runs a SINGLE judge pass on one before/after pair.
func (g Gate) verifyOnce(ctx context.Context, beforePath, afterPath, roomLabel string) (pipeline.Verdict, error) {
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
					"description": g.archDesc,
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
		System:     []anthropic.TextBlockParam{{Text: g.systemPrompt}},
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

// maxEdge caps the long edge of an image sent to the vision model. Anthropic downscales
// to ~1568px server-side anyway and rejects images over 10MB base64, so we pre-shrink large
// photos here. Without this, real listing photos (e.g. 3072×1728, ~8MB PNG) tip over the
// 10MB base64 limit and the gate's API call ERRORS — which best-of-N counts as "not passed",
// producing a false 0/4 that looks like structural drift. (Zeniamar's 1024×800 photos never
// hit this; the second property surfaced it.) 1568px keeps ample detail to judge structure.
const maxEdge = 1568

// rawFastPath is the raw-byte size under which we skip decode/resize — ~5.3MB base64,
// safely under the 10MB limit.
const rawFastPath = 4 << 20

// loadImage reads an image file and returns its media type + base64 data, downscaling
// oversized images so the gate never trips the 10MB limit. It detects the type from the
// bytes, not the filename — generators often return PNG even when named .jpg, and Anthropic
// rejects a mismatched media type.
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
	// Small enough already → send as-is (fast path).
	if len(raw) < rawFastPath {
		return mediaType, base64.StdEncoding.EncodeToString(raw), nil
	}
	// Large file → decode, downscale the long edge to maxEdge, re-encode as JPEG.
	img, _, derr := image.Decode(bytes.NewReader(raw))
	if derr != nil {
		// Undecodable (e.g. webp without a registered decoder) — fall back to raw.
		return mediaType, base64.StdEncoding.EncodeToString(raw), nil
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	long := w
	if h > w {
		long = h
	}
	if long > maxEdge {
		scale := float64(maxEdge) / float64(long)
		dst := image.NewRGBA(image.Rect(0, 0, int(float64(w)*scale), int(float64(h)*scale)))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return "", "", fmt.Errorf("re-encode %s: %w", path, err)
	}
	return "image/jpeg", base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
