#!/usr/bin/env python3
"""Build the shoppable "potential" property page (funnel MVP demand test).

Reads web/funnel/<property>.json + the before/after renders, and emits a self-contained
editorial page: room-by-room before/after + "shop this room" cards (affiliate links).
The MVP test: does anyone click to shop the look? EVAL/product-preview tooling for now
(productionise into a Go-served page once the shape is validated).

    python3 experiments/build_funnel_page.py web/funnel/zeniamar.json out.html
"""
import base64, json, os, subprocess, sys, tempfile

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
# durable, committed page assets — NOT the gitignored .coldrun (which gets wiped)
RENDERS = os.path.join(ROOT, "web", "funnel", "renders")


def uri(path, tmp):
    if not os.path.exists(path):
        return ""
    out = os.path.join(tmp, os.path.basename(path) + ".opt.jpg")
    subprocess.run(["sips", "-s", "format", "jpeg", "-Z", "1000", "-s", "formatOptions", "78",
                    path, "--out", out], capture_output=True)
    src = out if os.path.exists(out) else path
    with open(src, "rb") as f:
        return "data:image/jpeg;base64," + base64.b64encode(f.read()).decode()


def main():
    data = json.load(open(sys.argv[1]))
    out_path = sys.argv[2] if len(sys.argv) > 2 else os.path.join(tempfile.gettempdir(), "funnel.html")
    tmp = tempfile.mkdtemp(prefix="funnel-")

    sections = []
    for r in data["rooms"]:
        before = uri(os.path.join(RENDERS, r["id"] + "_before.jpg"), tmp)
        after = uri(os.path.join(RENDERS, r["id"] + "_after.jpg"), tmp)
        cards = "".join(
            f'<a class="card" href="{p["url"]}" target="_blank" rel="noopener" data-track="shop" '
            f'data-room="{r["id"]}" data-product="{p["name"]}">'
            f'<span class="ret">{p["retailer"]}</span>'
            f'<span class="pname">{p["name"]}</span>'
            f'<span class="cardfoot"><span class="price">{p["price"]}</span><span class="shop">Shop →</span></span>'
            f'</a>' for p in r["products"])
        sections.append(f"""
  <section class="room">
    <div class="rhead"><h2>{r['name']}</h2><p>{r['blurb']}</p></div>
    <div class="ba">
      <figure><img src="{before}" alt="{r['name']} before" loading="lazy"><figcaption>Now</figcaption></figure>
      <figure class="af"><img src="{after}" alt="{r['name']} after" loading="lazy"><figcaption>The potential</figcaption></figure>
    </div>
    <div class="shoprow"><p class="shoplabel">Shop this room</p><div class="cards">{cards}</div></div>
  </section>""")
    body = "\n".join(sections)

    html = f"""<title>{data['property']} — the potential</title>
<style>
  :root {{
    --bg:#f6f4f0; --surface:#ffffff; --ink:#23201c; --muted:#7a756c; --line:#e4dfd6;
    --accent:#c25e3a; --olive:#55603f;
    --sans:'Helvetica Neue',Inter,system-ui,-apple-system,Arial,sans-serif;
    --mono:'SF Mono',ui-monospace,'JetBrains Mono',Menlo,monospace;
  }}
  @media (prefers-color-scheme:dark) {{ :root {{
    --bg:#16140f; --surface:#1e1b16; --ink:#ece8e0; --muted:#a0998f; --line:#2e2a22; --accent:#d8794f; --olive:#8a9366; }} }}
  :root[data-theme=dark] {{ --bg:#16140f; --surface:#1e1b16; --ink:#ece8e0; --muted:#a0998f; --line:#2e2a22; --accent:#d8794f; --olive:#8a9366; }}
  :root[data-theme=light] {{ --bg:#f6f4f0; --surface:#ffffff; --ink:#23201c; --muted:#7a756c; --line:#e4dfd6; --accent:#c25e3a; --olive:#55603f; }}
  * {{ box-sizing:border-box; }}
  body {{ margin:0; background:var(--bg); color:var(--ink); font-family:var(--sans); line-height:1.55;
    -webkit-font-smoothing:antialiased; }}
  .wrap {{ max-width:1120px; margin:0 auto; padding:0 clamp(16px,4vw,40px); }}
  header {{ padding:clamp(40px,7vw,90px) 0 clamp(28px,4vw,52px); }}
  .eyebrow {{ font-family:var(--mono); font-size:12px; letter-spacing:.24em; text-transform:uppercase;
    color:var(--accent); margin:0 0 18px; }}
  h1 {{ font-size:clamp(40px,9vw,92px); line-height:.98; letter-spacing:-.03em; font-weight:800; margin:0;
    text-wrap:balance; }}
  h1 em {{ font-style:normal; color:var(--accent); }}
  .lede {{ font-size:clamp(17px,2.4vw,22px); color:var(--muted); max-width:44ch; margin:22px 0 0; text-wrap:balance; }}
  .room {{ padding:clamp(34px,6vw,72px) 0; border-top:1px solid var(--line); }}
  .rhead {{ display:flex; flex-wrap:wrap; align-items:baseline; gap:6px 20px; margin:0 0 22px; }}
  .rhead h2 {{ font-size:clamp(24px,3.6vw,38px); letter-spacing:-.02em; margin:0; font-weight:750; }}
  .rhead p {{ color:var(--muted); font-size:15.5px; margin:0; max-width:52ch; flex:1 1 320px; }}
  .ba {{ display:grid; grid-template-columns:1fr 1fr; gap:14px; }}
  @media (max-width:680px) {{ .ba {{ grid-template-columns:1fr; }} }}
  figure {{ margin:0; position:relative; border-radius:14px; overflow:hidden; border:1px solid var(--line);
    background:var(--surface); }}
  figure img {{ display:block; width:100%; height:100%; object-fit:cover; aspect-ratio:3/2; }}
  figcaption {{ position:absolute; left:12px; bottom:12px; font-family:var(--mono); font-size:11px;
    letter-spacing:.08em; text-transform:uppercase; color:#fff; background:rgba(20,16,10,.62);
    padding:5px 10px; border-radius:999px; backdrop-filter:blur(4px); }}
  figure.af figcaption {{ background:var(--accent); }}
  .shoprow {{ margin-top:22px; }}
  .shoplabel {{ font-family:var(--mono); font-size:12px; letter-spacing:.16em; text-transform:uppercase;
    color:var(--olive); margin:0 0 12px; }}
  .cards {{ display:grid; grid-template-columns:repeat(4,1fr); gap:12px; }}
  @media (max-width:820px) {{ .cards {{ grid-template-columns:repeat(2,1fr); }} }}
  @media (max-width:460px) {{ .cards {{ grid-template-columns:1fr; }} }}
  .card {{ display:flex; flex-direction:column; gap:8px; padding:16px; background:var(--surface);
    border:1px solid var(--line); border-radius:12px; text-decoration:none; color:inherit;
    transition:border-color .15s, transform .15s; min-height:128px; }}
  .card:hover {{ border-color:var(--accent); transform:translateY(-2px); }}
  .ret {{ font-family:var(--mono); font-size:10.5px; letter-spacing:.1em; text-transform:uppercase; color:var(--muted); }}
  .pname {{ font-size:14.5px; font-weight:600; line-height:1.3; flex:1; }}
  .cardfoot {{ display:flex; align-items:center; justify-content:space-between; margin-top:auto; }}
  .price {{ font-family:var(--mono); font-size:13px; color:var(--ink); }}
  .shop {{ font-family:var(--mono); font-size:12px; color:var(--accent); font-weight:600; }}
  footer {{ padding:clamp(40px,6vw,72px) 0 80px; border-top:1px solid var(--line); }}
  .cta {{ font-size:clamp(20px,3vw,28px); font-weight:700; letter-spacing:-.01em; margin:0 0 8px; text-wrap:balance; }}
  .fine {{ font-family:var(--mono); font-size:11.5px; color:var(--muted); letter-spacing:.03em; margin-top:22px; }}
</style>
<div class="wrap">
  <header>
    <p class="eyebrow">Latent Frame · {data['property']}</p>
    <h1>Not dated.<br><em>Full of potential.</em></h1>
    <p class="lede">{data['tagline']}</p>
  </header>
  {body}
  <footer>
    <p class="cta">Love the look? Every room is shoppable.</p>
    <p style="color:var(--muted);max-width:50ch;margin:0">Tap any product to shop the real thing at Sklum or Leroy Merlin — and make the potential yours.</p>
    <p class="fine">Restaged with Latent Frame · honest AI: same rooms, same windows, only the potential added.</p>
  </footer>
</div>"""
    with open(out_path, "w") as f:
        f.write(html)
    print(f"wrote {out_path} ({os.path.getsize(out_path)//1024} KB), {len(data['rooms'])} rooms")


if __name__ == "__main__":
    main()
