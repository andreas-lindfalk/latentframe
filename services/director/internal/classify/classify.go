// Package classify implements the SCENE-CLASSIFY + CURATE stage: point it at a folder
// of listing photos and it returns a curated property model — every space identified,
// duplicate angles collapsed to one hero, spaces categorised (interior / outdoor-private
// / shared), and, crucially, EDITED DOWN to a tight set worth showing ("less is more").
//
// It is Claude-vision only. It spends nothing at fal/Veo — instead it proposes a plan
// (which spaces to restage, in which styles, which to animate) plus a cost estimate, so a
// human can clear it BEFORE any paid generation runs. This is the credit-saving gate.
package classify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andreas-lindfalk/latentframe/pkg/propertymodel"
	"github.com/anthropics/anthropic-sdk-go"
	xdraw "golang.org/x/image/draw"
)

// The curation/understanding task is reasoning-heavy over many images — use a top model.
const model = anthropic.ModelClaudeOpus4_8

const systemPrompt = `You are the art director AND photo editor for Latent Frame, a product that restages
dated property listings to show their potential.

You are shown ALL the photos from ONE property's listing, each labelled "PHOTO #N".
Your job is to UNDERSTAND the property and CURATE it — not to use every photo.

Do all of this, then call record_property exactly once:

FIRST — classify the PROPERTY TYPE. Pick the SINGLE best fit from this fixed taxonomy, and
use it to guide the private-vs-shared outdoor call:
  - "apartment": a unit inside a multi-unit block, usually one level; NO private ground
    garden. Private outdoor = balcony/terrace only; any ground pool/garden is COMMUNAL.
  - "penthouse": a top-floor apartment, usually with a large PRIVATE roof terrace/solarium
    and views (the rooftop is the hero); any ground pool/garden is COMMUNAL.
  - "townhouse": a multi-level house sharing walls in a row/terrace, own entrance, often a
    small PRIVATE patio/terrace; frequently in a gated community with a SHARED pool/garden.
  - "semi-detached": a house sharing ONE wall with a neighbour, with its OWN PRIVATE garden
    and its facade in view.
  - "villa": a fully DETACHED house standing alone, with a PRIVATE garden/plot (often room
    for a pool) and its facade prominent in outdoor shots.
Default the outdoor categorisation from the type: apartment/penthouse → a ground pool &
garden are almost always "shared" (enhance-context); semi-detached/villa → the garden is
"outdoor_private" (restyle); townhouse → a small private terrace plus often shared community
amenities. When the photos genuinely contradict the type's default, TRUST THE PHOTOS.

Then, for the spaces:

1. IDENTIFY each space and what it is (living room, kitchen, bathroom, bedroom, dining,
   covered terrace, balcony, rooftop solarium, garden, pool, facade, hallway/landing, etc.).
2. GROUP multiple angles of the SAME physical space together and pick ONE hero photo
   (the clearest, best-composed, most flattering angle).
3. CATEGORISE each space: "interior", "outdoor_private" (the property's own terraces,
   balconies, solarium, private garden), or "shared" (communal pool/garden, the street,
   the building facade).
4. CURATE — this is the point. Select a TIGHT set of spaces that tells this property's
   story at its best. LESS IS MORE. Skip redundant near-duplicate rooms (e.g. a 4th
   similar bedroom), weak or cluttered photos, circulation/landings, and anything that
   doesn't add to the pitch. Set selected=true only for the spaces worth showing.
5. RANK each selected space's showcase_value: "hero" (the wow spaces — usually the best
   living space + the standout outdoor spaces), "strong", or "supporting".
6. Set restage_tier:
   - "restyle" for private spaces we will re-stage (interior rooms + private outdoor).
   - "enhance-context" for SHARED spaces (communal pool/garden, facade) — real and shared,
     shown for lifestyle context only. NEVER imply a shared amenity is private.
   - "skip" for spaces not worth staging.
7. Propose animate=true for only the 2-3 STRONGEST hero spaces (the ones a short moving
   reel would sell hardest). Everything else animate=false.
8. List every photo you did NOT use in "excluded", with a one-line reason (duplicate of
   #N, low quality, redundant, circulation, etc.).

HONESTY (hard rules): never invent architecture, and never propose adding a pool, an
extension or any structure that is not already there. If a space physically has no room
for something, it does not get it. Shared amenities are real but shared — context only.

Keep "current" and "potential" to one vivid sentence each. Use the photo's #N number for
all indexes.`

// The curated property model types live in pkg/propertymodel (shared with render).

// Options configures Classify.
type Options struct {
	Name     string
	Location string
	MaxPhotos int // safety cap; 0 = default
	Logf     func(format string, args ...any)
}

