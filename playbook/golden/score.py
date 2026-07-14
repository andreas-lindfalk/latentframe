#!/usr/bin/env python3
"""Golden-set regression harness for the RESTAGE stage.

For every entry in manifest.json: re-run the room's prompt template on `before`,
then score the fresh candidate against the approved reference —
  - honesty:  `director verify`  (shell-vs-contents gate; exit 0 = honest)
  - quality:  `director judge`   (is candidate >= reference, right aesthetic, no new
                                  defects; exit 0 = holds the bar)
An entry PASSES only if it stays honest AND holds the bar. Run this after any prompt or
model change; ship the change only if nothing regresses.

    python3 playbook/golden/score.py            # score all
    python3 playbook/golden/score.py rosas_kitchen zeniamar_bath   # subset

Image generation is non-deterministic, so a single miss can be a fluke — re-run a
flagged entry before treating it as a real regression.
"""
import json, os, subprocess, sys, tempfile, glob, time

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
PROMPTS = os.path.join(ROOT, "playbook", "prompts")


def load_env():
    env = dict(os.environ)
    path = os.path.join(ROOT, ".env")
    if os.path.exists(path):
        for line in open(path):
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env.setdefault(k.strip(), v.strip())
    # the SDKs read these; make sure they're present
    for k in ("GEMINI_API_KEY", "ANTHROPIC_API_KEY"):
        if not env.get(k):
            sys.exit(f"missing {k} (put it in {path} or the environment)")
    return env


def build(env, bindir):
    for svc in ("director", "render"):
        out = os.path.join(bindir, svc)
        r = subprocess.run(["go", "build", "-o", out, "./services/" + svc], cwd=ROOT, env=env,
                           capture_output=True, text=True)
        if r.returncode != 0:
            sys.exit(f"go build {svc} failed:\n{r.stderr}")
    return os.path.join(bindir, "director"), os.path.join(bindir, "render")


def run(cmd, env):
    r = subprocess.run(cmd, env=env, capture_output=True, text=True)
    return r.returncode, (r.stdout or "") + (r.stderr or "")


def field(text, label):
    for line in text.splitlines():
        if line.strip().lower().startswith(label):
            return line.split(":", 1)[1].strip() if ":" in line else ""
    return ""


def main():
    env = load_env()
    manifest = json.load(open(os.path.join(HERE, "manifest.json")))
    wanted = set(sys.argv[1:])
    entries = [e for e in manifest["entries"] if not wanted or e["id"] in wanted]

    bindir = tempfile.mkdtemp(prefix="lf-golden-bin-")
    outdir = os.path.join(HERE, ".out")
    os.makedirs(outdir, exist_ok=True)
    print(f"building binaries… ({len(entries)} entries)")
    director, render = build(env, bindir)

    results, npass = [], 0
    print(f"\n{'entry':<20} {'honest':<7} {'quality':<8} {'verdict'}")
    print("─" * 60)
    for e in entries:
        before = os.path.join(HERE, e["before"])
        approved = os.path.join(HERE, e["approved"])
        tpl = os.path.join(PROMPTS, e["template"] + ".txt")
        prompt = open(tpl).read()
        cand = os.path.join(outdir, e["id"] + "_candidate.png")

        rc, _ = run([render, "restage", "--in", before, "--out", cand, "--prompt", prompt], env)
        got = sorted(glob.glob(os.path.join(outdir, e["id"] + "_candidate.*")))
        if rc != 0 or not got:
            results.append({**e, "honest": None, "meets_bar": None, "verdict": "ERROR (restage)"})
            print(f"{e['id']:<20} {'—':<7} {'—':<8} ERROR (restage)")
            continue
        cand = got[0]

        vrc, vout = run([director, "verify", "--before", before, "--after", cand, "--room", e["room"]], env)
        honest = (vrc == 0)
        jrc, jout = run([director, "judge", "--before", before, "--candidate", cand,
                         "--reference", approved, "--room", e["room"]], env)
        holds = (jrc == 0)
        quality = field(jout, "quality vs reference") or "?"

        ok = honest and holds
        npass += ok
        verdict = "PASS" if ok else ("REGRESS: " + ("dishonest" if not honest else "quality<ref"))
        results.append({"id": e["id"], "room": e["room"], "honest": honest, "meets_bar": holds,
                        "quality_vs_reference": quality, "verdict": verdict,
                        "verify_reason": field(vout, "reason"), "judge_reason": field(jout, "reason")})
        print(f"{e['id']:<20} {str(honest):<7} {quality:<8} {verdict}")

    print("─" * 60)
    print(f"{npass}/{len(entries)} hold the bar")
    json.dump({"passed": npass, "total": len(entries), "results": results},
              open(os.path.join(HERE, "results.json"), "w"), indent=2)
    print(f"→ results written to playbook/golden/results.json")
    sys.exit(0 if npass == len(entries) else 1)


if __name__ == "__main__":
    main()
