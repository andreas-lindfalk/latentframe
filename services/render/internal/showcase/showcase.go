// Package showcase is the multi-style RESTAGE stage behind the agent-facing
// property page (web/showcase). Given a property spec (rooms + their hero photos),
// it restages each room in several decoration styles via the Nano-Banana engine
// and emits the page's data manifest (data/property.json).
//
// It is the in-repo, reproducible replacement for the throwaway script that first
// produced the Zeniamar renders: pipeline output now feeds the page directly.
//
// The instruction folds in the restage-playbook rules learned from per-image
// corrections (recolour every wall, keep doors clear, restyle all cabinetry
// consistently, never mirror the room) so each style variant inherits the flywheel.
package showcase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/andreas-lindfalk/latentframe/pkg/imageedit"
	"github.com/andreas-lindfalk/latentframe/pkg/propertymodel"
	"github.com/andreas-lindfalk/latentframe/pkg/verify"
)

// Product is one shop-the-look item.
type Product struct {
	Name     string `json:"name"`
	Retailer string `json:"retailer"`
	Price    string `json:"price"`
	URL      string `json:"url"`
}

// Style is a decoration style (id drives the accent in the page's CSS).
type Style struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tagline string `json:"tagline"`
	Accent  string `json:"accent"`
}

// PropertyMeta is the page header copy.
type PropertyMeta struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Kicker   string `json:"kicker"`
	Lede     string `json:"lede"`
}

// RoomSpec is one room in the input spec: a hero photo to restage + display copy.
type RoomSpec struct {
	ID    string               `json:"id"`
	Name  string               `json:"name"`
	Index string               `json:"index"`
	Blurb string               `json:"blurb"`
	Hero  string               `json:"hero"` // filesystem path to the dated source photo
	Shop  map[string][]Product `json:"shop,omitempty"`
}

// Spec is the input: property meta + rooms (+ optional style override).
type Spec struct {
	Property PropertyMeta `json:"property"`
	Rooms    []RoomSpec   `json:"rooms"`
	Styles   []Style      `json:"styles,omitempty"`
}

// RoomOut is one room in the emitted manifest.
type RoomOut struct {
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Index  string               `json:"index"`
	Blurb  string               `json:"blurb"`
	Before string               `json:"before"`
	After  map[string]string    `json:"after"`
	Shop   map[string][]Product `json:"shop"`
}

// Manifest is the emitted data/property.json the page consumes.
type Manifest struct {
	Version     int          `json:"version"`
	GeneratedBy string       `json:"generatedBy"`
	Property    PropertyMeta `json:"property"`
	Styles      []Style      `json:"styles"`
	Rooms       []RoomOut    `json:"rooms"`
}

// DefaultStyles are the three out-of-the-box looks (accents mirror globals.css).
func DefaultStyles() []Style {
	return []Style{
		{ID: "mediterranean", Name: "Mediterranean", Tagline: "Warm · characterful · sun-washed", Accent: "#BC6437"},
		{ID: "scandinavian", Name: "Scandinavian", Tagline: "Light · calm · pared-back", Accent: "#6E7A5C"},
		{ID: "coastal", Name: "Coastal", Tagline: "Fresh · classic · airy", Accent: "#356381"},
	}
}

// builtinFlavor is the fallback style directive when no prompt file is present.
var builtinFlavor = map[string]string{
	"mediterranean": "Warm, sun-washed Mediterranean: lime-washed cream walls, terracotta and zellige, natural oak and rattan, linen and jute, olive and clay accents, wrought-iron details and greenery.",
	"scandinavian":  "Light, calm Scandinavian: soft white and pale-oak palette, pared-back furniture, natural light, muted textiles, matte-black accents. Uncluttered and airy.",
	"coastal":       "Fresh classic coastal: crisp whites with soft blue and sand accents, painted timber, rattan and linen, brushed brass, breezy sheer curtains. Bright and relaxed.",
}

// roomType maps a room id/name to a natural noun for the instruction.
func roomType(id, name string) string {
	switch strings.ToLower(id) {
	case "living", "livingroom", "lounge":
		return "living room"
	case "kitchen":
		return "kitchen"
	case "bath", "bathroom":
		return "bathroom"
	case "bed", "bedroom", "bedroom_bunk":
		return "bedroom"
	case "dining":
		return "dining area"
	case "balcony":
		return "covered balcony"
	case "roof_solarium", "solarium", "roof_terrace":
		return "rooftop terrace"
	case "covered_terrace", "terrace", "patio", "porch":
		return "terrace"
	case "front_terrace":
		return "front terrace"
	case "side_terrace":
		return "side terrace"
	case "garden", "garden_community":
		return "garden"
	case "facade", "facade_exterior":
		return "frontage"
	}
	if name != "" {
		return strings.ToLower(name)
	}
	return "room"
}

// outdoorTypes marks space types that are outdoor (staged as outdoor living, not rooms).
var outdoorTypes = map[string]bool{
	"balcony": true, "roof_solarium": true, "solarium": true, "roof_terrace": true,
	"covered_terrace": true, "terrace": true, "front_terrace": true, "side_terrace": true,
	"patio": true, "porch": true, "garden": true, "facade": true, "facade_exterior": true,
}