// Classify reads the images in dir and returns a curated property model. Claude only —
// no fal/Veo. Handles a single vision call (good to ~40 photos); larger folders are
// capped with a warning until batching lands.
func Classify(ctx context.Context, dir string, opts Options) (propertymodel.Model, error) {
	log := opts.Logf
	if log == nil {
		log = func(string, ...any) {}
	}
	files, err := listImages(dir)
	if err != nil {
		return propertymodel.Model{}, err
	}
	if len(files) == 0 {
		return propertymodel.Model{}, fmt.Errorf("no images found in %s", dir)
	}
	maxN := opts.MaxPhotos
	if maxN <= 0 {
		maxN = 40
	}
	if len(files) > maxN {
		log("⚠ %d photos > cap %d — classifying the first %d (batching for larger folders is TODO)", len(files), maxN, maxN)
		files = files[:maxN]
	}

	index := map[string]string{}
	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock(header(opts) + fmt.Sprintf("\n\nThere are %d photos. Study them all, then call record_property once.", len(files))),
	}
	for i, f := range files {
		n := i + 1
		index[fmt.Sprint(n)] = filepath.Base(f)
		jpg, err := loadThumb(f, 768)
		if err != nil {
			return propertymodel.Model{}, fmt.Errorf("thumbnail %s: %w", filepath.Base(f), err)
		}
		blocks = append(blocks,
			anthropic.NewTextBlock(fmt.Sprintf("PHOTO #%d:", n)),
			anthropic.NewImageBlockBase64("image/jpeg", base64.StdEncoding.EncodeToString(jpg)),
		)
	}

	tool := propertyTool()
	log("classifying %d photos with %s …", len(files), model)
	client := anthropic.NewClient()
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 16384,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Tools:     []anthropic.ToolUnionParam{{OfTool: &tool}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(blocks...)},
	})
	if err != nil {
		return propertymodel.Model{}, fmt.Errorf("claude classify request: %w", err)
	}

	for _, block := range resp.Content {
		tu, ok := block.AsAny().(anthropic.ToolUseBlock)
		if !ok {
			continue
		}
		var m propertymodel.Model
		if err := json.Unmarshal([]byte(tu.JSON.Input.Raw()), &m); err != nil {
			return propertymodel.Model{}, fmt.Errorf("parse property model: %w", err)
		}
		m.PhotoIndex = index
		if opts.Name != "" {
			m.Property.Name = opts.Name
		}
		if opts.Location != "" {
			m.Property.Location = opts.Location
		}
		return m, nil
	}
	return propertymodel.Model{}, fmt.Errorf("model returned no property model (stop reason: %s)", resp.StopReason)
}

func header(o Options) string {
	h := "Classify and curate this property's listing photos."
	if o.Name != "" {
		h += " Property: " + o.Name + "."
	}
	if o.Location != "" {
		h += " Location: " + o.Location + "."
	}
	return h
}

func propertyTool() anthropic.ToolParam {
	str := func(d string) map[string]any { return map[string]any{"type": "string", "description": d} }
	enum := func(d string, vals ...string) map[string]any {
		return map[string]any{"type": "string", "description": d, "enum": vals}
	}
	spaceProps := map[string]any{
		"id":             str("short kebab-case id, e.g. 'living', 'balcony-2f'"),
		"name":           str("display name, e.g. 'Living room', 'Rooftop solarium'"),
		"type":           str("space type, e.g. living_room, kitchen, bathroom, bedroom, balcony, roof_solarium, terrace, garden, pool, facade, circulation"),
		"category":       enum("interior | outdoor_private | shared", "interior", "outdoor_private", "shared"),
		"photo_indexes":  map[string]any{"type": "array", "description": "all photo #N of this space", "items": map[string]any{"type": "integer"}},
		"hero_index":     map[string]any{"type": "integer", "description": "the best photo #N for this space"},
		"current":        str("one vivid sentence: what the space is like now"),
		"potential":      str("one vivid sentence: what it could become (honest — no invented structures/pools)"),
		"showcase_value": enum("hero | strong | supporting", "hero", "strong", "supporting"),
		"restage_tier":   enum("restyle | enhance-context | skip", "restyle", "enhance-context", "skip"),
		"selected":       map[string]any{"type": "boolean", "description": "true = show on the page (curate tightly; less is more)"},
		"animate":        map[string]any{"type": "boolean", "description": "true = propose a moving reel (only 2-3 strongest heroes)"},
		"reason":         str("one line: why selected / skipped / animated"),
	}
	spaceReq := []string{"id", "name", "type", "category", "photo_indexes", "hero_index", "current", "potential", "showcase_value", "restage_tier", "selected", "animate", "reason"}

	return anthropic.ToolParam{
		Name:        "record_property",
		Description: anthropic.String("Record the curated property model."),
		Strict:      anthropic.Bool(true),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"property": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":            str("property name if visible/known, else empty"),
						"location":        str("location if known, else empty"),
						"type":            enum("property type — pick the SINGLE best fit from the taxonomy defined above", propertymodel.PropertyTypes...),
						"sleeps_estimate": map[string]any{"type": "integer", "description": "rough sleeps count from the bedrooms"},
					},
					"required":             []string{"name", "location", "type", "sleeps_estimate"},
					"additionalProperties": false,
				},
				"spaces": map[string]any{
					"type":        "array",
					"description": "every distinct space (angles grouped), curated",
					"items": map[string]any{
						"type":                 "object",
						"properties":           spaceProps,
						"required":             spaceReq,
						"additionalProperties": false,
					},
				},
				"excluded": map[string]any{
					"type":        "array",
					"description": "photos not used, with reasons",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"photo_index": map[string]any{"type": "integer"},
							"reason":      str("why excluded (duplicate of #N, low quality, redundant, circulation, ...)"),
						},
						"required":             []string{"photo_index", "reason"},
						"additionalProperties": false,
					},
				},
			},
			Required:    []string{"property", "spaces", "excluded"},
			ExtraFields: map[string]any{"additionalProperties": false},
		},
	}
}

