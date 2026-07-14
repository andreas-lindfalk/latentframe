# Latent Frame — Restage Playbook

The accumulated rules and tuned prompts for the RESTAGE stage (turn a dated listing
photo into an aspirational, *honest* "after"). Distilled from a mid-2026 POC that tuned
the prompts on two real Costa Blanca properties and validated them **cold** on a third.

The single inviolable rule: **RE-STAGE, NEVER RESTRUCTURE.** Show the potential; never
misrepresent the property.

## Prompt templates — `prompts/`

`living.txt` · `kitchen.txt` · `bathroom.txt` · `bedroom.txt` — the tuned per-room
templates. They are **generic**: run them with no per-house tuning (validated first-shot
on a never-seen property). Each encodes both the aesthetic and the honesty guards below.

## Target aesthetic — per room type, NOT one global look

The Costa Blanca target is **warm Spanish-Mediterranean / riad**, not cool modern
coastal — but it is calibrated per room:

- **Living + bedroom → warm cozy riad:** limewash plaster walls, terracotta floors,
  olive trees & greenery, slipcovered linen/cream seating, jute rugs, aged brass,
  rattan/cane, earthenware, warm golden light.
- **Bathroom → clean, cool, luxe SPA** (NOT warm-terracotta-riad): warm stone, a
  floating oak/wood vanity, a frameless glass walk-in shower, aged brass, a backlit round
  mirror. Refined, not rustic.
- **Kitchen → warm minimalist, REFINED-MODERN:** light-oak **and cream** cabinetry
  (do NOT default to sage-green), a natural stone/concrete worktop, a subtle backsplash,
  curated (not crowded) open shelving, terracotta/warm-stone floor. NOT rustic farmhouse.

## Honesty rules — SHELL vs CONTENTS (enforced by the VERIFY gate)

- **SHELL is locked:** walls, windows, exterior openings, room shape/proportions,
  ceiling. Never add/enlarge/move a window; never invent glazing, beams or arches that
  aren't there; never erase a functional zone; keep every door/opening; don't relocate
  fixtures to a different wall. Kitchens stay FUNCTIONAL (keep hob + oven).
- **CONTENTS are free — replaced IN PLACE:** furniture, finishes, and fixtures
  (tub → walk-in shower in the same wet zone; new kitchen along the same wall).
- Verified by an Opus vision gate (shell-vs-contents rubric). Gate is stochastic on
  borderline cases → vote best-of-N in production.

## Prompting techniques (learned the hard way)

- Recolour ALL walls consistently — don't leave one wall the old colour.
- Never block a door/opening/walkway with furniture.
- **Break a stubborn object-anchor by naming a concretely different replacement**
  (Gemini keeps dated furniture otherwise): e.g. "remove the dated cabinets, install NEW
  handleless flat-slab"; "a round travertine coffee table with slim black legs".
- Removals → reliable via surgical edit. Recolour/reposition → re-generate (or mask).
- A full regeneration to fix one thing can REGRESS other good elements → re-pin every
  keeper explicitly in the prompt.
- Owner context beats a single photo (a barely-visible window makes the gate flip-flop).

## Output-polish grade — generic, applied to every "after"

Deterministic ffmpeg grade: warm balance + **shadow lift** (NOT a global brightness lift,
which washes the sky) + a touch of blue in the highlights + mild contrast/saturation +
light vignette. **Preserve the vivid blue sky** — it's a selling point. (And a vivid sky
must come from GENERATION — a render that came out pale can't be recovered in post.)

```
curves=all='0/0.02 0.25/0.30 0.72/0.75 1/1',eq=saturation=1.10:contrast=1.03,\
colorbalance=rs=0.02:rm=0.02:gm=0.004:bs=-0.008:bm=-0.010:bh=0.02,\
unsharp=5:5:0.30,vignette=PI/8
```

## Reference-conditioning — the aesthetic engine (POC: `../experiments/refcond.py`)

- Showing the model curated **style-reference images** captures the "feeling" far better
  than prose (show ≫ describe). Build a hand-curated reference library per room type.
- BUT it bleeds the references' ARCHITECTURE into the output, and prompt-locking can't
  stop it. The **honest** version needs structural conditioning:
  **SD/FLUX + ControlNet** (depth/edge/segmentation from the source = hard architecture
  lock) **+ IP-Adapter** (style from the reference library). Also gives region masking;
  self-hostable. This is the recommended next build.
- The reference library is also the substrate for furniture product-placement / affiliate
  (Sklum, Leroy Merlin, IKEA) — the "get the look" revenue stream.

## The flywheel — improve without regressing

- Every human correction becomes a **generic rule here**, not a per-photo patch.
- **Golden regression set:** keep every APPROVED output as `{input, room type, context,
  prompt version, approved after}`; on any prompt change, re-run + re-score the whole set
  (VERIFY + a quality judge) and ship only if nothing regresses. Consistency IS the product.

## Model direction

- **Gemini 2.5 Flash Image** (current): good honest edits, but it ANCHORS (keeps dated
  furniture) and has a brightness/window bias — the ceiling on prompt-only reliability.
- **Evaluate:** FLUX.1 Kontext, gpt-image-1, Seedream (edit models); **SD/FLUX +
  ControlNet + IP-Adapter** (the honest reference-conditioning path). Video: Runway Aleph
  vs the current Kling O3 Pro (v2v).

## POC demo galleries (published artifacts)

- Zeniamar 5 (owner's own house): https://claude.ai/code/artifact/1cd7f0f0-e464-4b21-ba8f-cb89b32ca2d1
- Calle las Rosas (cold transfer test, never-seen property): https://claude.ai/code/artifact/07eec955-1b98-467b-9886-3a103880a2c9