func isOutdoor(spaceType, category string) bool {
	return category == propertymodel.CategoryOutdoor || outdoorTypes[strings.ToLower(spaceType)]
}

// isHouseType is true for property types whose own BUILDING/facade can appear in outdoor
// shots (villa, semi-detached, townhouse) — unlike apartments/penthouses, whose private
// outdoor is a balcony/terrace/solarium without the whole building in frame. House types get
// the facade-in-frame lock so a restage dresses the GROUND but never modifies the building
// (this is the fix for the villa-courtyard gap, where the facade got reshaped/recoloured).
func isHouseType(propertyType string) bool {
	switch propertyType {
	case propertymodel.TypeVilla, propertymodel.TypeSemiDetached, propertymodel.TypeTownhouse:
		return true
	}
	return false
}

// outdoorDirective is how to dress each outdoor space type (paired with a style flavor).
// Vision target = "Spanish outdoor LIVING" — where people SIT, EAT and hang out in the SHADE.
// The HERO is the usable furniture + shade + overhead lighting, NOT the planting: a generous
// weathered-wood DINING table with comfortable woven-rush/rattan or cushioned chairs, and/or
// a deep linen/rattan LOUNGE; clear overhead SHADE (the existing pergola/portico, or a large
// cream fringed parasol on open areas); statement rattan/lantern PENDANTS and warm string
// lights over the table. Planting is a RESTRAINED framing accent — a gnarled olive or agave
// in ONE large urn, a climbing vine, a few herbs — NEVER a crowd of pots (earlier renders
// over-did the pots and under-did the seating/shade). Materials: weathered wood, rattan/cane/
// rush/rope, jute, natural stone, linen, aged brass/black metal, warm terracotta accents. Only
// ADD; the building's structure and COLOUR are locked (see the guard).
var outdoorDirective = map[string]string{
	"roof_solarium":   "an open rooftop sun terrace dressed to its EXACT existing size — a pair of teak sun loungers with cream linen cushions AND a small bistro dining spot for two for sundowner meals, the existing pergola dressed with light drapes, statement string lights and a lantern, a statement potted olive and a couple of lush clusters of lavender and herbs in terracotta for warmth. Do NOT add a large dining set, and do NOT enlarge or reshape the terrace",
	"solarium":        "an open rooftop sun terrace dressed to its EXACT existing size — a pair of teak sun loungers with cream linen cushions AND a small bistro dining spot for two for sundowner meals, the existing pergola dressed with light drapes, statement string lights and a lantern, a statement potted olive and a couple of lush clusters of lavender and herbs in terracotta for warmth. Do NOT add a large dining set, and do NOT enlarge or reshape the terrace",
	"balcony":         "a characterful covered outdoor LOUNGE-and-dining nook built for sitting, eating and hanging out — a deep low rattan or built-in-look sofa and an armchair with generous cream linen cushions and ochre pillows, a rustic wood coffee table, AND a small rustic bistro dining table for two set to one side to eat out here, a jute rug, a statement rattan pendant or lanterns and warm string lights overhead, and GENEROUS but tidy greenery — a statement potted olive or palm in a corner and a couple of lush plant clusters (trailing greenery, herbs) for warmth and life. The comfortable seating is the hero, but keep it green and characterful, not bare. KEEP THE FRONT DOOR and a clear walkway to it completely unobstructed — arrange all furniture AWAY from the door so you could walk straight in",
	"covered_terrace": "an inviting covered outdoor DINING-and-living room — a generous rustic wood DINING table with comfortable woven-rush or cushioned chairs for long lunches, PLUS a deep rattan/linen sofa lounge, a large jute rug, statement rattan pendants or lanterns and string lights over the table, and GENEROUS but tidy greenery — statement potted olive, citrus or palm trees and lush clusters of pots (lavender, herbs, trailing greenery) that frame the space and fill the corners. Furniture and shade lead, but keep it green and characterful, not bare. SPATIAL RULE: the entrance door is on one wall and the OPEN (arch/view) side is opposite — put the DINING TABLE and the main seating toward the OPEN side, and keep the DOOR WALL and the whole area in front of the door completely CLEAR (at most a slim console or a potted plant beside it). NEVER place the dining table, a sofa or chairs in front of, across, or blocking the door — you must be able to walk straight in.",
	"terrace":         "an inviting outdoor DINING-and-lounge terrace centred on a generous rustic wood DINING table with comfortable woven or cushioned chairs for long al-fresco meals, PLUS a deep linen/rattan lounge, shaded by the existing cover or a large cream fringed parasol, with statement pendants/lanterns and string lights strung overhead and a jute rug. Frame it with GENEROUS, tidy greenery — a statement olive, citrus or palm in a corner and a few lush clusters of terracotta pots (lavender, herbs, trailing greenery) — lush and characterful, without burying the seating in pots. The seating, dining and shade are the hero",
	"front_terrace":   "a welcoming front terrace built around a rustic wood DINING table with comfortable chairs for al-fresco meals, plus a small lounge, shaded by a large cream parasol, with lanterns and string lights overhead and a jute rug, framed by generous tidy greenery — a statement olive or citrus in a corner and a few lush clusters of pots — lush but never crowding out the seating. KEEP THE FRONT DOOR and a clear walkway to it unobstructed",
	"side_terrace":    "a relaxed side-return with a rustic bench or bistro dining spot to sit and eat, shaded and softened by a climbing vine, a statement potted olive and a couple of plant clusters, with lanterns and string lights — usable and green, not cluttered",
	"garden":          "a relaxed outdoor DINING or lounge spot to sit and gather within lush, tidy dry-Mediterranean planting — a rustic dining table with chairs or a comfortable lounge as the hero, shaded by the existing trees or a parasol, framed by olive, lavender and clustered greenery. Landscape the EXISTING garden footprint only, never regrade or extend it",
	"facade":          "the private entrance terrace as an inviting Mediterranean outdoor DINING-and-lounge destination — a comfortable rustic wood-and-rattan LOUNGE with deep cream linen cushions under the covered porch AND a generous outdoor DINING table with woven or cushioned chairs on the open terrace, SHADED by a large cream fringed parasol, with a jute/flatweave rug, Moroccan lanterns and warm string lights strung overhead. Frame and FILL this generous terrace with ABUNDANT, tidy greenery so the large tiled floor never reads sterile or empty — SEVERAL statement trees (a gnarled olive, a citrus and a palm) in large earthenware urns at the corners and along the balustrade, cascading magenta bougainvillea trained over the EXISTING arch, and generous lush clusters of terracotta pots (lavender, rosemary, herbs, trailing greenery) filling the corners and edges. Leave NO bare, empty corner. Keep it lush and characterful, framing but never burying the furniture — the comfortable seating, dining and shade stay the HERO. KEEP THE FRONT DOOR and a clear walkway to it unobstructed; place the dining table and seating toward the open terrace, not in front of the door. CRITICAL: express the style ONLY through the plants, furniture, cushions, rug and lighting — NEVER repaint, whitewash, tint or recolour the building, its stucco, the arch, the balustrade or any railing (protected community colour); keep the building's EXACT existing colour and finish (only cleaned and freshened), and keep the arches, windows, balustrades and floor tiling exactly as they are",
	"facade_exterior": "the private entrance terrace as an inviting Mediterranean outdoor DINING-and-lounge destination — a comfortable rustic wood-and-rattan LOUNGE with deep cream linen cushions under the covered porch AND a generous outdoor DINING table with woven or cushioned chairs on the open terrace, SHADED by a large cream fringed parasol, with a jute/flatweave rug, Moroccan lanterns and warm string lights strung overhead. Frame and FILL this generous terrace with ABUNDANT, tidy greenery so the large tiled floor never reads sterile or empty — SEVERAL statement trees (a gnarled olive, a citrus and a palm) in large earthenware urns at the corners and along the balustrade, cascading magenta bougainvillea trained over the EXISTING arch, and generous lush clusters of terracotta pots (lavender, rosemary, herbs, trailing greenery) filling the corners and edges. Leave NO bare, empty corner. Keep it lush and characterful, framing but never burying the furniture — the comfortable seating, dining and shade stay the HERO. KEEP THE FRONT DOOR and a clear walkway to it unobstructed; place the dining table and seating toward the open terrace, not in front of the door. CRITICAL: express the style ONLY through the plants, furniture, cushions, rug and lighting — NEVER repaint, whitewash, tint or recolour the building, its stucco, the arch, the balustrade or any railing (protected community colour); keep the building's EXACT existing colour and finish (only cleaned and freshened), and keep the arches, windows, balustrades and floor tiling exactly as they are",
}

