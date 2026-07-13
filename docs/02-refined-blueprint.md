# Latent Frame — Refined Blueprint (Source of Truth)

> This supersedes [`01-idea-mvp.md`](01-idea-mvp.md) (the earlier Gemini-generated blueprint,
> kept for history). Where the two disagree, **this document wins.**

## 1. What Latent Frame actually is

A B2B sales tool for the Spanish premium real-estate market. It turns a **phone
walkthrough video** of a property — with the owner/agent **talking over it** to give
context — into an **aspirational "after" video** that shows the property's *potential*:
the old, cluttered Spanish furniture removed and the space beautifully **re-staged**.

The problem it solves: Idealista/Kyero listings have poor photography, properties show
badly (dated furniture, clutter), and buyers are often **foreign** and can't see past it.
Latent Frame lets them *feel* what the home could be.

**The one inviolable rule: re-stage, never restructure.**
Empty the old furniture and furnish beautifully — but **never** move walls, windows, or
layout, and never imply a structural renovation. Aspirational, not deceptive. This is a
*feature*: it's what an agent can legally stand behind, and it's a machine-checkable
boundary (architecture identical, contents replaced).

## 2. The business is a funnel, not a single product

```
Idealista ad ──(agent's line-1 link)──► latentframe.ai/[property]
                                              │
        ┌─────────────────────┬───────────────┼────────────────────────┐
        ▼                     ▼               ▼                         ▼
   REVEAL REEL          before/after      LEAD CAPTURE             AFFILIATION
   (the hook)           sliders           (the business)           (the margin)
```

- **Video = the bait.** Earns and holds a foreign buyer's attention.
- **Property page = the trap.** Pulls them off the garbage listing onto our surface.
- **Lead = the catch.** Captured buyer intent is the actual product.
- **Affiliation = the meal.** Spanish mortgage brokers, solar/PVGIS, turnkey furniture —
  the high-margin layer, only viable *after* traffic + leads exist.
- **B2B asset fee (~€450/property)** is secondary cash flow, not the thesis.

Sequencing: (1) nail the video → (2) wrap in the page → (3) lead capture → (4) affiliation.
Steps 2–4 do not change what we build first.

## 3. The core technical bet (and its clean dodge)

The naive approach — transform the source walkthrough **frame-by-frame** — melts, badly,
*especially* for re-staging (the model must invent new objects and keep them consistent as
the camera moves). That path is where video-restage projects die.

**The dodge that makes this tractable:**
> Perfect **one hero still per room** (total control, geometrically honest), then use
> **image-to-video** to add a short, tasteful camera move.

There is only ever *one* re-staged reality, so consistency stops being the problem. The
quality dial becomes **how much camera motion we allow**: short cinematic moves (slow
push-in, gentle orbit) are flawless; big sweeps expose the AI's invention. Taste lives in
choosing moves that wow without overexposing.

## 4. Pipeline (6 stages)

```
Phone video (+ spoken context)
      │
1. INGEST     Go/GCP: scene-split into rooms; pick the sharpest, best-composed
      │       hero frame per room. (videra samples fps=1/N — NOT enough; smart
      │       frame selection is net-new work here.)
      ▼
2. UNDERSTAND Claude (vision + audio transcript): per-room restage brief +
      │       ONE global design vision so the whole house is coherent.
      ▼
3. RESTAGE    Structure-locked image gen (depth/edge from hero frame): produce the
      │       "after" STILL — walls/windows locked, furniture replaced.
      ▼
4. VERIFY     Claude (vision) honesty gate: architecture unchanged? believable?
      │       → reject & regenerate if it drifted.
      ▼
5. ANIMATE    Image-to-video: short cinematic camera move → per-room "after" clip.
      ▼
6. ASSEMBLE   Go + Next.js: stitch clips into a reveal reel (optionally intercut with
              real "before" footage), music, "AI visualization" label → the property page.
```

**Where the moat is:** stages **2 and 4** — Claude as an automated **art director**
(spoken intent → precise, coherent per-room prompts) and **QA/honesty gate** (regenerate
until architecture is preserved and the result is believable, no human in the loop).
Stage 3 (image gen) and stage 5 (image-to-video) are commodity API calls. The generators
are a race to the bottom; the taste + consistency + honesty **brain** is defensible.

## 5. What we reuse

- **From `~/dev/videra`** (older Go multimodal-video project) — lift organs, don't fork:
  `internal/ingestion/ffmpeg.go` (audio + keyframe extraction), `transcriber_whisper.go`
  (audio → timestamped segments), the async job-queue + job-state abstraction
  (submit → poll-by-jobId), and the Docker/Cloud Run deploy scaffolding. **Leave** videra's
  vector DB, CLIP, OCR, semantic search, and MCP server — that's a different product.
- **Structure inspiration only: `~/dev/goals/cloud`** — a mature Bazel + Go + buf/Connect +
  pnpm monorepo. Learn the *patterns*; **never copy code/config — it's employer IP.**

## 6. First build (de-risk the whole company in an afternoon)

Skip the pipeline. By hand, on **one room of the founder's own Spanish property**:
1. Pull one sharp hero frame.
2. Structure-lock it, re-stage it into a gorgeous "after" still (Claude writes the prompt
   from the spoken description; Claude judges the result for honesty).
3. Animate that still into a short clip with image-to-video.
4. Watch it. **Does it make you go "that's my house, but incredible"?**

Yes → automate stages 1–6 and add cross-room coherence. Mushy/drifting → tune the
structure-conditioning before building any infrastructure. Either way, the riskiest bet is
answered before a line of pipeline code is written.

## 7. Deferred (explicitly not now)

Navigable 3D / Gaussian-splat portals, MCP servers (Catastro/renovation-costing), the
capture app / "Uber for scanning" gig network, matched-camera-path before/after. All
Phase 2+, contingent on the video wow landing first.
