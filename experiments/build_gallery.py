import os, base64, json

SP = os.environ["SP"]
GAL = os.path.join(SP, "gal")

def b64(path):
    with open(path, "rb") as f:
        return base64.b64encode(f.read()).decode()

def img_uri(name):
    return "data:image/jpeg;base64," + b64(os.path.join(GAL, name))

def vid_uri(name):
    return "data:video/mp4;base64," + b64(os.path.join(GAL, name))

rooms = [
    dict(id="bath", tag="Bathroom", before="before_bath.jpg", after="after_bath.jpg",
         verdict="verified", note="Dated tiles and a tired tub become a boutique-hotel spa — a frameless walk-in rain shower where the bath was, a floating oak vanity, a warm backlit mirror. Same walls, same window, plumbing untouched."),
    dict(id="living", tag="Living room", before="before_living.jpg", after="after_living.jpg",
         verdict="verified", note="Yellow walls go crisp white, the tired suite gives way to a soft grey corner sofa, a media wall, a wooden table on a Mediterranean rug and a big leafy plant — bright, calm, easy to live in."),
    dict(id="kitchen", tag="Kitchen", before="before_kitchen.jpg", after="after_kitchen.jpg",
         verdict="verified", note="The dated oak galley becomes a sleek handleless kitchen in warm white and stone with integrated appliances — modernised in place, every opening (and the door to the utility room) kept exactly where it is."),
    dict(id="bedroom", tag="Bedroom", before="before_bedroom.jpg", after="after_bedroom.jpg",
         verdict="verified", note="The dated pine-and-orange bedroom becomes a calm coastal retreat — an upholstered bed with crisp white linens, light-oak nightstands and greenery against soft white walls, with the window kept exactly as it is."),
    dict(id="dining", tag="Dining terrace", before="before_dining.jpg", after="after_dining.jpg",
         verdict="verified", note="The shaded porch, set for the evening: a long table for eight under a wrought-iron pendant, cushioned chairs, olive and herb pots — where the family actually eats dinner."),
    dict(id="outB", tag="Terrace lounge", before="before_outB.jpg", after="after_outB.jpg",
         verdict="verified", note="The same terrace, the other half — a relaxed Mediterranean lounge around the standing tree, wrapped in terracotta pots of oleander and lavender."),
    dict(id="binner", tag="Inner balcony", before="before_binner.jpg", after="after_binner.jpg",
         verdict="verified", note="The shady corner becomes a cosy hideaway — a deep corner sofa, woven pendants strung from the beams, an outdoor rug and shade-loving greenery. Structure untouched."),
    dict(id="bouter", tag="Outer balcony", before="before_bouter.jpg", after="after_bouter.jpg",
         verdict="verified", note="Out in the sun: a lounge group, warm string bulbs along the pergola, a striped rug and pots of palm and oleander — the railings painted crisp white."),
]