// buildOutdoorInstruction dresses an outdoor space for aspirational outdoor living while
// keeping the structure/view honest — and never inventing a pool or any new structure.
func buildOutdoorInstruction(rt, spaceType, styleName, flavor, propertyType string) string {
	guard := "KEEP THE EXACT STRUCTURE, FOOTPRINT AND VIEW — same walls, pergola, arches, balustrade, railings, columns and beams, in their EXACT number and position, and the same outlook. Do NOT move, add, remove, multiply, extend, resize or duplicate any wall, beam, column or opening, and do NOT enlarge or reshape the space to fit furniture — fit the furniture to the REAL space, and if it doesn't fit, use LESS furniture. Keep every door and its access COMPLETELY clear and usable — NEVER place a sofa, dining table, chairs, lounger or any furniture in front of, across or blocking a door or opening, ESPECIALLY the FRONT/ENTRY door; always leave an open, unobstructed walking path to every door and arrange ALL furniture to the sides so each doorway stays freely accessible and you could walk straight in. NEVER add a pool, building, extension, window or any structure that is not already there. CLEAN and freshen any dirty, stained or grimy surface so it reads new — grimy concrete beams and woodwork MUST be cleaned so they look fresh, but keep every beam, column and surface its SAME MATERIAL, colour family and shape: a concrete beam stays concrete (just cleaned), never turned into wood or any other material, and never left dirty; keep each surface's exact form and position. Do NOT change the COLOUR of the building, its walls or any fence or railing (community colour rules) — only refresh them in their SAME colour. The named style and its palette apply ONLY to the furniture, cushions, textiles, rug, planting and lighting you ADD — NEVER to the building's walls, stucco, arches, fences or railings, which keep their EXACT original colour and finish whatever the style (a 'white' or 'coastal' look must NOT whitewash or repaint a beige/stone building)."
	if isHouseType(propertyType) {
		guard += " THE BUILDING MAY BE IN THIS SHOT: this is a house, so its own facade, walls, windows, doors, balconies, roofline and stairs MUST stay EXACTLY as they are, in their exact colour and finish — you may ONLY restyle and dress the ground-level outdoor space (furniture, planting, rug, lighting) and clean/refresh existing surfaces in their SAME colour. Do NOT add, remove, resize, move, restyle or recolour any part of the building, and do NOT attach a new pergola, canopy, awning or structure to it."
	}
	dir := outdoorDirective[strings.ToLower(spaceType)]
	if dir == "" {
		dir = "an aspirational outdoor living space with tasteful outdoor furniture, planting and lighting"
	}
	body := fmt.Sprintf("REMOVE the current tired or cheap outdoor furniture and clutter, and dress this %s as %s, in a %s palette: %s. Above all make it a place people want to SIT, EAT and linger in the SHADE — comfortable, usable furniture (real seating and, where it fits, a dining table) plus clear overhead shade and warm lighting LEAD; complement them with GENEROUS but tidy greenery — a statement tree (olive, citrus or palm) in a corner and a few lush clusters of potted plants for warmth and life — without letting pots crowd out or bury the seating. Not a bare terrace and not a pot-jungle: furniture-led, but green and characterful. Fill any EMPTY corner or dead floor space with a statement plant, a small tree or a cluster of pots — never leave a bare, empty corner.", rt, dir, styleName, strings.TrimSpace(flavor))
	out := fmt.Sprintf("Photorealistic — a professional real-estate photograph of this exact %s at its best, structure and view unchanged.", rt)
	return guard + " " + body + " " + out
}

