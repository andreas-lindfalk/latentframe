#!/usr/bin/env python3
"""Aesthetic-engine POC — the honest style engine (structure-lock + reference-style).

Thesis (Andreas): capture the Mediterranean look by SHOWING the model a reference image,
not describing it — but split the two signals so honesty holds:
  - STRUCTURE from the source room (depth ControlNet) -> architecture can't drift, and dated
    furniture isn't anchored (unlike Gemini in-context edit);
  - STYLE from a curated reference (IP-Adapter) or, in this first pass, from prose.

Pipeline (this pass = depth-lock + prose):
  source.jpg --(fal depth preprocessor)--> depth map
  depth map + prompt --(FLUX Control-LoRA Depth)--> restaged room

    python3 experiments/aesthetic_engine.py --in room.jpg --out out.jpg \
        --prompt "warm Mediterranean living room, ..." [--strength 0.95] [--depth-scale 0.8]

Needs FAL_API_KEY (or FAL_KEY) in .env or the environment.
"""
import argparse, base64, json, os, sys, time, urllib.request, urllib.error

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
FAL_QUEUE = "https://queue.fal.run"
FAL_UPLOAD_INIT = "https://rest.alpha.fal.ai/storage/upload/initiate?storage_type=fal-cdn-v3"

# structural conditioning: depth = 3D layout (blind to flat openings — loses doors/windows);
# canny = edges (locks door/window/opening LINES — the taming lever for removed openings).
CONTROL = {
    "depth": {"pre": "fal-ai/imageutils/depth",
              "t2i": "fal-ai/flux-control-lora-depth",
              "i2i": "fal-ai/flux-control-lora-depth/image-to-image"},
    "canny": {"pre": "fal-ai/image-preprocessors/canny",
              "t2i": "fal-ai/flux-control-lora-canny",
              "i2i": "fal-ai/flux-control-lora-canny/image-to-image"},
}
FLUX_GENERAL = "fal-ai/flux-general"                          # controlnet(depth) + IP-adapter(style ref)


def fal_key():
    k = os.environ.get("FAL_API_KEY") or os.environ.get("FAL_KEY")
    if not k:
        env = os.path.join(ROOT, ".env")
        if os.path.exists(env):
            for line in open(env):
                line = line.strip()
                if line.startswith(("FAL_API_KEY=", "FAL_KEY=")):
                    k = line.split("=", 1)[1].strip().strip('"\'')
                    break
    if not k:
        sys.exit("no FAL_API_KEY (put it in .env or the environment)")
    return k


KEY = None


def _req(url, method="GET", body=None, headers=None, raw=False):
    data = None
    h = headers or {}
    if body is not None and not raw:
        data = json.dumps(body).encode()
        h.setdefault("Content-Type", "application/json")
    elif raw:
        data = body
    req = urllib.request.Request(url, data=data, method=method, headers=h)
    try:
        with urllib.request.urlopen(req, timeout=120) as r:
            rb = r.read()
    except urllib.error.HTTPError as e:
        body = e.read().decode(errors="replace")
        print(f"    HTTP {e.code} from {url.split('?')[0]}: {body[:600]}", file=sys.stderr)
        raise
    return rb if raw else (json.loads(rb) if rb else {})


def upload(path):
    """Upload a local file to fal storage, return its public URL."""
    init = _req(FAL_UPLOAD_INIT, "POST",
                {"content_type": "video/mp4" if path.endswith(".mp4") else "image/jpeg",
                 "file_name": os.path.basename(path)},
                {"Authorization": f"Key {KEY}"})
    with open(path, "rb") as f:
        _req(init["upload_url"], "PUT", f.read(),
             {"Content-Type": "image/jpeg"}, raw=True)
    return init["file_url"]


def find_url(obj):
    """Pull the first result image URL out of a fal response."""
    if isinstance(obj, dict):
        if "images" in obj and obj["images"]:
            return obj["images"][0].get("url")
        if "image" in obj and isinstance(obj["image"], dict):
            return obj["image"].get("url")
        for v in obj.values():
            u = find_url(v)
            if u:
                return u
    if isinstance(obj, list):
        for e in obj:
            u = find_url(e)
            if u:
                return u
    return None