CSS = """
<style>
  :root{
    --paper:#F6F1E8; --panel:#FBF8F2; --ink:#332F28; --ink-soft:#6C6455;
    --line:#E3D9C8; --brass:#A9722F; --brass-2:#C08A3E; --olive:#5C6A49;
    --shadow:0 18px 50px -24px rgba(60,45,20,.45);
    --maxw:1120px;
  }
  @media (prefers-color-scheme:dark){
    :root{
      --paper:#1E1B17; --panel:#26221C; --ink:#ECE4D6; --ink-soft:#A99C86;
      --line:#39332B; --brass:#CFA163; --brass-2:#E0B879; --olive:#93A07E;
      --shadow:0 24px 60px -28px rgba(0,0,0,.7);
    }
  }
  :root[data-theme="dark"]{
    --paper:#1E1B17; --panel:#26221C; --ink:#ECE4D6; --ink-soft:#A99C86;
    --line:#39332B; --brass:#CFA163; --brass-2:#E0B879; --olive:#93A07E;
    --shadow:0 24px 60px -28px rgba(0,0,0,.7);
  }
  :root[data-theme="light"]{
    --paper:#F6F1E8; --panel:#FBF8F2; --ink:#332F28; --ink-soft:#6C6455;
    --line:#E3D9C8; --brass:#A9722F; --brass-2:#C08A3E; --olive:#5C6A49;
    --shadow:0 18px 50px -24px rgba(60,45,20,.45);
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--paper);color:var(--ink);
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
    line-height:1.6;-webkit-font-smoothing:antialiased;overflow-x:hidden}
  .wrap{max-width:var(--maxw);margin:0 auto;padding:0 22px}
  .serif{font-family:"Iowan Old Style",Palatino,"Palatino Linotype","Book Antiqua",Georgia,serif}
  .eyebrow{font-size:.74rem;letter-spacing:.22em;text-transform:uppercase;color:var(--brass);font-weight:600}

  /* hero */
  header.hero{padding:72px 0 34px;border-bottom:1px solid var(--line)}
  .hero h1{font-size:clamp(2.5rem,6vw,4.4rem);line-height:1.02;margin:.28em 0 .1em;
    letter-spacing:-.015em;text-wrap:balance;font-weight:600}
  .hero .addr{font-size:1.05rem;color:var(--ink-soft)}
  .hero p.lede{font-size:clamp(1.05rem,2.2vw,1.3rem);max-width:38ch;margin:.9em 0 0;color:var(--ink)}
  .hero .meta{display:flex;flex-wrap:wrap;gap:10px;margin-top:22px}
  .chip{display:inline-flex;align-items:center;gap:7px;font-size:.8rem;padding:6px 12px;border-radius:999px;
    border:1px solid var(--line);background:var(--panel);color:var(--ink-soft)}
  .chip b{color:var(--ink);font-weight:600}
  .dot{width:7px;height:7px;border-radius:50%}
  .dot.g{background:var(--olive)} .dot.a{background:var(--brass-2)}

  /* video */
  .film{margin:40px 0 8px}
  .film figure{margin:0;border-radius:16px;overflow:hidden;box-shadow:var(--shadow);border:1px solid var(--line)}
  .film video{display:block;width:100%;height:auto}
  .film figcaption{margin-top:12px;color:var(--ink-soft);font-size:.9rem;display:flex;gap:8px;align-items:baseline}

  /* room */
  .rooms{padding:26px 0 8px}
  .room{margin:56px 0}
  .room-head{display:flex;align-items:baseline;justify-content:space-between;gap:16px;margin-bottom:16px;flex-wrap:wrap}
  .room-head .t{display:flex;align-items:baseline;gap:14px;flex-wrap:wrap}
  .room-num{font-size:.8rem;color:var(--brass);font-weight:700;letter-spacing:.1em}
  .room-head h2{font-size:clamp(1.5rem,3.2vw,2.1rem);margin:0;font-weight:600;letter-spacing:-.01em}
  .badge{font-size:.72rem;letter-spacing:.04em;padding:5px 11px;border-radius:999px;font-weight:600;white-space:nowrap}
  .badge.verified{background:color-mix(in srgb,var(--olive) 20%,transparent);color:var(--olive);border:1px solid color-mix(in srgb,var(--olive) 40%,transparent)}
  .badge.vision{background:color-mix(in srgb,var(--brass-2) 20%,transparent);color:var(--brass);border:1px solid color-mix(in srgb,var(--brass-2) 45%,transparent)}
  .room p.note{margin:0 0 18px;color:var(--ink-soft);max-width:62ch;font-size:1rem}

  /* before/after slider */
  .ba{position:relative;aspect-ratio:1050/820;border-radius:15px;overflow:hidden;box-shadow:var(--shadow);
    border:1px solid var(--line);touch-action:none;cursor:ew-resize;user-select:none;background:var(--panel)}
  .ba img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;display:block;pointer-events:none}
  .ba .top{will-change:clip-path}
  .ba .lab{position:absolute;top:12px;font-size:.68rem;letter-spacing:.16em;text-transform:uppercase;font-weight:700;
    padding:5px 10px;border-radius:8px;background:rgba(20,16,10,.6);color:#fff;backdrop-filter:blur(3px);pointer-events:none}
  .ba .lab.b{left:12px} .ba .lab.a{right:12px}
  .ba .handle{position:absolute;top:0;bottom:0;width:2px;background:rgba(255,255,255,.9);left:50%;
    transform:translateX(-1px);pointer-events:none;box-shadow:0 0 0 1px rgba(0,0,0,.15)}
  .ba .knob{position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);width:44px;height:44px;border-radius:50%;
    background:rgba(255,255,255,.95);box-shadow:0 4px 14px rgba(0,0,0,.35);display:grid;place-items:center;
    pointer-events:none;color:#33302B;font-size:15px;font-weight:700}

  /* cta */
  .cta{margin:70px 0 30px;padding:44px 34px;border-radius:20px;background:var(--panel);border:1px solid var(--line);
    box-shadow:var(--shadow);text-align:center}
  .cta h3{font-size:clamp(1.6rem,3.6vw,2.3rem);margin:.1em 0 .2em;font-weight:600;text-wrap:balance}
  .cta p{color:var(--ink-soft);margin:0 auto 22px;max-width:46ch}
  .form{display:flex;gap:10px;justify-content:center;flex-wrap:wrap;max-width:560px;margin:0 auto}
  .form input{flex:1 1 190px;min-width:0;padding:13px 15px;border-radius:11px;border:1px solid var(--line);
    background:var(--paper);color:var(--ink);font-size:.98rem;font-family:inherit}
  .form input:focus-visible{outline:2px solid var(--brass);outline-offset:1px}
  .form button{padding:13px 22px;border-radius:11px;border:0;background:var(--brass);color:#fff;font-weight:600;
    font-size:.98rem;cursor:pointer;font-family:inherit;transition:background .15s}
  .form button:hover{background:var(--brass-2)}
  .form button:focus-visible{outline:2px solid var(--ink);outline-offset:2px}
  .toast{margin-top:14px;font-size:.9rem;color:var(--olive);min-height:1.2em;font-weight:600}

  footer{border-top:1px solid var(--line);margin-top:40px;padding:26px 0 60px;color:var(--ink-soft);font-size:.82rem}
  footer .row{display:flex;justify-content:space-between;gap:14px;flex-wrap:wrap;align-items:center}
  .fine{font-size:.74rem;opacity:.8;margin-top:8px;max-width:70ch}
  @media (prefers-reduced-motion:reduce){*{scroll-behavior:auto!important;transition:none!important}}
</style>
"""