// instructionFor picks the interior or outdoor instruction for a curated space.
func instructionFor(sp propertymodel.Space, styleName, flavor, propertyType string) string {
	rt := roomType(sp.Type, sp.Name)
	if isOutdoor(sp.Type, sp.Category) {
		return buildOutdoorInstruction(rt, sp.Type, styleName, flavor, propertyType)
	}
	return buildInstruction(rt, styleName, flavor)
}

// buildInstruction assembles the Nano-Banana edit instruction: the honesty/arch
// guardrail + the restage-playbook rules + the style flavor.
func buildInstruction(rt, styleName, flavor string) string {
	guard := "KEEP THE EXACT ARCHITECTURE, FOOTPRINT AND LAYOUT — same shape, size and proportions; do not mirror, flip, reshape or resize the room. NEVER add any new window, skylight, glazed door or bright/glazed opening ANYWHERE — if a wall is solid in the original it MUST stay solid; keep ONLY the windows and openings that already exist, exactly as they are (same size, position and glazing), and never treat a wall painting, mirror, tiled panel or bright wall as a window; do not invent an outdoor view. Do NOT add, remove, multiply, enlarge, shrink or move any window, door, wall, opening or built-in. Preserve every interior ARCHWAY, cased opening, room division and the open-plan flow between areas EXACTLY — do not widen, narrow, reshape, close or move an archway or the openings between rooms. Keep any fireplace, chimney breast, column or beam in its EXACT position and form. Keep the room's NATURAL LIGHT level HONEST — do not flood a normally-lit or dim room with extra daylight or brightness it does not have. Keep all doors and access clear and usable — NEVER place a bed, sofa, table or any furniture across, in front of, or blocking a door or opening."
	rules := "Restyle EVERY wall and surface consistently — leave no original wall finish; CLEAN and freshen tired or dirty surfaces (repaint grimy woodwork, refresh worn finishes). If there is cabinetry, restyle ALL units the same way and never mix new and original. Keep decor realistic and correctly placed (never put a plant or tree inside a shower, bath or basin)."
	extra := ""
	if strings.Contains(rt, "kitchen") {
		extra = " Keep the kitchen's EXACT layout: the SAME run(s) of units on the SAME wall(s), the same worktop line, the same sink and appliance positions, and any pass-through/breakfast-bar — do NOT add an extra run of cabinets on the opposite wall, and do NOT add a window or glazed opening that is not already there (keep every wall exactly as in the original)."
	}
	body := fmt.Sprintf("REMOVE EVERY existing piece of furniture, fixture and clutter — leave NONE of the original in the scene (no old table, sofa, bed or freestanding unit left behind) — and fully restage this %s as an aspirational, editorial %s %s: %s", rt, styleName, rt, strings.TrimSpace(flavor))
	out := fmt.Sprintf("Photorealistic — a professional real-estate photograph of this exact %s, beautifully renovated in the %s style.", rt, styleName)
	return guard + " " + rules + extra + " " + body + " " + out
}