// FormatPlan renders the human-readable clearance plan + a rough cost estimate.
// It calls nothing paid — it's the gate a human reviews before generation runs.
func FormatPlan(m propertymodel.Model, nStyles int) string {
	var selPrivate, videos, ctxShots int
	for _, s := range m.Spaces {
		if !s.Selected {
			continue
		}
		switch s.RestageTier {
		case "restyle":
			selPrivate++
		case "enhance-context":
			ctxShots++
		}
		if s.Animate {
			videos++
		}
	}
	restages := selPrivate * nStyles

	var b strings.Builder
	f := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	f("\n==============================================================\n")
	f("  PROPOSED PLAN — %s\n", orDash(m.Property.Name))
	f("  %s · %s · sleeps ~%d\n", orDash(m.Property.Type), orDash(m.Property.Location), m.Property.Sleeps)
	f("==============================================================\n")

	for _, c := range []struct{ key, label string }{
		{"interior", "INSIDE"}, {"outdoor_private", "OUTSIDE (private)"}, {"shared", "THE SETTING (shared)"},
	} {
		first := true
		for _, s := range m.Spaces {
			if s.Category != c.key || !s.Selected {
				continue
			}
			if first {
				f("\n  %s\n", c.label)
				first = false
			}
			tags := s.ShowcaseValue
			if s.RestageTier == "enhance-context" {
				tags += " · context"
			}
			if s.Animate {
				tags += " · reel"
			}
			f("   - %-22s [%s]  hero=%s\n", s.Name, tags, m.PhotoIndex[fmt.Sprint(s.HeroIndex)])
			f("       %s\n", s.Potential)
		}
	}

	var skipped []string
	for _, s := range m.Spaces {
		if !s.Selected {
			skipped = append(skipped, s.Name)
		}
	}
	if len(skipped) > 0 {
		f("\n  SKIPPED spaces: %s\n", strings.Join(skipped, ", "))
	}
	if len(m.Excluded) > 0 {
		f("  EXCLUDED photos: %d\n", len(m.Excluded))
	}

	loR, hiR := restages*8, restages*15 // nano-banana-2..pro, cents/img
	loV, hiV := videos*30, videos*60    // veo reel, rough cents/clip
	f("\n  -- PLAN ---------------------------------------------------\n")
	f("   restages : %d   (%d selected private spaces x %d styles)\n", restages, selPrivate, nStyles)
	f("   context  : %d   shared shots (shown as-is, no restage)\n", ctxShots)
	f("   reels    : %d   (image -> video)\n", videos)
	f("   est spend: ~$%.2f - $%.2f   (NOTHING sent to fal/Veo yet)\n", float64(loR+loV)/100, float64(hiR+hiV)/100)
	f("==============================================================\n")
	f("  CLEARANCE REQUIRED — review, then run the paid generation.\n")
	return b.String()
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func listImages(dir string) ([]string, error) {
	var out []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".png", ".jpg", ".jpeg", ".webp":
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// loadThumb reads an image and returns a downscaled JPEG (long edge <= maxDim) — small
// enough to send many at once, and plenty of detail for classification. PNG/JPEG in.
func loadThumb(path string, maxDim int) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if w > maxDim || h > maxDim {
		if w >= h {
			nw, nh = maxDim, h*maxDim/w
		} else {
			nw, nh = w*maxDim/h, maxDim
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
