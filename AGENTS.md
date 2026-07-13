# AGENTS.md — working conventions for Latent Frame

## What this is
See [`docs/02-refined-blueprint.md`](docs/02-refined-blueprint.md) (source of truth) and
[`README.md`](README.md). One-line: phone walkthrough video → aspirational "after" video
for Spanish real estate, monetized as video → property page → leads → affiliation.

## The inviolable product rule
**Re-stage, never restructure.** Change contents (furniture, decor, finishes); never move
walls, windows, or layout, and never imply structural renovation. The VERIFY stage exists
to enforce this — fail closed rather than ship a drifted room.

## Engineering conventions
- **Go 1.26**, single root module `github.com/andreas-lindfalk/latentframe`. Microservice
  layout: `services/<svc>/` (entrypoint `main.go`, private code under its own `internal/`),
  shared libraries in `pkg/`. See [`services/README.md`](services/README.md) for the service
  map. Run `make check` (fmt + vet + test) before commit.
- Plain Go tooling now; layout is **Bazel-ready** — adopt Bazel when CI pain or a second
  engineer justifies it, not before.
- Keep the moat (stages UNDERSTAND + VERIFY) sharp; treat image-gen and image-to-video as
  swappable commodity providers behind interfaces.

## Source boundaries (important)
- **`~/dev/videra`** — the author's own earlier project. Porting from it is fine and
  encouraged (ffmpeg + whisper organs already ported into `pkg/media`).
- **`~/dev/goals/cloud`** — the author's **employer's** repo. Learn its *patterns* only.
  **Never copy code, config, proto, or `.bzl` from it.**

## Scope discipline
Everything serves the one bet: *does one room look incredible, honestly, as a clip?*
Resist building infra (web page, deploy, proto, Bazel) ahead of that proof.
