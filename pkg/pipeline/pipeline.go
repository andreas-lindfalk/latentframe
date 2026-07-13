// Package pipeline defines the six-stage Latent Frame ingestion pipeline that turns
// a phone walkthrough video (plus spoken context) into an aspirational "after"
// reveal video.
//
// See docs/02-refined-blueprint.md. The one inviolable rule enforced here is:
// RE-STAGE, NEVER RESTRUCTURE — the VERIFY stage exists to reject any output whose
// architecture (walls, windows, layout) drifted from the source.
//
// The stage interfaces are contracts; concrete implementations (Claude calls, image
// generation, image-to-video) are added incrementally. The moat lives in stages
// UNDERSTAND and VERIFY — the automated art-director and honesty gate.
package pipeline

import (
	"context"
	"fmt"

	"github.com/andreas-lindfalk/latentframe/pkg/media"
)

// Stage identifies one of the six pipeline stages.
type Stage int

const (
	StageIngest     Stage = iota + 1 // 1. scene-split; pick sharpest hero frame per room
	StageUnderstand                  // 2. Claude: per-room brief + one global design vision
	StageRestage                     // 3. structure-locked image gen of the "after" still
	StageVerify                      // 4. Claude honesty gate: architecture unchanged & believable?
	StageAnimate                     // 5. image-to-video: short cinematic camera move
	StageAssemble                    // 6. stitch clips into the reveal reel + property page
)

// Property is the unit of work: one listing, its rooms, and one coherent design
// vision applied across all of them.
type Property struct {
	ID     string
	Vision GlobalVision
	Rooms  []Room
}

// Room accumulates the per-stage artifacts for a single space.
type Room struct {
	ID         string
	Label      string    // e.g. "kitchen", "master bedroom"
	Hero       HeroFrame // stage 1
	Brief      RestageBrief
	AfterStill string  // stage 3 output (path or URI)
	Verdict    Verdict // stage 4
	Clip       string  // stage 5 output (path or URI)
}

// HeroFrame is the single best frame chosen to represent a room — the control
// surface for the whole "after" (we perfect one still, then animate it).
type HeroFrame struct {
	Path        string
	TimestampMs int64
	Sharpness   float64 // higher is sharper; used to pick the hero frame
}

// GlobalVision is the single design identity applied to every room so the whole
// property reads as one renovation, not eight unrelated pretty pictures.
type GlobalVision struct {
	Style string // e.g. "warm Nordic minimalism, oak + off-white, soft daylight"
	Notes string
}

// RestageBrief is stage 2's output for a room: what to change and the exact
// structure-locked prompt to generate the "after".
type RestageBrief struct {
	CurrentState  string
	DesiredChange string
	Prompt        string
}

// Verdict is stage 4's ruling. Both flags must be true for the room to ship.
type Verdict struct {
	ArchitecturePreserved bool // walls/windows/layout unchanged vs. the source frame
	Believable            bool
	Reason                string
}

func (v Verdict) OK() bool { return v.ArchitecturePreserved && v.Believable }

// Stage interfaces. Each is injected into the Runner.
type (
	Ingestor interface {
		Ingest(ctx context.Context, videoPath string) (*Property, error)
	}
	Understander interface {
		Understand(ctx context.Context, p *Property, transcript []media.Segment) error
	}
	Restager interface {
		Restage(ctx context.Context, room *Room, vision GlobalVision) error
	}
	Verifier interface {
		Verify(ctx context.Context, room *Room) (Verdict, error)
	}
	Animator interface {
		Animate(ctx context.Context, room *Room) error
	}
	Assembler interface {
		Assemble(ctx context.Context, p *Property) (reelPath string, err error)
	}
)

// Runner wires the six stages together.
type Runner struct {
	Ingest     Ingestor
	Understand Understander
	Restage    Restager
	Verify     Verifier
	Animate    Animator
	Assemble   Assembler

	// MaxRestageRetries bounds the RESTAGE→VERIFY honesty-gate loop per room.
	MaxRestageRetries int
}

// Run executes the full pipeline for one video and returns the assembled reel path.
//
// The RESTAGE→VERIFY loop is the heart of the product: regenerate the "after" until
// VERIFY confirms the architecture was preserved and the result is believable, or
// give up (fail closed — we never ship a room that drifted).
func (r *Runner) Run(ctx context.Context, videoPath string, transcript []media.Segment) (string, error) {
	prop, err := r.Ingest.Ingest(ctx, videoPath)
	if err != nil {
		return "", fmt.Errorf("stage ingest: %w", err)
	}
	if err := r.Understand.Understand(ctx, prop, transcript); err != nil {
		return "", fmt.Errorf("stage understand: %w", err)
	}

	for i := range prop.Rooms {
		room := &prop.Rooms[i]

		var verdict Verdict
		for attempt := 0; attempt <= r.MaxRestageRetries; attempt++ {
			if err := r.Restage.Restage(ctx, room, prop.Vision); err != nil {
				return "", fmt.Errorf("stage restage (room %s): %w", room.ID, err)
			}
			if verdict, err = r.Verify.Verify(ctx, room); err != nil {
				return "", fmt.Errorf("stage verify (room %s): %w", room.ID, err)
			}
			if verdict.OK() {
				break
			}
		}
		room.Verdict = verdict
		if !verdict.OK() {
			return "", fmt.Errorf("room %s failed the honesty gate after %d attempt(s): %s",
				room.ID, r.MaxRestageRetries+1, verdict.Reason)
		}

		if err := r.Animate.Animate(ctx, room); err != nil {
			return "", fmt.Errorf("stage animate (room %s): %w", room.ID, err)
		}
	}

	reel, err := r.Assemble.Assemble(ctx, prop)
	if err != nil {
		return "", fmt.Errorf("stage assemble: %w", err)
	}
	return reel, nil
}
