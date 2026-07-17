#!/usr/bin/env python3
"""Model bake-off — run one room through every 2026 in-context EDIT model on fal.

The question: do the current editors keep the architecture (windows/doors!) AND fully gut
the dated furniture — the thing early-Gemini couldn't — beating our FLUX-canny baseline?
See docs/model-bakeoff-plan.md. EVAL tooling (not the pipeline).

    python3 experiments/bakeoff.py --in room.jpg --out-dir out/ --prompt "Restage ... keep architecture ..."

Needs FAL_API_KEY. Downloads <model>.jpg per candidate for side-by-side + harness scoring.
"""
import argparse, base64, json, os, sys, time, urllib.request, urllib.error

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
FAL_QUEUE = "https://queue.fal.run"
FAL_UPLOAD_INIT = "https://rest.alpha.fal.ai/storage/upload/initiate?storage_type=fal-cdn-v3"

# name | endpoint | image input field (single image_url vs image_urls array)
MODELS = [
    ("kontext",         "fal-ai/flux-pro/kontext",              "image_url"),
    ("qwen-edit",       "fal-ai/qwen-image-edit",               "image_url"),
    ("nano-banana-2",   "fal-ai/nano-banana-2/edit",            "image_urls"),
    ("nano-banana-pro", "fal-ai/nano-banana-pro/edit",          "image_urls"),
    ("seedream-4.5",    "fal-ai/bytedance/seedream/v4.5/edit",  "image_urls"),
]


def fal_key():
    k = os.environ.get("FAL_API_KEY") or os.environ.get("FAL_KEY")
    if not k:
        p = os.path.join(ROOT, ".env")
        if os.path.exists(p):
            for line in open(p):
                line = line.strip()
                if line.startswith(("FAL_API_KEY=", "FAL_KEY=")):
                    k = line.split("=", 1)[1].strip().strip('"\'')
                    break
    if not k:
        sys.exit("no FAL_API_KEY")
    return k


KEY = None


def _req(url, method="GET", body=None, headers=None, raw=False):
    data, h = None, dict(headers or {})
    if body is not None and not raw:
        data = json.dumps(body).encode(); h.setdefault("Content-Type", "application/json")
    elif raw:
        data = body
    req = urllib.request.Request(url, data=data, method=method, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=120) as r:
            rb = r.read()
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"HTTP {e.code}: {e.read().decode(errors='replace')[:400]}")
    return rb if raw else (json.loads(rb) if rb else {})


def upload(path):
    init = _req(FAL_UPLOAD_INIT, "POST", {"content_type": "image/jpeg", "file_name": os.path.basename(path)},
               {"Authorization": f"Key {KEY}"})
    with open(path, "rb") as f:
        _req(init["upload_url"], "PUT", f.read(), {"Content-Type": "image/jpeg"}, raw=True)
    return init["file_url"]


def find_url(o):
    if isinstance(o, dict):
        if o.get("images"): return o["images"][0].get("url")
        if isinstance(o.get("image"), dict): return o["image"].get("url")
        for v in o.values():
            u = find_url(v)
            if u: return u
    if isinstance(o, list):
        for e in o:
            u = find_url(e)
            if u: return u
    return None


def run(model, inp, timeout=300):
    sub = _req(f"{FAL_QUEUE}/{model}", "POST", inp, {"Authorization": f"Key {KEY}"})
    su, ru = sub.get("status_url"), sub.get("response_url")
    if not su or not ru:
        raise RuntimeError(f"no status/response url: {sub}")
    end = time.time() + timeout
    while time.time() < end:
        st = _req(su, "GET", headers={"Authorization": f"Key {KEY}"})
        s = st.get("status", "")
        if s == "COMPLETED":
            return _req(ru, "GET", headers={"Authorization": f"Key {KEY}"})
        if s not in ("IN_QUEUE", "IN_PROGRESS", ""):
            raise RuntimeError(f"status {s}: {st}")
        time.sleep(4)
    raise RuntimeError("timed out")


def main():
    global KEY
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="inp", required=True)
    ap.add_argument("--out-dir", required=True)
    ap.add_argument("--prompt", required=True, help="edit instruction (keep architecture, gut furniture, style)")
    ap.add_argument("--only", help="comma-separated subset of model names")
    ap.add_argument("--ref", help="extra reference image URL (e.g. a product photo) — for image_urls models")
    a = ap.parse_args()
    KEY = fal_key()
    os.makedirs(a.out_dir, exist_ok=True)

    print(f"uploading {os.path.basename(a.inp)} …")
    src = upload(a.inp)
    want = set(a.only.split(",")) if a.only else None

    for name, ep, imgfield in MODELS:
        if want and name not in want:
            continue
        if imgfield == "image_urls":
            imgs = [src] + ([a.ref] if a.ref else [])
            inp = {"image_urls": imgs, "prompt": a.prompt}
        else:
            inp = {"image_url": src, "prompt": a.prompt}
        try:
            t = time.time()
            res = run(ep, inp)
            url = find_url(res)
            if not url:
                print(f"  {name:<16} NO IMAGE: {json.dumps(res)[:180]}"); continue
            out = os.path.join(a.out_dir, f"{name}.jpg")
            with open(out, "wb") as f:
                f.write(_req(url, "GET", raw=True))
            print(f"  {name:<16} ✓ {int(time.time()-t)}s → {out}")
        except Exception as e:
            print(f"  {name:<16} ✗ {e}")


if __name__ == "__main__":
    main()
