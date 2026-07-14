# Golden-set regression harness

The safety rail for tuning prompts and swapping models: **improve without silently
regressing** rooms that already look good. This is how "consistency" — the actual
product — gets defended.

## The idea

Every approved "after" is kept as a **golden reference**. When a prompt or model changes,
we re-run every golden room through the current template and check the fresh output still:

1. **stays honest** — `director verify` (shell-vs-contents gate: walls/windows/openings
   unchanged, room stays functional), and
2. **holds the quality bar** — `director judge` (a Claude vision judge: is the candidate at
   least as good as the approved reference, in the right aesthetic, with no *new* defects
   like kept-dated-furniture, a half-painted wall, a blocked door, or a washed-out sky).

An entry **passes** only if it's honest AND holds the bar. A change ships only if every
entry passes. Image generation is non-deterministic, so the check is "clears the bar", not
a pixel match — and a lone miss can be a fluke (re-run before calling it a regression).

## Run it

```bash
python3 playbook/golden/score.py                 # score every golden room
python3 playbook/golden/score.py rosas_kitchen   # a subset
```

Needs `GEMINI_API_KEY` + `ANTHROPIC_API_KEY` (from `.env` or the environment). It builds
the `director` and `render` binaries, re-runs each room's `playbook/prompts/<template>.txt`,
scores it, prints a table, and writes `results.json`. Exit 0 iff all pass.

## Files

- `manifest.json` — the golden entries: `{id, room, template, before, approved}`.
- `images/` — the `before` (real listing photo) and `approved` (blessed after) for each entry.
- `score.py` — the runner (restage → verify → judge → report).
- `.out/`, `results.json` — runtime outputs (gitignored).

## Adding a golden entry

When a new "after" is approved (owner/reviewer blessed it), add it: drop
`images/<id>_before.jpg` and `images/<id>_after.jpg`, and append an entry to
`manifest.json` with the template it used. The set grows with every property — that
growing, curated set is the compounding, defensible asset.

## The judge

`director judge --before <before> --candidate <new> --reference <approved> --room <type>`
→ `services/director/internal/judge`. Opus, forced structured verdict
`{meets_bar, quality_vs_reference, reason}`. Sibling to the VERIFY gate
(`internal/verify`): VERIFY guards honesty, judge guards quality.