def run(model, inp, poll=5, timeout=600):
    """Submit to a fal model, poll the queue, return the result JSON."""
    sub = _req(f"{FAL_QUEUE}/{model}", "POST", inp, {"Authorization": f"Key {KEY}"})
    status_url, response_url = sub.get("status_url"), sub.get("response_url")
    if not status_url or not response_url:
        sys.exit(f"fal submit ({model}) gave no status/response url: {sub}")
    deadline = time.time() + timeout
    last = None
    while time.time() < deadline:
        st = _req(status_url + "?logs=0", "GET", headers={"Authorization": f"Key {KEY}"})
        s = st.get("status", "")
        tag = f"{s} q={st.get('queue_position')}"
        if tag != last:
            print(f"    [{int(time.time()-(deadline-timeout))}s] {tag}", flush=True)
            last = tag
        if s == "COMPLETED":
            return _req(response_url, "GET", headers={"Authorization": f"Key {KEY}"})
        if s not in ("IN_QUEUE", "IN_PROGRESS", ""):
            sys.exit(f"fal ({model}) status {s}: {st}")
        time.sleep(poll)
    sys.exit(f"fal ({model}) timed out after {timeout}s (last status: {last})")


def download(url, path):
    _req_out = _req(url, "GET", raw=True)
    with open(path, "wb") as f:
        f.write(_req_out)


def main():
    global KEY
    ap = argparse.ArgumentParser()
    ap.add_argument("--in", dest="inp", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--prompt", required=True)
    ap.add_argument("--strength", type=float, default=0.95, help="i2i denoise; high = ignore source pixels")
    ap.add_argument("--depth-scale", type=float, default=0.8, help="control_lora_strength (structure lock)")
    ap.add_argument("--control", choices=["depth", "canny"], default="depth",
                    help="structural conditioning: depth (3D layout) or canny (edges — locks door/window lines)")
    ap.add_argument("--steps", type=int, default=30)
    ap.add_argument("--guidance", type=float, default=3.5)
    ap.add_argument("--t2i", action="store_true", help="fresh generation from control+prompt (no source pixels)")
    ap.add_argument("--ref", help="style reference image (IP-Adapter, via flux-general) — show the aesthetic")
    ap.add_argument("--ref-scale", type=float, default=0.85, help="IP-Adapter strength")
    ap.add_argument("--keep-depth", help="optional path to also save the depth map")
    a = ap.parse_args()
    KEY = fal_key()

    print(f"1/3 uploading {os.path.basename(a.inp)} …")
    src_url = upload(a.inp)

    ctrl = CONTROL[a.control]
    print(f"2/3 {a.control} preprocess …")
    pre = run(ctrl["pre"], {"image_url": src_url})
    depth_url = find_url(pre)
    if not depth_url:
        sys.exit(f"no {a.control} map in response: {json.dumps(pre)[:400]}")
    if a.keep_depth:
        download(depth_url, a.keep_depth)

    if a.ref:
        print(f"3/3 reference restage (flux-general: depth ControlNet + IP-Adapter style) …")
        ref_url = upload(a.ref)
        res = run(FLUX_GENERAL, {
            "prompt": a.prompt,
            "image_size": "landscape_4_3",
            "num_inference_steps": a.steps,
            "guidance_scale": a.guidance,
            "output_format": "jpeg",
            "controlnets": [{
                "path": "jasperai/Flux.1-dev-Controlnet-Depth",
                "control_image_url": depth_url,
                "conditioning_scale": a.depth_scale,
            }],
            "ip_adapters": [{
                "path": "XLabs-AI/flux-ip-adapter",
                "weight_name": "ip_adapter.safetensors",
                "image_encoder_path": "openai/clip-vit-large-patch14",
                "image_url": ref_url,
                "scale": a.ref_scale,
            }],
        })
    else:
        mode = "t2i (fresh content)" if a.t2i else "i2i"
        print(f"3/3 {a.control}-locked restage (FLUX Control-LoRA {a.control}, {mode}) …")
        inp = {
            "control_lora_image_url": depth_url,
            "prompt": a.prompt,
            "control_lora_strength": a.depth_scale,
            "num_inference_steps": a.steps,
            "guidance_scale": a.guidance,
            "image_size": "landscape_4_3",
            "output_format": "jpeg",
        }
        if a.t2i:
            res = run(ctrl["t2i"], inp)
        else:
            res = run(ctrl["i2i"], {**inp, "image_url": src_url, "strength": a.strength})
    out_url = find_url(res)
    if not out_url:
        sys.exit(f"no output image in response: {json.dumps(res)[:400]}")
    download(out_url, a.out)
    print(f"✓ wrote {a.out}")
    print(f"  next: director verify --before {a.inp} --after {a.out}")


if __name__ == "__main__":
    main()
