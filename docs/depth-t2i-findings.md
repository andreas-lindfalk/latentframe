# depth-t2i / canny engine — tuned settings & findings

Status: **fallback engine.** Nano-Banana (Gemini 3) is the default and won the bake-off
(see `model-bakeoff-plan.md` + the "Switch default RESTAGE engine to Nano-Banana" commit).
This file captures the depth-t2i tuning so it isn't lost if we return to it (e.g. for a
self-hosted / cost-controlled route — FLUX Control-LoRA is self-hostable, Nano-Banana is not).

## The engine
`pkg/imageedit/fluxdepth.go` — text-to-image locked to a control map (fresh content, fully
guts the room). fal flow: source → control preprocessor → FLUX Control-LoRA (t2i).

## Settings (env; code defaults in parens)
- `LATENTFRAME_CONTROL` — `canny` (default) or `depth`.
  - **canny** (edges) locks the LINES of windows/doors/openings — the openings lever.
  - **depth** is blind to flat wall openings → drops windows/doors. Don't use for honesty.
- `LATENTFRAME_DEPTH_SCALE` — `control_lora_strength` (**0.8**). THE central dial (below).
- `LATENTFRAME_FLUX_STEPS` (30), `LATENTFRAME_FLUX_GUIDANCE` (3.5).

## The strength dial — the key finding (canny)
A single dial can't give both wow AND unchanged windows:
- **canny 0.7** → WOW back (riad drama) + fixture prompts land (toilet-keep works), BUT
  openings DRIFT (window → arch, window → fabricated sea-view door) → often fails honesty.
- **canny 0.8** → openings honest, windows hold, BUT conservative ("safe refresh", muted wow).
- The prompt "keep windows exactly" does NOT hold at 0.7 — prose can't enforce structure.

Per-draw honesty (inspire gate), cold run over both houses: depth = 7/9 rooms, ~48%/draw;
**canny 0.8 = 9/9 rooms, ~74%/draw.** (Nano-Banana, for comparison: 9/9, ~93%/draw.)

## The recipe that worked (canny)
canny ~0.8 + a **view-guardrail** in every prompt ("windows show only soft neutral
daylight; do NOT invent a sea/countryside/garden/pool view") + **explicit fixture-upgrade
instructions** ("keep the toilet, remove the bidet, replace the tub with a walk-in shower").
canny-moderate keeps the shell while the prompt drives fixture changes — no masking needed
for the 80/20. Prompts: `playbook/prompts/depth-t2i/*.txt`.

## Dead ends (don't re-try)
- **MLSD shell-only** (`--control mlsd` in `experiments/aesthetic_engine.py`) freed the
  fixtures (bidet gone) but came out sterile/boxy — the MLSD→canny-control-lora mismatch.
- **Prompt-tuning for structure fidelity** — never stabilizes. Structure is deterministic;
  it needs conditioning/masking, not prose (stochastic-for-taste, deterministic-for-facts).

## Window fidelity (unbuilt)
To get 0.7 wow AND exact windows, the plan was **window-pixel masking** (SAM/segmentation →
composite the real window pixels back over the render). Never built — Nano-Banana preserves
windows natively, so it became unnecessary. Keep in mind if returning to depth-t2i.

## Iteration tooling
`experiments/aesthetic_engine.py --control canny|depth|mlsd [--t2i] [--depth-scale X]` —
the fast POC harness (fal upload + preprocess + control-lora + optional IP-adapter ref).
