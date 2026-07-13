# One-Room Experiment

The whole company rests on one unproven bet: **can we make one room of a real Spanish
property look incredible — honestly — as a short clip?** Answer that here, by hand,
before automating anything.

## Runbook

1. **Record** a slow phone walkthrough of a single room, talking over it ("this kitchen
   is dark and dated; buyers want it bright and open"). Drop the file here, e.g.
   `kitchen.mov`. (Raw video is git-ignored — keep it local.)

2. **Ingest** — keyframes + Spanish transcript:
   ```bash
   make ingest VIDEO=experiments/one-room/kitchen.mov
   ```
   Output lands in `out/` (keyframes + printed narration segments).

3. **Pick the hero frame** — the sharpest, best-composed shot of the room from
   `out/keyframes/`.

4. **Re-stage the still** (by hand for now): structure-lock the hero frame (depth/edge)
   and generate the "after" — furniture replaced, **walls/windows/layout untouched**.
   Use the transcript to write the prompt.

5. **Honesty check:** same room? windows in the same place? believable? If it drifted,
   redo. (This is what stage 4 / VERIFY will automate.)

6. **Animate** the approved still with an image-to-video model — a short, tasteful
   camera move (slow push-in or gentle orbit). Keep the move small.

7. **Judge it.** Does it make you go *"that's my house, but incredible"*?
   - **Yes** → automate stages 1–6.
   - **Mushy / drifting** → tune the structure-conditioning before building infra.

## Notes

Log what worked here (models, prompts, camera-move limits) so the automated stages
inherit the taste, not just the code.
