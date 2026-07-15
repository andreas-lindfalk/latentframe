#!/usr/bin/env python3
"""Build a before/after gallery artifact from the depth-t2i cold run.

Pairs each room's real BEFORE with the shipped depth-t2i AFTER (from
playbook/golden/.coldrun), grouped by property, so the reliable-wow-at-breadth
question can be judged by eye. EVAL/reporting tooling — not part of the pipeline.

    python3 experiments/build_coldrun_gallery.py out.html
"""
import base64, os, subprocess, sys, tempfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
IMG = os.path.join(ROOT, "playbook", "golden", "images")
COLD = os.path.join(ROOT, "playbook", "golden", ".coldrun")

# id | display name | property
ROOMS = [
    ("zeniamar_living", "Living room", "Zeniamar"),
    ("zeniamar_kitchen", "Kitchen", "Zeniamar"),
    ("zeniamar_bath", "Bathroom", "Zeniamar"),
    ("zeniamar_bedroom", "Bedroom", "Zeniamar"),
    ("rosas_living", "Living room", "Calle las Rosas"),
    ("rosas_kitchen", "Kitchen", "Calle las Rosas"),
    ("rosas_bath", "Bathroom", "Calle las Rosas"),
    ("rosas_bath2", "Bathroom (2)", "Calle las Rosas"),
    ("rosas_bedroom", "Bedroom", "Calle las Rosas"),
]


def uri(path, tmp):
    """sips-optimize to ~780px JPEG, return a data URI (or '' if missing)."""
    if not os.path.exists(path):
        return ""
    out = os.path.join(tmp, os.path.basename(path) + ".opt.jpg")
    subprocess.run(["sips", "-s", "format", "jpeg", "-Z", "780", "-s", "formatOptions", "72",
                    path, "--out", out], capture_output=True)
    src = out if os.path.exists(out) else path
    with open(src, "rb") as f:
        return "data:image/jpeg;base64," + base64.b64encode(f.read()).decode()


def main():
    out_path = sys.argv[1] if len(sys.argv) > 1 else os.path.join(tempfile.gettempdir(), "coldrun.html")
    tmp = tempfile.mkdtemp(prefix="coldrun-gallery-")

    groups, shipped = {}, 0
    for rid, name, prop in ROOMS:
        before = uri(os.path.join(IMG, rid + "_before.jpg"), tmp)
        after = uri(os.path.join(COLD, rid + "_after.jpg"), tmp)
        groups.setdefault(prop, []).append((name, before, after))
        if after:
            shipped += 1

    cards = []
    for prop, rooms in groups.items():
        cards.append(f'<h2>{prop}</h2><div class="grid">')
        for name, before, after in rooms:
            after_html = (f'<figure><img src="{after}" alt="after"><figcaption class="a">After · depth-t2i</figcaption></figure>'
                          if after else '<figure class="missing"><div>no honest candidate</div></figure>')
            cards.append(
                f'<div class="pair"><h3>{name}</h3><div class="ba">'
                f'<figure><img src="{before}" alt="before"><figcaption class="b">Before · real photo</figcaption></figure>'
                f'{after_html}</div></div>')
        cards.append('</div>')
    body = "\n".join(cards)

    html = f"""<title>Cold run — depth-t2i across both houses</title>
<style>
  :root {{ --bg:#f3f1ec; --surface:#fff; --ink:#20242b; --muted:#6d7178; --line:#e0ddd5;
    --accent:#b5763a; --sans:'Helvetica Neue',Inter,system-ui,Arial,sans-serif; --mono:'SF Mono',ui-monospace,Menlo,monospace; }}
  @media (prefers-color-scheme:dark) {{ :root {{ --bg:#101214; --surface:#181b1f; --ink:#e9e7e1; --muted:#9b9aa0; --line:#2a2e34; --accent:#d59a5f; }} }}
  :root[data-theme=dark] {{ --bg:#101214; --surface:#181b1f; --ink:#e9e7e1; --muted:#9b9aa0; --line:#2a2e34; --accent:#d59a5f; }}
  :root[data-theme=light] {{ --bg:#f3f1ec; --surface:#fff; --ink:#20242b; --muted:#6d7178; --line:#e0ddd5; --accent:#b5763a; }}
  * {{ box-sizing:border-box; }}
  body {{ margin:0; background:var(--bg); color:var(--ink); font-family:var(--sans); line-height:1.5; -webkit-font-smoothing:antialiased; }}
  .wrap {{ max-width:1180px; margin:0 auto; padding:clamp(24px,4vw,52px) clamp(16px,3vw,32px) 72px; }}
  .eyebrow {{ font-family:var(--mono); font-size:12px; letter-spacing:.18em; text-transform:uppercase; color:var(--accent); margin:0 0 12px; }}
  h1 {{ font-size:clamp(28px,5vw,46px); letter-spacing:-.02em; margin:0 0 12px; font-weight:800; }}
  .dek {{ color:var(--muted); max-width:64ch; margin:0 0 8px; font-size:16px; }}
  h2 {{ font-size:22px; margin:44px 0 4px; padding-bottom:8px; border-bottom:1px solid var(--line); }}
  .grid {{ display:grid; grid-template-columns:1fr; gap:26px; margin-top:20px; }}
  .pair h3 {{ font-size:14px; font-family:var(--mono); letter-spacing:.04em; color:var(--muted); margin:0 0 8px; text-transform:uppercase; }}
  .ba {{ display:grid; grid-template-columns:1fr 1fr; gap:12px; }}
  @media (max-width:640px) {{ .ba {{ grid-template-columns:1fr; }} }}
  figure {{ margin:0; background:var(--surface); border:1px solid var(--line); border-radius:12px; overflow:hidden; }}
  img {{ display:block; width:100%; height:auto; }}
  figcaption {{ font-family:var(--mono); font-size:11px; letter-spacing:.05em; padding:8px 10px; color:var(--muted); }}
  figcaption.a {{ color:var(--accent); }}
  .missing {{ display:flex; align-items:center; justify-content:center; min-height:200px; color:var(--muted); font-size:13px; }}
  footer {{ margin-top:48px; padding-top:18px; border-top:1px solid var(--line); font-family:var(--mono); font-size:11.5px; color:var(--muted); }}
</style>
<div class="wrap">
  <p class="eyebrow">Latent Frame · cold run</p>
  <h1>depth-t2i, across both houses</h1>
  <p class="dek">Every key room of both properties, run cold through the new engine — depth-locked
     FLUX generation, best-of-3, selected on the <b>inspire</b> honesty bar. No per-photo tuning.</p>
  <p class="dek"><b>{shipped}/{len(ROOMS)}</b> rooms shipped an honest wow candidate.</p>
  {body}
  <footer>depth-t2i · best-of-3 · inspire gate · director beststage · Latent Frame</footer>
</div>"""
    with open(out_path, "w") as f:
        f.write(html)
    print(f"wrote {out_path} ({os.path.getsize(out_path)//1024} KB), {shipped}/{len(ROOMS)} shipped")


if __name__ == "__main__":
    main()