// Options configures a Run.
type Options struct {
	RendersDir   string          // filesystem dir for images (e.g. web/showcase/public/renders or …/renders/<slug>)
	ReelsDir     string          // filesystem dir for reel .mp4s (namespaced under --slug); empty = derive from RendersDir
	ReelsWebBase string          // web path prefix for reels in the manifest (e.g. /reels or /reels/<slug>); empty = /reels
	ManifestPath string          // filesystem path for the manifest (e.g. web/showcase/data/property.json)
	WebBase      string          // web path prefix for images in the manifest (e.g. /renders or /renders/<slug>)
	PromptDir    string          // dir with per-style flavor prompts (e.g. playbook/prompts/styles)
	Engine       string          // restage engine (default nano-banana)
	Generate     bool            // run the model; false = emit manifest only from existing images
	OnlyStyle    string          // limit to one style id (optional)
	OnlyRooms    map[string]bool // limit to these room ids (optional; empty = all)
	Concurrency  int             // max concurrent restage calls
	BestOf       int             // candidates to generate per space×style (>1 = best-of-N + honesty gate)
	KeepK        int             // how many honest, distinct variants to keep (default 1)
	GateVotes    int             // independent honesty-gate passes per candidate (>1 = unanimous-pass; default 1)
	PropertyType string          // taxonomy type (villa|semi-detached|townhouse|apartment|penthouse); drives type-specific rules (e.g. facade-in-frame lock). Set from the model in RunFromModel.
	Logf         func(format string, args ...any)
}

type task struct {
	room  RoomSpec
	style Style
	out   string // filesystem output path
}

// Run restages each (room × style) and writes the manifest. With Generate=false it
// skips the model and only (re)emits the manifest for images already on disk.
func Run(ctx context.Context, spec Spec, opts Options) error {
	log := opts.Logf
	if log == nil {
		log = func(string, ...any) {}
	}
	styles := spec.Styles
	if len(styles) == 0 {
		styles = DefaultStyles()
	}
	if opts.OnlyStyle != "" {
		styles = filterStyles(styles, opts.OnlyStyle)
		if len(styles) == 0 {
			return fmt.Errorf("--only-style %q matched no style", opts.OnlyStyle)
		}
	}
	if opts.WebBase == "" {
		opts.WebBase = "/renders"
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 3
	}
	if err := os.MkdirAll(opts.RendersDir, 0o755); err != nil {
		return err
	}

	rooms := spec.Rooms
	if len(opts.OnlyRooms) > 0 {
		rooms = filterRooms(rooms, opts.OnlyRooms)
	}

	// Generation phase.
	if opts.Generate {
		var editor imageedit.Editor
		var err error
		if editor, err = imageedit.NewEditor(opts.Engine); err != nil {
			return err
		}
		log("restage engine: %s", engineOrDefault(opts.Engine))

		var tasks []task
		for _, rm := range rooms {
			if rm.Hero == "" {
				return fmt.Errorf("room %q has no hero photo", rm.ID)
			}
			// Copy the hero to the page's "before" slot.
			before := filepath.Join(opts.RendersDir, rm.ID+"_before.jpg")
			if err := copyFile(rm.Hero, before); err != nil {
				return fmt.Errorf("copy hero for %s: %w", rm.ID, err)
			}
			for _, st := range styles {
				tasks = append(tasks, task{room: rm, style: st, out: filepath.Join(opts.RendersDir, st.ID+"_"+rm.ID+".jpg")})
			}
		}
		log("generating %d render(s) across %d room(s) × %d style(s)…", len(tasks), len(rooms), len(styles))
		if err := runTasks(ctx, editor, tasks, opts, log); err != nil {
			return err
		}
	}

	// Emit manifest.
	man := Manifest{
		Version:     1,
		GeneratedBy: "render showcase",
		Property:    spec.Property,
		Styles:      styles,
		Rooms:       make([]RoomOut, 0, len(rooms)),
	}
	for _, rm := range rooms {
		after := map[string]string{}
		for _, st := range styles {
			after[st.ID] = opts.WebBase + "/" + st.ID + "_" + rm.ID + ".jpg"
		}
		man.Rooms = append(man.Rooms, RoomOut{
			ID: rm.ID, Name: rm.Name, Index: rm.Index, Blurb: rm.Blurb,
			Before: opts.WebBase + "/" + rm.ID + "_before.jpg",
			After:  after, Shop: rm.Shop,
		})
	}
	if err := writeJSON(opts.ManifestPath, man); err != nil {
		return err
	}
	log("✓ wrote manifest → %s (%d rooms)", opts.ManifestPath, len(man.Rooms))
	return nil
}

// runTasks executes restage tasks with bounded concurrency, failing if any errors.
func runTasks(ctx context.Context, editor imageedit.Editor, tasks []task, opts Options, log func(string, ...any)) error {
	flavors := loadFlavors(opts.PromptDir)
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, t := range tasks {
		wg.Add(1)
		go func(t task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			mu.Lock()
			stop := firstErr != nil
			mu.Unlock()
			if stop {
				return
			}

			rt := roomType(t.room.ID, t.room.Name)
			instr := buildInstruction(rt, t.style.Name, flavors[t.style.ID])
			log("  ↻ %s · %s", t.room.ID, t.style.ID)
			if _, err := imageedit.EditFile(ctx, editor, t.room.Hero, t.out, instr); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("restage %s/%s: %w", t.room.ID, t.style.ID, err)
				}
				mu.Unlock()
				return
			}
			log("  ✓ %s · %s → %s", t.room.ID, t.style.ID, filepath.Base(t.out))
		}(t)
	}
	wg.Wait()
	return firstErr
}

