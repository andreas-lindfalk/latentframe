# Nano-Banana engine — operating notes & taming rules

The current default RESTAGE engine (bake-off winner, 2026-07-17). Simpler than the
depth-t2i fallback: image + edit-instruction → restaged image, structure preserved
natively. This doc is the flywheel — per-image findings live *here in the repo*, not just
in agent-memory.

## The engine
- `pkg/imageedit/nanobanana.go` — in-context edit via fal (`fal-ai/nano-banana-2/edit`,
  Gemini 3). Flow: upload → edit → download. No preprocessor, no strength dial.
- Model: `LATENTFRAME_NANOBANANA_MODEL` (default `nano-banana-2`, ~$0.08/img; `nano-banana-pro`
  ~$0.15 is equal quality — NB-2 is the pick).
- Prompts: **edit instructions** in `playbook/prompts/nano-banana/*.txt` — "keep the exact
  architecture + layout … remove all dated furniture/fixtures … replace with <target look>."
- Reliability: `director beststage` (best-of-N → inspire gate → select). Cold run over both
  houses: **9/9 rooms, ~93%/draw honest.**

## What it solves natively (was hard on depth-t2i)
- **Windows / geometry** preserved exactly (real grilles, real views) — no masking needed.
- **Fixture identity** correct — tells a toilet from a bidet (keeps the toilet, removes the
  bidet, tub → walk-in shower).
- **Wow + honesty at once** — no wow↔lock tradeoff.

## Taming rules (from Andreas's gallery reviews)
1. **Anti-mirror (2026-07-17).** Rare (~1/9): the model occasionally **mirrors / reorganises
   the room** — the Rosas bedroom flipped left-right (window ↔ painting-wall swapped sides).
   The inspire gate does NOT catch this (a mirrored room still has honest size/light), so it
   passed. **Fix (in every prompt): "keep the SAME LEFT-RIGHT LAYOUT — do not mirror, flip or
   reorganise the room; keep the window, door and furniture on their original sides, and never
   treat a wall painting as a window."** Verified: window stays on the source side across all
   draws. (Future option: add an orientation check to the gate.)
2. **Wow via prompting is SAFE.** Because structure is the *model's* job now, prompt-tuning
   only moves taste — push prompts bolder/editorial for more wow with zero honesty risk
   (stochastic-for-taste, deterministic-for-facts, with the model owning the facts).

## Open / next
- best-of-2 likely enough (93%/draw) — drop from 3 to save cost.
- Point the golden harness at nano-banana; Fable-as-judge A/B.
- Product: Whisper/UNDERSTAND → owner-specific edit instructions ("lose the bidet, open this
  wall") — nano-banana follows instructions well, so this is where it gets personal.
