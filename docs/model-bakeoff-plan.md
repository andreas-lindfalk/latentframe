# Model bake-off & window-masking plan

Where we are: the depth/canny-t2i engine + best-of-N + inspire gate ships reliable wow
(canny 0.7 = "wow is back", canny 0.8 = 9/9 rooms cold across two properties). But two
strategic gaps remain, and this plan closes them in the right order.

## Framing: stochastic for taste, deterministic for facts

The product has two requirement types, each with its correct tool:

- **Taste / wow — stochastic.** Gen models give a *distribution*, not deterministic wow.
  Prompts shift the distribution, best-of-N samples it, the gate + a human pick. Per-draw
  perfection is impossible and not the goal; "best-of-N reliably ships a wow" is achievable
  and we're ~there. **Aesthetic prompt-tuning generalizes acceptably — leave it, don't chase.**
- **Facts / structure — deterministic.** "Windows unchanged, exact openings" **cannot be
  prompted** (proven: the window clause failed at canny 0.7). It must be *enforced* by
  conditioning + pre/post-processing (masking). Never chase structure with prompts.

## Why a bake-off now (the biggest open gap)

We **assumed the model** — FLUX Control-LoRA canny was the *first* that worked, never the
*chosen* one. Mid-2026 in-context edit models may (a) preserve structure/windows **natively**
— which would make window-masking unnecessary — or (b) give better wow at honest structure.
We built the golden harness for exactly this decision. Settle it with data before investing
more in one model.

## Bake-off design

**Candidates** (confirm current fal availability first; wire each behind the `Editor`
interface or a thin experiment):
- FLUX Control-LoRA **canny** — our baseline.
- **FLUX.1 Kontext** — in-context edit, built to preserve scene while restyling.
- **Gemini** image editor (Nano-Banana / the current 2026 model) — strong instructed edit.
- **gpt-image** (OpenAI) — strong instruct-edit / inpaint.
- **Seedream / SeedEdit** (ByteDance), **Qwen-Image-Edit**.
- (Optional) SDXL/FLUX + ControlNet + inpaint — the masking-native route.

**Test set:** the golden rooms (both properties, all key room types), same before-images
and same per-room target prompts, so it's apples-to-apples.

**Scoring — objective, through the harness:**
- **Honesty:** inspire-VERIFY pass rate (structure/openings preserved).
- **Window fidelity (the sharp one):** are the *real* windows unchanged? (targeted check.)
- **Wow / quality:** `judge` (candidate vs the best reference) + Andreas's eye on winners.
- **Reliability:** per-draw honest rate (best-of-N efficiency — fewer wasted draws).
- **Cost + speed:** $/image, latency.
- **Bonus A/B:** Fable-5-as-judge vs Opus-4.8-as-judge on the same candidates — is Fable
  worth it for the moat calls (verify/judge/select)?

**The questions it answers:** Does any model preserve windows *natively* (→ skip masking)?
Which gives the best wow at honest structure? Which wastes the fewest draws?

## Then: window masking (only if no model does it natively)

Detect the source window/opening regions (SAM / segmentation), and **composite the REAL
window pixels back** over a low-canny (0.7, wow) render — feathered blend + optional
harmonization. Model-agnostic; a *hard guarantee* that windows can't change. Solves Andreas's
"windows shouldn't change" permanently while keeping the 0.7 wow.

## Taste

Leave it. Best-of-N + per-room prompts is good enough. Revisit only with specific per-image
notes. Do **not** keep prompt-massaging for structure.

## Sequence

1. Confirm fal availability of candidates; wire each.
2. Run all candidates on the golden rooms; score (harness + Andreas's eye).
3. Decide the engine (and whether Fable-judge earns its cost).
4. Build window-masking if the winner doesn't preserve windows natively.
5. Re-run cold; ship.

## Success criteria

- Engine choice backed by **scores, not assumption**.
- Windows **provably unchanged** — native preservation or masking.
- Reliable wow at breadth (≥ current 9/9, higher per-draw rate).