// loadFlavors reads per-style flavor prompts from dir, falling back to built-ins.
func loadFlavors(dir string) map[string]string {
	out := map[string]string{}
	for id, def := range builtinFlavor {
		out[id] = def
	}
	if dir == "" {
		return out
	}
	for id := range out {
		if b, err := os.ReadFile(filepath.Join(dir, id+".txt")); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				out[id] = s
			}
		}
	}
	return out
}

func filterStyles(styles []Style, only string) []Style {
	var out []Style
	for _, s := range styles {
		if s.ID == only {
			out = append(out, s)
		}
	}
	return out
}

func filterRooms(rooms []RoomSpec, only map[string]bool) []RoomSpec {
	var out []RoomSpec
	for _, r := range rooms {
		if only[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func engineOrDefault(e string) string {
	if strings.TrimSpace(e) == "" {
		return "nano-banana"
	}
	return e
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeJSON(path string, v any) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// SpaceOut is one space in the full (grouped) page manifest.
type SpaceOut struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Category     string              `json:"category"` // interior | outdoor_private | shared
	Showcase     string              `json:"showcase"` // hero | strong | supporting
	Potential    string              `json:"potential"`
	Before       string              `json:"before"`
	After        map[string]string   `json:"after,omitempty"`        // style -> PRIMARY variant web path (back-compat)
	Variants     map[string][]string `json:"variants,omitempty"`     // style -> all variant web paths
	Descriptions map[string]string   `json:"descriptions,omitempty"` // style -> <=2 sentence caption
	Generated    bool                `json:"generated"`              // true once the styled afters actually exist
	Context      bool                `json:"context"`                // shared amenity, shown as-is
	Video        string              `json:"video,omitempty"`        // single style-agnostic reel (fallback)
	Videos       map[string]string   `json:"videos,omitempty"`       // style -> per-style reel web path (preferred)
}

// FullManifest is the grouped page manifest produced from a curated property model.
type FullManifest struct {
	Version  int                    `json:"version"`
	Property propertymodel.Property `json:"property"`
	Styles   []Style                `json:"styles"`
	Spaces   []SpaceOut             `json:"spaces"`
}

// styleVariations pushes each best-of-N candidate toward a DIFFERENT interpretation within
// the same look, so the kept variants are genuinely distinct — not near-duplicate draws.
// Each lean varies BOTH palette AND furniture layout/use: the strongest variants (per the
// bunk-bedroom result) reimagine the arrangement rather than mirroring the original
// composition. The structural guard + honesty gate keep fixed fixtures in place, so
// "reimagine the layout" moves only furnishings and styling, never walls, units or openings.
var styleVariations = map[string][]string{
	"mediterranean": {
		"Lean into warm terracotta, aged brass and oak, in a relaxed, generously-spaced layout.",
		"Lean into lime-washed cream, zellige tile and rattan, in a bright, pared-back minimal arrangement.",
		"Lean into olive-green accents, jute and earthenware, in a cosy, layered characterful setup.",
		"Lean into a richer boutique-riad mood, boldly reimagining the furniture layout and use of the space (do not copy the original arrangement).",
	},
	"scandinavian": {
		"Lean into pale oak and soft white, very minimal and open.",
		"Lean into warm-white with light birch and muted greys, in a cosy, layered layout.",
		"Lean into crisp white with matte-black accents, in a bold, graphic arrangement.",
		"Lean into hygge textures — wool, linen and plants — boldly reimagining the furniture layout and use of the space (do not copy the original arrangement).",
	},
	"coastal": {
		"Lean into crisp white with navy-blue accents, in an airy, open layout.",
		"Lean into white and soft-blue with rattan and linen, in a relaxed, casual arrangement.",
		"Lean into sand-and-white with brushed brass, in a refined, minimal setup.",
		"Lean into breezy white with pale driftwood timber, boldly reimagining the furniture layout and use of the space (do not copy the original arrangement).",
	},
}

func variationsFor(styleID string) []string {
	if v, ok := styleVariations[styleID]; ok {
		return v
	}
	return []string{""}
}

// generateVariants runs best-of-N for ONE space×style: generate opts.BestOf candidates with
// diverse prompts, keep up to opts.KeepK that pass the honesty gate, written as
// <style>_<id>_vN.jpg. Falls back to one candidate if none pass (flagged for review).
func generateVariants(ctx context.Context, editor imageedit.Editor, gate verify.Gate, hero string,
	sp propertymodel.Space, st Style, flavor string, sem chan struct{}, opts Options, log func(string, ...any)) error {
	n := opts.BestOf
	vars := variationsFor(st.ID)
	tmp, err := os.MkdirTemp("", "variants-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// 1. generate n candidates concurrently (bounded by the shared sem).
	paths := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			instr := instructionFor(sp, st.Name, flavor, opts.PropertyType)
			if v := vars[i%len(vars)]; v != "" {
				instr += " " + v
			}
			paths[i], errs[i] = imageedit.EditFile(ctx, editor, hero, filepath.Join(tmp, fmt.Sprintf("c%d.jpg", i)), instr)
		}(i)
	}
	wg.Wait()

	// 2. honesty-gate each candidate (only when doing best-of-N); keep the honest ones in order.
	honest := make([]bool, n)
	if n > 1 {
		rt := roomType(sp.Type, sp.Name)
		var wg2 sync.WaitGroup
		for i := 0; i < n; i++ {
			if errs[i] != nil {
				continue
			}
			wg2.Add(1)
			go func(i int) {
				defer wg2.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if v, e := gate.VerifyPair(ctx, hero, paths[i], rt); e == nil && v.OK() {
					honest[i] = true
				}
			}(i)
		}
		wg2.Wait()
	} else {
		honest[0] = errs[0] == nil // single-shot: no gate
	}

	var keep []string
	for i := 0; i < n; i++ {
		if honest[i] {
			keep = append(keep, paths[i])
		}
	}
	if len(keep) == 0 { // none passed → keep the first successful candidate, flagged
		for i := 0; i < n; i++ {
			if errs[i] == nil {
				keep = []string{paths[i]}
				log("  ⚠ %s · %s: 0/%d passed the gate — kept 1 (flag for review)", sp.ID, st.ID, n)
				break
			}
		}
	}
	if len(keep) == 0 {
		return fmt.Errorf("all %d candidates failed to generate", n)
	}

	// 3. clear stale variants for this space×style, then write up to KeepK.
	stale, _ := filepath.Glob(filepath.Join(opts.RendersDir, st.ID+"_"+sp.ID+"_v*.jpg"))
	for _, s := range stale {
		os.Remove(s)
	}
	os.Remove(filepath.Join(opts.RendersDir, st.ID+"_"+sp.ID+".jpg")) // legacy single-shot name
	wrote := 0
	for i, src := range keep {
		if i >= opts.KeepK {
			break
		}
		dst := filepath.Join(opts.RendersDir, fmt.Sprintf("%s_%s_v%d.jpg", st.ID, sp.ID, i+1))
		if err := copyFile(src, dst); err != nil {
			return err
		}
		wrote++
	}
	log("  ✓ %s · %s: kept %d/%d honest variant(s)", sp.ID, st.ID, wrote, n)
	return nil
}

// variantFiles returns the existing after-image base filenames for a space×style, preferring
// _v1.._vN variants and falling back to the legacy single file. Sorted.
func variantFiles(dir, styleID, id string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, styleID+"_"+id+"_v*.jpg"))
	sort.Strings(matches)
	var out []string
	for _, m := range matches {
		out = append(out, filepath.Base(m))
	}
	if len(out) == 0 {
		legacy := styleID + "_" + id + ".jpg"
		if fileExists(filepath.Join(dir, legacy)) {
			out = append(out, legacy)
		}
	}
	return out
}

