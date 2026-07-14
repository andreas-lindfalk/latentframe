# Core Approach — Mid-2026 Capability Scouting

> Dated 2026-07-14. Scouting the single question that decides whether Latent Frame is a
> company: can we turn a **30-second walkthrough of a dump into the same 30-second walk,
> transformed into a gorgeous "here's the potential" version, in high quality**? Answer:
> **yes, this is buildable now.** This doc records the landscape, the chosen approach, and
> the spike that must validate it.

## ✅ SPIKE RESULT — validated 2026-07-14

Ran the make-or-break test: a real 12s gimbal walkthrough of an open-plan home →
**Kling O3 Pro** video-to-video edit (`fal-ai/kling-video/o3/pro/video-to-video/edit`,
via fal.ai) with one prompt ("keep architecture + camera, restage into a warm
Mediterranean villa"). **It works.** 1080p, ~$2 for 12s.

- **Architecture preserved** across the whole moving shot — columns, staircase,
  floor-to-ceiling windows, ceiling beams, doors, room proportions all held their
  positions. This is re-stage-never-restructure, for free (it edits the real footage).
- **Dramatic re-stage** — oak floor → terracotta tile, white → warm plaster, cool-modern
  furniture → warm Mediterranean palette. Photorealistic, cohesive.
- **Temporally stable** — coherent deep into the walk (t=11s), no flicker/melt despite real
  parallax (columns passing).

**Implication:** the north star is real, and v2v **collapses RESTAGE + ANIMATE into one
step** — feed the real clip + Claude's brief → out comes the transformed walk. RESTAGE-still
(Gemini) + Veo-i2v are no longer the core (kept only as anchor/fallback). Revised pipeline:
INGEST (clip + QC) → UNDERSTAND (Claude writes the transform prompt) → **v2v TRANSFORM**
(Kling/Happy Horse) → VERIFY (Claude on output frames) → **[optional] UPSCALE + ADD SOUND** → ASSEMBLE.

**Optional post-processing (all fal, same client — just different model ids):**
- **UPSCALE** (`fal-ai/topaz/upscale/video`, `seedvr`, `flashvsr`, …) — this is the "high
  quality" lever: transform lands at 1080p, upscale delivers crisp 4K.
- **ADD SOUND** (`mirelo-ai/sfx-v1.5/video-to-video`, `cassetteai/...`) — ambient room tone /
  SFX for the reveal reel.
Because they're the same fal async queue shape as TRANSFORM, each is a thin typed wrapper on
the generic fal client — the video track chains `transform → upscale → add-sound`, all swappable.

**THE quality bar (Andreas, 2026-07-14) — the hard requirement the spike did NOT yet prove:**
it is **not** enough to restyle existing furniture (brown sofa → white sofa). A room must be
viewable at its **full potential regardless of the crappy furniture crowding it** — i.e. the
v2v must **remove the old furniture entirely and re-furnish per the owner's VISION** (captured
via Whisper narration). The spike leaned *restyle* (kept the Barcelona chairs, changed
floors/palette); the real test is **wholesale re-staging driven by a specific spoken vision**.
This elevates UNDERSTAND: Claude turns the narration ("rip out this sofa, I picture a big linen
sectional facing the window, a reading nook here…") into precise remove/add v2v instructions.
Still consistent with re-stage-never-restructure (architecture fixed; *contents* fully reimaginable).

Next validations (in priority order): (1) **vision-driven wholesale re-stage** of a dated,
crowded room — the bar above; (2) shaky **handheld** footage (spike was a smooth gimbal);
(3) full **30s** length (Kling caps at 15s → Happy Horse, ≤60s).

**Planned infra (not now):** on INGEST, store the raw walkthrough **as-is in GCS** so we keep
the original material — behind a `pkg/storage` interface (GCS impl), mirroring cloud's
`pkg/cloudstorage` shape. This also supplies the public/signed URLs the fal video-track
consumes (so we never need fal's own upload). Structured logging via `pkg/log` (zap) and
testify `require` in tests are now in place from the start.

## The target (north star)

Input: a ~30s handheld phone walkthrough of a dated property.
Output: **the same walk** — same camera path, same timing, same rooms — but re-staged
(old furniture out, beautifully furnished, refreshed finishes), photorealistic and
high-res. One continuous transformed walkthrough, not a slideshow or a fresh clip.

Non-negotiable rule stands: **re-stage, never restructure** (architecture unchanged).

## The capability that unlocks it: in-context video-to-video editing

A tool class matured in 2025–2026 that edits **existing footage** from a text prompt while
**preserving the original camera motion, geometry, framing and cuts** — you "name only what
changes." This is exactly re-staging a real walk, and it makes our honesty rule nearly free
(it's the real property, edited — not invented).

### Candidate stack (ranked)

1. **Runway Aleph 2.0** — the closest fit. In-context v2v:
   - 2–30s in, **same length out**; up to **1080p**; source aspect preserved.
   - Preserves original **camera motion, scene geometry, framing, cuts**.
   - Ops: **add / remove / replace objects** (auto shadows/reflections/lighting), relight, restyle, background replace.
   - **Up to 5 frame anchors** pinned at timestamps — pin a restaged still, it propagates across the clip.
   - ~**$0.15/sec** (15 credits/s @ $0.01) → **~$4.50 per 30s**.
   - Needs **stable, well-lit, slow-to-moderate** camera moves; shake/blur degrade output.
   - ⚠️ **Runway API access moved to Enterprise (Jan 2026)** → reach it via a reseller (fal.ai / Runware / kie.ai), not direct.
2. **Kling 3.0** — strong, more accessible alternative (likely our primary):
   - **OmniEdit** (swap objects, relight, generative fill, reframe, cleanup — *preserving motion*) + **Video Restyle**.
   - **4K** capable (higher than Aleph); First/Last-frame control; canonical-still-as-I2V-seed; Multi-Shot Storyboard.
   - Explicitly used for **renovation before/after** videos in the wild.
3. **Higgsfield** — aggregator/studio: one interface to Sora 2 / Kling 3 / Veo 3.1 + a camera rig + character consistency. Useful for multi-model access and cinematic reveal styling.
4. **Sora 2** — best "world-state" spatiotemporal consistency; more generation than in-context edit.
5. **3D Gaussian Splatting → edit → re-render the camera path** (e.g. Splat Labs "AI Scene Redesign": remove furniture from any viewpoint by prompt; text/image-guided 3DGS editing; InteriorGS dataset). **Cleanest consistency** (it's a real 3D scene) and doubles as Gemini's navigable tour — but heavier pipeline, less mature. **Hold as the v2 / premium-consistency route.**

## Provider access reality (how we actually call these)

- **OpenRouter** (the key we have): launched a **video API in Apr 2026** — but it's **text-to-video / image-to-video *generation*** (Sora 2 Pro, Veo 3.1, Seedance, Wan, **Kling 3.0 as t2v/i2v**). It does **not** expose the **in-context v2v *edit*** mode we need (Runway/Aleph absent; Kling exposed as generation, not OmniEdit). → **OpenRouter is great for our LLM stages and for gen-video model-switching, but not for the core v2v-edit.**
- **fal.ai** — the right aggregator for the core: single key to **600+ media models incl. Kling and Runway**, fast inference infra. **Get a fal.ai key for the v2v-edit spike.** (Replicate is a pricier alternative with better docs.)
- **Gemini/Veo** (key we have): generation only (t2v/i2v) — cannot edit an existing clip in place. Stays useful for the RESTAGE still (Gemini image) and any i2v reveal fallback.

## How this re-composes what we already built (nothing wasted)

Aleph's frame-anchors / Kling's I2V-seed are the bridge between the two paths debated at the
start (Path A "matched walk" vs Path B "perfect one still"). They **merge**:

| Stage we built | Its new role in the v2v approach |
|---|---|
| INGEST (smart hero-frame selection) | pick the **anchor frame(s)** + capture-quality QC |
| UNDERSTAND (Claude art-director brief) | the **edit prompt** for the v2v model |
| RESTAGE (Gemini restages one still, gate-approved) | the **anchor/seed** the v2v model propagates across the whole walk |
| VERIFY (Claude honesty gate) | now run on **sampled output-video frames** |
| ANIMATE | **pivots**: "invent a dolly from a still" → "**transform the real walk, anchored on the approved still**" |
| ASSEMBLE | unchanged (page hosts the transformed walk) |

## Competitive read

The "AI renovation video" players (Media.io, TopMediai, ByRenovate, PropertyLimBrothers,
Ai4Spaces) are mostly **photo → generated reel/flythrough** or **still virtual staging**.
**Transforming a real handheld walkthrough in place** is not productized. The wedge holds;
the moat is orchestration + honesty gate + "it's the real walk."

## Risks the spike must resolve

1. Believable furniture **replacement** (not just restyle) across 30s of motion **without melt/flicker**.
2. How much **capture steadiness** matters (decides whether we need a capture protocol / AR app).
3. Access path confirmed (fal.ai → Kling-edit / Runway-Aleph).

## Recommendation + spike

**Core approach:** in-context v2v (**Kling 3.0 / Runway Aleph 2.0**), **anchored on our
gate-approved restaged still**, **prompted by Claude's brief**, **verified on output frames**,
generated at max res.

**The spike (make-or-break):** one steady ~30s walkthrough → run through Kling and Aleph via
fal.ai → judge melt / architecture preservation / wow. This single test pins ANIMATE and the
whole binary's output stage. Blockers: a **fal.ai key** + a **steady 30s clip** (stock to start).

## Sources
- Runway Aleph specs: https://runware.ai/docs/models/runway-aleph-2-0/guides/editing-video · pricing: https://www.pixazo.ai/models/runway
- Kling 3.0 / Sora 2 editing & consistency: https://www.atlascloud.ai/blog/guides/kling-3-0-vs-sora-2-0-which-is-the-best-ai-video-generator-for-2026
- Renovation-video use: https://www.media.io/video-effects/ai-renovation-video-generator.html
- 3DGS scene editing: https://www.splatlabs.ai/docs/ai/scene-redesign
- Higgsfield: https://higgsfield.ai/blog/5-Best-AI-Video-Models-2026-Tested-Compared
- OpenRouter video API: https://openrouter.ai/collections/video-models · https://www.kucoin.com/news/flash/openrouter-launches-video-generation-api-integrating-sora-2-veo-3-1-seedance
- fal.ai coverage: https://www.buildfastwithai.com/ai-tools/fal-ai
