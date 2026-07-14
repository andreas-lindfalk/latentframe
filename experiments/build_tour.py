#!/usr/bin/env python3
"""Stitch per-room i2v clips into a single property tour with crossfades + a light grade.

    python3 build_tour.py out.mp4 clipA.mp4 clipB.mp4 ...

Each clip is normalized (1280x720, 30fps, trimmed to TRIM s, audio stripped), then
xfade-chained. A subtle, consistent grade (shadow lift + warmth + gentle vignette) is
applied to the whole tour so the rooms read as one cohesive piece.
"""
import subprocess, sys, os

TRIM = 6.0     # seconds kept per room
XF = 0.7       # crossfade duration
W, H, FPS = 1280, 720, 30

# subtle, tasteful grade (interior real-estate): lift shadows a touch, warm slightly,
# gentle saturation + vignette. Keep it light — over-graded reads fake.
GRADE = ("curves=r='0/0.02 0.5/0.52 1/1':b='0/0 0.5/0.48 1/0.98',"
         "eq=contrast=1.04:saturation=1.07,"
         "vignette=PI/5")


def main():
    out = sys.argv[1]
    clips = sys.argv[2:]
    if len(clips) < 2:
        sys.exit("need at least 2 clips")

    inputs = []
    for c in clips:
        inputs += ["-i", c]

    fc = []
    # normalize each input to a common format so xfade can splice them
    for i in range(len(clips)):
        fc.append(
            f"[{i}:v]trim=0:{TRIM},setpts=PTS-STARTPTS,"
            f"scale={W}:{H}:force_original_aspect_ratio=decrease,"
            f"pad={W}:{H}:(ow-iw)/2:(oh-ih)/2:color=black,"
            f"setsar=1,fps={FPS},format=yuv420p[v{i}]"
        )

    # xfade chain: offset accumulates as (TRIM - XF) per join
    prev = "v0"
    seg_len = TRIM
    for i in range(1, len(clips)):
        offset = seg_len - XF
        label = f"x{i}" if i < len(clips) - 1 else "xf"
        fc.append(f"[{prev}][v{i}]xfade=transition=fade:duration={XF}:offset={offset:.3f}[{label}]")
        prev = label
        seg_len = seg_len + (TRIM - XF)

    fc.append(f"[{prev}]{GRADE}[out]")

    cmd = ["ffmpeg", "-v", "error", "-y", *inputs,
           "-filter_complex", ";".join(fc),
           "-map", "[out]", "-an",
           "-c:v", "libx264", "-crf", "19", "-pix_fmt", "yuv420p", "-movflags", "+faststart",
           out]
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit("ffmpeg failed:\n" + r.stderr[-1500:])
    print(f"✓ wrote {out} ({os.path.getsize(out)//1024} KB), {len(clips)} rooms")


if __name__ == "__main__":
    main()
