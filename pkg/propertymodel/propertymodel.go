// Package propertymodel is the canonical, cross-stage property model: the curated
// understanding of a property — its spaces, the hero photo for each, how each is
// categorised, and what should be restaged/animated. It is emitted by the
// SCENE-CLASSIFY stage (`director classify`) and consumed by the multi-style RESTAGE
// stage (`render showcase`), so the two agree on one shape.
package propertymodel

import (
	"encoding/json"
	"os"
	"strconv"
)

// Category and RestageTier values.
const (
	CategoryInterior = "interior"
	CategoryOutdoor  = "outdoor_private"
	CategoryShared   = "shared"

	TierRestyle = "restyle"         // re-stage this space (private spaces)
	TierContext = "enhance-context" // shared amenity — show as-is, never restage/imply private
	TierSkip    = "skip"
)

// Property-type taxonomy — the fixed set the classifier picks the single best fit from.
// This is not cosmetic: it drives type-specific behaviour, above all the OUTDOOR scope
// (apartments/penthouses have a COMMUNAL ground pool/garden + private balcony/solarium;
// semi-detached/villa have a PRIVATE garden and their facade in-frame → facade-lock).
// Extend deliberately (e.g. finca, bungalow, plot) as the market needs.
const (
	TypeApartment    = "apartment"
	TypePenthouse    = "penthouse"
	TypeTownhouse    = "townhouse"
	TypeSemiDetached = "semi-detached"
	TypeVilla        = "villa"
)

// PropertyTypes is the allowed taxonomy in display order.
var PropertyTypes = []string{TypeApartment, TypePenthouse, TypeTownhouse, TypeSemiDetached, TypeVilla}

// Space is one curated space in the property model.
type Space struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Category      string `json:"category"` // interior | outdoor_private | shared
	PhotoIndexes  []int  `json:"photo_indexes"`
	HeroIndex     int    `json:"hero_index"`
	Current       string `json:"current"`
	Potential     string `json:"potential"`
	ShowcaseValue string `json:"showcase_value"` // hero | strong | supporting
	RestageTier   string `json:"restage_tier"`   // restyle | enhance-context | skip
	Selected      bool   `json:"selected"`
	Animate       bool   `json:"animate"`
	Reason        string `json:"reason"`
	// Descriptions maps styleId -> a short (<=2 sentence) caption of this room in that
	// style, filled by the `describe` stage. Empty until then.
	Descriptions map[string]string `json:"descriptions,omitempty"`
}

// Excluded is a photo the editor chose not to use.
type Excluded struct {
	PhotoIndex int    `json:"photo_index"`
	Reason     string `json:"reason"`
}

// Property is the header understanding.
type Property struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Type     string `json:"type"`
	Sleeps   int    `json:"sleeps_estimate"`
}

// Model is the curated property model.
type Model struct {
	Property   Property          `json:"property"`
	PhotoIndex map[string]string `json:"photo_index"` // "1" -> filename
	Spaces     []Space           `json:"spaces"`
	Excluded   []Excluded        `json:"excluded"`
}

// HeroFile returns the hero photo filename for a space (relative to the source folder).
func (m Model) HeroFile(s Space) string { return m.PhotoIndex[strconv.Itoa(s.HeroIndex)] }

// SelectedSpaces returns the spaces marked for the page, in model order.
func (m Model) SelectedSpaces() []Space {
	var out []Space
	for _, s := range m.Spaces {
		if s.Selected {
			out = append(out, s)
		}
	}
	return out
}

// Load reads a model JSON file.
func Load(path string) (Model, error) {
	var m Model
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	return m, err
}

// Save writes a model JSON file (pretty-printed).
func Save(path string, m Model) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
