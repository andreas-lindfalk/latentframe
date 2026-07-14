#!/usr/bin/env python3
"""Best-of-N restage — the reliability engine.

Image generation is non-deterministic (~78% of single draws hold the bar in golden-set
testing), so a single restage is a coin-flip on quality/honesty. This turns that
unreliable per-draw model into a reliable pipeline:

    1. GENERATE  N candidates from the same prompt (render restage x N)
    2. KEEP      only the ones that pass the honesty gate (director verify)
    3. SELECT    the single best of the survivors (director select)

Ship the winner. If zero candidates are honest, it fails loudly (exit 3) — that room
needs a stronger prompt or more draws, not a silent dishonest render.

    python3 playbook/bestof.py --in room.jpg --template kitchen --out after.png
    python3 playbook/bestof.py --in room.jpg --prompt-file my.txt --room kitchen -n 5 --out after.png --keep

Needs GEMINI_API_KEY + ANTHROPIC_API_KEY (from .env or the environment).
"""
import argparse, glob, json, os, shutil, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, ".."))
PROMPTS = os.path.join(HERE, "prompts")


def load_env():
    env = dict(os.environ)
    path = os.path.join(ROOT, ".env")
    if os.path.exists(path):
        for line in open(path):
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env.setdefault(k.strip(), v.strip())
    for k in ("GEMINI_API_KEY", "ANTHROPIC_API_KEY"):
        if not env.get(k):
            sys.exit(f"missing {k} (put it in {path} or the environment)")
    return env


def build(env, bindir):
    bins = {}
    for svc in ("director", "render"):
        out = os.path.join(bindir, svc)
        r = subprocess.run(["go", "build", "-o", out, "./services/" + svc], cwd=ROOT, env=env,
                           capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit(f"go build {svc} failed:\n{r.stderr}")
        bins[svc] = out
    return bins


def resolve_prompt(a):
    if a.prompt:
        return a.prompt
    if a.prompt_file:
        return open(a.prompt_file).read()
    if a.template:
        return open(os.path.join(PROMPTS, a.template + ".txt")).read()
    sys.exit("provide --template, --prompt-file, or --prompt")


def main():
    ap = argparse.ArgumentParser(description="Best-of-N restage: generate N, keep honest, pick best.")
    ap.add_argument("--in", dest="inp", required=True, help="input BEFORE image")
    ap.add_argument("--out", required=True, help="where to write the winning AFTER image")
    ap.add_argument("-n", "--n", type=int, default=3, help="candidates to generate (default 3)")
    ap.add_argument("--room", default="", help="room label, e.g. 'kitchen'")
    ap.add_argument("--template", help="prompt template name from playbook/prompts/<name>.txt")
    ap.add_argument("--prompt-file", dest="prompt_file", help="path to a prompt file")
    ap.add_argument("--prompt", help="inline prompt text")
    ap.add_argument("--keep", action="store_true", help="keep all candidates next to --out")
    a = ap.parse_args()

    env = load_env()
    prompt = resolve_prompt(a)
    bindir = tempfile.mkdtemp(prefix="lf-bestof-bin-")
    workdir = os.path.dirname(os.path.abspath(a.out)) if a.keep else tempfile.mkdtemp(prefix="lf-bestof-")
    stem = os.path.splitext(os.path.basename(a.out))[0]

    print(f"building binaries…")
    b = build(env, bindir)

    print(f"generating {a.n} candidate(s)…")
    honest = []  # (path, reason)
    for i in range(1, a.n + 1):
        cand = os.path.join(workdir, f"{stem}_cand{i}.png")
        r = subprocess.run([b["render"], "restage", "--in", a.inp, "--out", cand, "--prompt", prompt],
                           env=env, capture_output=True, text=True)
        got = sorted(glob.glob(os.path.join(workdir, f"{stem}_cand{i}.*")))
        if r.returncode != 0 or not got:
            print(f"  cand {i}: restage FAILED — {(r.stderr or r.stdout).strip()[:120]}")
            continue
        cand = got[0]
        v = subprocess.run([b["director"], "verify", "--before", a.inp, "--after", cand, "--room", a.room],
                           env=env, capture_output=True, text=True)
        ok = (v.returncode == 0)
        reason = next((ln.split(":", 1)[1].strip() for ln in (v.stdout or "").splitlines()
                       if ln.lower().startswith("reason")), "")
        print(f"  cand {i}: {'HONEST ✓' if ok else 'dishonest ✗'} {('— ' + reason) if reason else ''}"[:110])
        if ok:
            honest.append(cand)

    if not honest:
        print(f"\n✗ 0/{a.n} candidates were honest — nothing to ship. "
              f"Strengthen the prompt or raise -n.", file=sys.stderr)
        sys.exit(3)

    if len(honest) == 1:
        winner, why = honest[0], "only honest candidate"
    else:
        s = subprocess.run([b["director"], "select", "--before", a.inp, "--room", a.room, *honest],
                           env=env, capture_output=True, text=True)
        if s.returncode != 0:
            print(s.stdout + s.stderr, file=sys.stderr)
            sys.exit(f"select failed")
        best_path = next((ln.split(":", 1)[1].strip() for ln in s.stdout.splitlines()
                          if ln.lower().startswith("best path")), honest[0])
        why = next((ln.split(":", 1)[1].strip() for ln in s.stdout.splitlines()
                    if ln.lower().startswith("reason")), "")
        winner = best_path

    shutil.copyfile(winner, a.out)
    print(f"\n✓ shipped {len(honest)}/{a.n} honest → picked best")
    print(f"  winner : {os.path.basename(winner)}")
    print(f"  reason : {why}")
    print(f"  written: {a.out}")


if __name__ == "__main__":
    main()