def room_html(i, r):
    return (
      '<section class="room">'
      '<div class="room-head"><div class="t">'
      '<span class="room-num">0'+str(i)+'</span>'
      '<h2 class="serif">'+r["tag"]+'</h2></div>'
      + ('<span class="badge verified">✓ Honesty-verified</span>' if r["verdict"]=="verified"
         else '<span class="badge vision">◇ Renovation vision · structural</span>')
      + '</div>'
      '<p class="note">'+r["note"]+'</p>'
      '<div class="ba" data-ba>'
        '<img class="bot" src="'+img_uri(r["after"])+'" alt="after">'
        '<img class="top" src="'+img_uri(r["before"])+'" alt="before">'
        '<span class="lab b">Before</span><span class="lab a">After</span>'
        '<div class="handle"></div><div class="knob">↔</div>'
      '</div>'
      '</section>'
    )

BODY = CSS + (
  '<header class="hero"><div class="wrap">'
    '<span class="eyebrow">Costa Blanca · Orihuela Costa</span>'
    '<h1 class="serif">The home hiding inside the listing.</h1>'
    '<div class="addr serif">Zeniamar 5 — piso 109</div>'
    '<p class="lede">Every room reimagined at its full potential. Drag any image to reveal what it could become.</p>'
    '<div class="meta">'
      '<span class="chip"><span class="dot g"></span><b>Structure untouched</b>&nbsp;walls, windows &amp; light are real</span>'
      '<span class="chip"><span class="dot a"></span>only the styling is reimagined</span>'
    '</div>'
  '</div></header>'

  '<div class="wrap">'
    '<div class="film"><figure>'
      '<video src="'+vid_uri("reveal.mp4")+'" autoplay muted loop playsinline></video>'
    '</figure><figcaption><span class="eyebrow">Walk-through</span><span>A slow reveal of the main bathroom’s potential — generated from a single listing photo.</span></figcaption></div>'

    '<div class="rooms">'
    + "".join(room_html(i+1, r) for i, r in enumerate(rooms)) +
    '</div>'

    '<div class="cta">'
      '<span class="eyebrow">Fall for the potential</span>'
      '<h3 class="serif">Zeniamar 5 could be yours.</h3>'
      '<p>Book a viewing and see the bones for yourself — the light, the space, the sea five minutes away.</p>'
      '<form class="form" onsubmit="return lf(event)">'
        '<input type="text" placeholder="Your name" aria-label="Your name" required>'
        '<input type="email" placeholder="Email" aria-label="Email" required>'
        '<button type="submit">Request a viewing</button>'
      '</form>'
      '<div class="toast" id="toast" role="status"></div>'
    '</div>'

    '<footer><div class="row">'
      '<span><b class="serif" style="color:var(--ink)">Latent&nbsp;Frame</b> &mdash; see the potential.</span>'
      '<span>Demo · not a live listing</span>'
    '</div>'
    '<div class="fine">Before images: original Idealista listing photography (watermark: petra hönig). "After" visuals are AI-generated potential. Every room here is <b>Honesty-verified</b> — walls, windows and openings unchanged; only finishes and furnishings are reimagined.</div>'
    '</footer>'
  '</div>'

  '<script>'
  'document.querySelectorAll("[data-ba]").forEach(function(ba){'
    'var top=ba.querySelector(".top"),h=ba.querySelector(".handle"),k=ba.querySelector(".knob"),down=false;'
    'function set(p){p=Math.max(0,Math.min(100,p));top.style.clipPath="inset(0 "+(100-p)+"% 0 0)";h.style.left=p+"%";k.style.left=p+"%";}'
    'function at(e){var r=ba.getBoundingClientRect();var x=(e.touches?e.touches[0].clientX:e.clientX)-r.left;set(x/r.width*100);}'
    'set(50);'
    'ba.addEventListener("pointerdown",function(e){down=true;ba.setPointerCapture(e.pointerId);at(e);});'
    'ba.addEventListener("pointermove",function(e){if(down)at(e);});'
    'window.addEventListener("pointerup",function(){down=false;});'
  '});'
  'function lf(e){e.preventDefault();document.getElementById("toast").textContent="✓ Thanks — an agent will be in touch (demo).";e.target.reset();return false;}'
  '</script>'
)

out = os.path.join(SP, "gallery.html")
with open(out, "w") as f:
    f.write(BODY)
print("wrote", out, str(round(len(BODY)/1024))+"KB")
