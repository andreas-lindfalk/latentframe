# Latent Frame

Turns a phone walkthrough video of a property into an aspirational **"after" video** —
old furniture removed, the space beautifully re-staged — so foreign buyers can see a
Spanish listing's *potential*. A sales tool for real-estate agents.

**The one rule: re-stage, never restructure.** We change the contents, never the walls,
windows, or layout. Aspirational, not deceptive.

> Full plan: [`docs/02-refined-blueprint.md`](docs/02-refined-blueprint.md) (source of truth).

## The pipeline

```
video (+ spoken context)
  → INGEST      scene-split, pick the sharpest hero frame per room
  → UNDERSTAND  Claude: per-room brief + one global design vision
  → RESTAGE     structure-locked image gen of the "after" still
  → VERIFY      Claude honesty gate: architecture unchanged & believable?
  → ANIMATE     image-to-video: short cinematic camera move
  → ASSEMBLE    reveal reel + before/after → the property page
```

The defensible part is **UNDERSTAND + VERIFY** (Claude as automated art director and
honesty gate); the generators are commodities. See
[`pkg/pipeline/pipeline.go`](pkg/pipeline/pipeline.go).

## Layout

```
services/    deployable microservices, one per pipeline concern (see services/README.md)
  ingest/    stage 1 — video → keyframes/audio/transcript (live; drives the experiment)
pkg/         shared libraries: media/ (ffmpeg + whisper), pipeline/ (six-stage contract)
proto/       inter-service API contracts (buf/Connect — added at the first network boundary)
web/         Next.js property page (pnpm — added at the ASSEMBLE stage)
deploy/      per-service Docker + Cloud Run
docs/        blueprints
experiments/ one-room/ — the hand-made de-risk experiment
```

Services talk over an async job queue + proto; shared code in `pkg/`, service-private code
under `services/<svc>/internal/`. Plain Go tooling now; the layout is **Bazel-ready** for
when scale justifies it.

## Quick start

```bash
make build
# de-risk experiment: keyframes + Spanish transcript from your own footage
make ingest VIDEO=experiments/one-room/kitchen.mov
```

Requires `ffmpeg`. Transcription requires the Whisper CLI (`pip install -U openai-whisper`).
