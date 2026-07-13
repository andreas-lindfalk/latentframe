# services/ — deployable microservices

Each pipeline concern is (or becomes) an independently deployable service, connected by
an **async job queue** (video restage is a minutes-long job) and typed proto contracts in
[`../proto`](../proto). One property flows through them like a job through a shop floor.

| service     | pipeline stages          | responsibility                                     | status                |
|-------------|--------------------------|----------------------------------------------------|-----------------------|
| `ingest`    | 1 · INGEST               | video → keyframes, audio, transcript, hero frames  | **live** (local one-shot) |
| `director`  | 2 · UNDERSTAND, 4 · VERIFY | Claude art-director + honesty gate — **the moat**  | **live** — UNDERSTAND + VERIFY (strict tool use) |
| `render`    | 3 · RESTAGE, 5 · ANIMATE | image gen + image-to-video (commodity providers)   | **live** — RESTAGE (Gemini) + ANIMATE (Veo 3.1), both proven on real input |
| `assembler` | 6 · ASSEMBLE             | property page: reveal + before/after + lead capture | **live** (`build` + `serve`) |
| `api`       | —                        | job submit/status gateway (lead capture seeded in assembler `serve`) | planned |

The interesting flow is the **`director` ↔ `render` loop**: render produces the "after",
director's VERIFY approves it or bounces it back to be regenerated — the honesty gate,
enforced across a service boundary.

## Conventions
- Shared domain types + the six-stage contract live in [`../pkg/pipeline`](../pkg/pipeline);
  shared media tools in [`../pkg/media`](../pkg/media).
- Service-private code goes under `services/<svc>/internal/`.
- Entry point is `services/<svc>/main.go`. Per-service Docker/Cloud Run config in
  [`../deploy`](../deploy).

We stand up a service only when its stage has earned automation — `ingest` first, because
it feeds the one-room experiment. No empty-service theater.
