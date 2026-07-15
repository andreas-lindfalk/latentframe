# Taming the depth-t2i beast — exploration plan

The depth-t2i engine has a high ceiling (genuine wow) and high variance ("an untamed
powerful beast"). Taming = getting the wow *reliably* and *honestly*. This plan folds in
what we've learned so far and where to explore next.

## The core reframe: three layers, controlled independently

We've been mimicking the existing interior wholesale. That conflates three things that
should each be controlled on their own dial:

| Layer | What | Desired control | Why |
|---|---|---|---|
| **1. Shell** | walls, windows, doors, openings, room shape/size | **LOCK** | honesty — a lead must recognise the space |
| **2. Fixtures & layout** | bidet, tub, toilet, sink, kitchen run, their positions | **INTENT-DRIVEN** | keep by default (real space), but *upgrade when wanted* — remove the bidet, tub → walk-in shower |
| **3. Finishes, furniture, décor** | tiles, paint, flooring, sofas, styling, lighting | **FREE** | this is the restage; reinvent every time |

The beast is untamed because our current controls act on all three at once:
- **depth** loses the shell (drops/moves windows) but frees fixtures.
- **canny** locks the shell *and* the fixtures — so the old bidet + tub survive (Andreas
  2026-07-15: fine for inspiration, but sometimes we'd want them gone).

**The unlock: lock layer 1, intent-control layer 2, free layer 3.**

## What's proven so far
- **Canny is the openings lever.** zeniamar_living FAILED depth-t2i (0/3, removed windows);
  canny @ control_lora_strength 0.85–0.92 + a view-guardrail prompt → PASSES inspire.
- **Strength ↔ wow dial:** canny 0.7 = dramatic but shell drifts (adds windows / fabricates
  view → fails); 0.85–0.92 = shell honest but transform goes modest (refresh, not gut).
- **View fabrication** (invented sea/countryside view) is a *separate* defect → prompt
  guardrail: "windows show only soft neutral daylight, no invented view."
- The inspire honesty gate is a real, discriminating gate (catches faked view/light).

## Exploration tracks

**A · Structural conditioning — lock the SHELL, not the fixtures.**
The star move: **shell-only conditioning** — a segmentation-masked canny that keeps only
wall/window/door/ceiling edges and drops furniture/fixture edges. That locks the shell
(honest openings) while leaving fixtures *free* to change (solves the openings problem AND
the bidet-mimicry problem at once). Explore fal segmentation preprocessors + masked canny;
also depth+canny combos and per-room-type strength.

**B · Wow at high lock.** At high structural lock the drama fades. Recover it with bolder
aesthetic prompts, best-of-N at mid strength (~0.8) selecting honest-AND-dramatic draws,
and the reference IP-Adapter (once its washout is tamed) to inject strong style while canny
holds structure.

**C · Intent-driven fixtures + Whisper.** Owner narration → Whisper → UNDERSTAND → prompt
drives fixture upgrades ("remove the bidet, tub → walk-in shower") and personal taste.
Needs Track A (so the prompt *can* change fixtures) + UNDERSTAND rules that enumerate the
owner's desired upgrades. New honesty mode: **owner-authorized** — allow the specific
changes the owner asked for (checked against stated intent), alongside strict/inspire.

**D · Gate calibration.** Tighten inspire's `believable` to catch tilted/warped geometry
(the crooked bedframe leaked through). Decide the interior-door policy (removed doors
currently pass inspire). Keep the décor leniency that makes inspire work.

**E · Reliability harness.** best-of-N + inspire gate (built: `director beststage`). Point
the golden harness at inspire mode + the depth-t2i engine; raise N where the per-draw
honest rate is low (cold run: ~48%/draw → best-of-3 = 7/9 rooms). Track per-draw rate by
room type.

**F · Productionize the winner.** Port the winning recipe (canny / shell-masked canny +
guardrail + strength) into the Go engine (`pkg/imageedit/fluxdepth.go`). Re-run the 2
failed cold-run rooms. fal webhooks for async in the deployed pipeline.

## The flywheel
Andreas's per-image feedback → taming rules in `playbook` (as in the Gemini era). Incoming
notes feed Tracks A–D.

## Immediate next steps (on resume)
1. Fold Andreas's incoming per-image notes into the tracks.
2. **Track A: shell-only (segmentation-masked) canny** — highest-leverage unlock (lock
   shell, free fixtures → fixes openings *and* the bidet-mimicry).
3. Track B: best-of-N at canny ~0.8 on the 2 failed rooms + bolder prompts.