// RunFromModel restages the SELECTED spaces of a curated property model and emits the
// grouped page manifest. photoDir is the source folder the hero filenames resolve
// against; enhance-context (shared) spaces are shown as-is, skip spaces are dropped.
// This is the paid step — run it only after the plan has been cleared.
func RunFromModel(ctx context.Context, m propertymodel.Model, photoDir string, opts Options) error {
	log := opts.Logf
	if log == nil {
		log = func(string, ...any) {}
	}
	opts.PropertyType = m.Property.Type // drives type-specific rules (facade-in-frame lock for house types)
	styles := DefaultStyles()
	if opts.OnlyStyle != "" {
		styles = filterStyles(styles, opts.OnlyStyle)
		if len(styles) == 0 {
			return fmt.Errorf("--only-style %q matched no style", opts.OnlyStyle)
		}
	}
	if opts.WebBase == "" {
		opts.WebBase = "/renders"
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 3
	}
	if err := os.MkdirAll(opts.RendersDir, 0o755); err != nil {
		return err
	}

	// allSpaces = every selected space: their befores are staged and they are ALL emitted to
	// the manifest, so a partial --only-rooms render never drops spaces from the page.
	// genSpaces = the subset to actually (re)generate this run.
	allSpaces := m.SelectedSpaces()
	genSpaces := allSpaces
	if len(opts.OnlyRooms) > 0 {
		var f []propertymodel.Space
		for _, s := range allSpaces {
			if opts.OnlyRooms[s.ID] {
				f = append(f, s)
			}
		}
		genSpaces = f
	}

	flavors := loadFlavors(opts.PromptDir)
	if opts.BestOf < 1 {
		opts.BestOf = 1
	}
	if opts.KeepK < 1 {
		opts.KeepK = 1
	}
	beforeExt := map[string]string{}
	// The before is a local copy — no spend — so always stage it for every selected space.
	for _, sp := range allSpaces {
		hero := m.HeroFile(sp)
		if hero == "" {
			return fmt.Errorf("space %q has no hero photo in the model", sp.ID)
		}
		ext := strings.ToLower(filepath.Ext(hero))
		if ext == "" {
			ext = ".jpg"
		}
		beforeExt[sp.ID] = ext
		if err := copyFile(filepath.Join(photoDir, hero), filepath.Join(opts.RendersDir, sp.ID+"_before"+ext)); err != nil {
			return fmt.Errorf("copy hero for %s: %w", sp.ID, err)
		}
	}

	if opts.Generate {
		editor, err := imageedit.NewEditor(opts.Engine)
		if err != nil {
			return err
		}
		votes := opts.GateVotes
		if votes < 1 {
			votes = 1
		}
		gate := verify.NewGate() // strict shell-vs-contents — catches added windows/doors/openings
		if votes > 1 {
			gate = gate.Voted(votes) // unanimous-pass across N passes → higher defect recall on stubborn spaces
		}
		log("restage engine: %s · best-of-%d, keep %d, gate-votes %d", engineOrDefault(opts.Engine), opts.BestOf, opts.KeepK, votes)
		sem := make(chan struct{}, opts.Concurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		for _, sp := range genSpaces {
			if sp.RestageTier != propertymodel.TierRestyle {
				continue
			}
			for _, st := range styles {
				wg.Add(1)
				go func(sp propertymodel.Space, st Style) {
					defer wg.Done()
					mu.Lock()
					stop := firstErr != nil
					mu.Unlock()
					if stop {
						return
					}
					hero := filepath.Join(photoDir, m.HeroFile(sp))
					if err := generateVariants(ctx, editor, gate, hero, sp, st, flavors[st.ID], sem, opts, log); err != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = fmt.Errorf("restage %s/%s: %w", sp.ID, st.ID, err)
						}
						mu.Unlock()
					}
				}(sp, st)
			}
		}
		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}

	man := FullManifest{Version: 2, Property: m.Property, Styles: styles}
	// ver appends the file's mtime as a cache-busting ?v= — a regenerated file keeps its
	// name, so without this the browser/Next image cache would keep serving the old one.
	ver := func(dir, name, web string) string {
		if fi, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return fmt.Sprintf("%s?v=%d", web, fi.ModTime().Unix())
		}
		return web
	}
	reelsDir := opts.ReelsDir
	if reelsDir == "" {
		reelsDir = filepath.Join(filepath.Dir(opts.RendersDir), "reels")
	}
	reelsWebBase := opts.ReelsWebBase
	if reelsWebBase == "" {
		reelsWebBase = "/reels"
	}
	for _, sp := range allSpaces {
		beforeName := sp.ID + "_before" + beforeExt[sp.ID]
		out := SpaceOut{
			ID: sp.ID, Name: sp.Name, Category: sp.Category, Showcase: sp.ShowcaseValue,
			Potential: sp.Potential, Descriptions: sp.Descriptions,
			Before: ver(opts.RendersDir, beforeName, opts.WebBase+"/"+beforeName),
		}
		if sp.RestageTier == propertymodel.TierRestyle {
			out.After = map[string]string{}
			out.Variants = map[string][]string{}
			for _, st := range styles {
				// List every variant that actually exists (a failed/missing style is
				// omitted so the page never 404s — it shows "potential pending").
				files := variantFiles(opts.RendersDir, st.ID, sp.ID)
				if len(files) == 0 {
					continue
				}
				var webs []string
				for _, name := range files {
					webs = append(webs, ver(opts.RendersDir, name, opts.WebBase+"/"+name))
				}
				out.Variants[st.ID] = webs
				out.After[st.ID] = webs[0] // primary = first variant (back-compat)
			}
			out.Generated = len(out.After) > 0
		} else {
			out.Context = true
		}
		// wire reels in when their clips exist. Prefer PER-STYLE reels
		// (public/reels/<id>_<style>.mp4); fall back to a single style-agnostic
		// clip (public/reels/<id>.mp4) for back-compat.
		if sp.Animate {
			for _, st := range styles {
				reel := sp.ID + "_" + st.ID + ".mp4"
				if fileExists(filepath.Join(reelsDir, reel)) {
					if out.Videos == nil {
						out.Videos = map[string]string{}
					}
					out.Videos[st.ID] = ver(reelsDir, reel, reelsWebBase+"/"+reel)
				}
			}
			if reel := sp.ID + ".mp4"; fileExists(filepath.Join(reelsDir, reel)) {
				out.Video = ver(reelsDir, reel, reelsWebBase+"/"+reel)
			}
		}
		man.Spaces = append(man.Spaces, out)
	}
	if err := writeJSON(opts.ManifestPath, man); err != nil {
		return err
	}
	log("✓ wrote full manifest → %s (%d spaces)", opts.ManifestPath, len(man.Spaces))
	return nil
}

// LoadSpec reads and parses an input spec JSON file.
func LoadSpec(path string) (Spec, error) {
	var s Spec
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse spec %s: %w", path, err)
	}
	if len(s.Rooms) == 0 {
		return s, fmt.Errorf("spec %s has no rooms", path)
	}
	return s, nil
}
