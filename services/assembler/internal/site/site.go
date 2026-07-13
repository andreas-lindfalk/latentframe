// Package site is the ASSEMBLE stage's renderer — pipeline stage 6: turn a property's
// pipeline artifacts (the reveal clip + per-room before/after stills) into the
// self-contained property page that lives at latentframe.ai/[property].
//
// The page is the funnel: the reveal video is the hook, the before/after is the proof,
// and the enquiry form is the catch (lead capture). The "AI visualization" disclosure
// is deliberate and load-bearing — it's the buyer-facing form of re-stage-never-restructure.
package site

import (
	"bytes"
	"html/template"
)

// RoomView is one room's before/after pair on the page.
type RoomView struct {
	Label  string
	Before template.URL // data URI or path
	After  template.URL
}

// Listing is everything the page needs.
type Listing struct {
	Title    string
	Location string
	Price    string
	Blurb    string
	Agent    string
	Beds     int
	Baths    int
	Area     int // m²
	HeroClip template.URL
	Rooms    []RoomView
	// LeadEndpoint is where the enquiry form POSTs. Empty renders a client-only
	// demo (shows the thank-you state without a network call).
	LeadEndpoint string
}

// Render produces the page as a body-only fragment (leading <title> + <style> +
// markup + <script>, no <!doctype>/<html>/<head>/<body> wrapper).
func Render(l Listing) (string, error) {
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, l); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Standalone wraps Render in a minimal HTML document for deployment as a file.
// Metadata (<title>, <style>) auto-routes to the implicit <head>; flow content to
// the implicit <body> — valid and reliably rendered.
func Standalone(l Listing) (string, error) {
	inner, err := Render(l)
	if err != nil {
		return "", err
	}
	return "<!doctype html>\n<html lang=\"en\">\n<meta charset=\"utf-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n" +
		inner + "\n</html>\n", nil
}

var pageTmpl = template.Must(template.New("page").Parse(pageHTML))

const pageHTML = `<title>{{.Title}} — Latent Frame</title>
<style>
:root{
  --bg:#FAF9F6; --surface:#FFFFFF; --sand:#F1EEE7;
  --ink:#191B1A; --ink-soft:#5E635E; --line:#E6E4DD;
  --accent:#1E5C54; --accent-ink:#12433D; --on-accent:#FBFAF7;
  --shadow:0 1px 2px rgba(25,27,26,.05),0 18px 50px rgba(25,27,26,.09);
  --sans:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;
  --serif:"Iowan Old Style","Palatino Linotype",Palatino,Georgia,"Times New Roman",serif;
}
@media (prefers-color-scheme:dark){
  :root{
    --bg:#0F1413; --surface:#161B1A; --sand:#1A211F;
    --ink:#ECEAE3; --ink-soft:#9CA49D; --line:#262E2B;
    --accent:#5FBEAF; --accent-ink:#8FD6C9; --on-accent:#0F1413;
    --shadow:0 1px 2px rgba(0,0,0,.35),0 20px 56px rgba(0,0,0,.42);
  }
}
:root[data-theme="light"]{--bg:#FAF9F6;--surface:#FFFFFF;--sand:#F1EEE7;--ink:#191B1A;--ink-soft:#5E635E;--line:#E6E4DD;--accent:#1E5C54;--accent-ink:#12433D;--on-accent:#FBFAF7;--shadow:0 1px 2px rgba(25,27,26,.05),0 18px 50px rgba(25,27,26,.09);}
:root[data-theme="dark"]{--bg:#0F1413;--surface:#161B1A;--sand:#1A211F;--ink:#ECEAE3;--ink-soft:#9CA49D;--line:#262E2B;--accent:#5FBEAF;--accent-ink:#8FD6C9;--on-accent:#0F1413;--shadow:0 1px 2px rgba(0,0,0,.35),0 20px 56px rgba(0,0,0,.42);}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font-family:var(--sans);line-height:1.6;-webkit-font-smoothing:antialiased;}
img,video{display:block;max-width:100%}
.eyebrow{font-size:12px;letter-spacing:.2em;text-transform:uppercase;font-weight:600;}
.wrap{max-width:1080px;margin:0 auto;padding:0 clamp(18px,4vw,40px);}

/* hero */
.hero{position:relative;height:min(86vh,780px);overflow:hidden;background:#0b0e0d;}
.hero video{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;}
.hero .veil{position:absolute;inset:0;background:linear-gradient(180deg,rgba(10,14,13,.45) 0%,rgba(10,14,13,0) 26%,rgba(10,14,13,0) 52%,rgba(10,14,13,.78) 100%);}
.hero .cnt{position:absolute;left:0;right:0;bottom:0;padding:clamp(22px,5vw,56px) 0;color:#FBFAF7;}
.hero .eyebrow{color:#CFE6E0;margin-bottom:12px;}
.hero h1{font-family:var(--serif);font-weight:600;font-size:clamp(34px,6.4vw,68px);line-height:1.02;margin:0 0 10px;text-wrap:balance;letter-spacing:-.01em;}
.hero .loc{font-size:clamp(15px,2.2vw,19px);opacity:.9;}
.hero .aitag{position:absolute;top:20px;right:clamp(18px,4vw,40px);background:rgba(10,14,13,.55);backdrop-filter:blur(6px);color:#EAF3F0;font-size:11px;letter-spacing:.12em;text-transform:uppercase;padding:7px 12px;border-radius:999px;border:1px solid rgba(255,255,255,.16);}

/* facts */
.facts{border-bottom:1px solid var(--line);}
.facts .row{display:flex;flex-wrap:wrap;gap:clamp(16px,4vw,48px);align-items:baseline;padding:26px 0;}
.facts .price{font-family:var(--serif);font-size:clamp(24px,4vw,34px);font-weight:600;margin-right:auto;}
.fact{display:flex;flex-direction:column;gap:2px;}
.fact .n{font-size:20px;font-weight:700;font-variant-numeric:tabular-nums;}
.fact .l{font-size:11px;letter-spacing:.12em;text-transform:uppercase;color:var(--ink-soft);}

/* sections */
section.block{padding:clamp(44px,8vw,88px) 0;}
.lead-serif{font-family:var(--serif);font-size:clamp(22px,3.2vw,30px);line-height:1.3;max-width:24ch;margin:0;}
.blurb{color:var(--ink-soft);font-size:clamp(16px,2.1vw,18px);max-width:60ch;margin:18px 0 0;}
.shead{display:flex;align-items:baseline;justify-content:space-between;gap:16px;margin-bottom:26px;border-bottom:1px solid var(--line);padding-bottom:14px;}
.shead .eyebrow{color:var(--accent);}
.shead h2{font-family:var(--serif);font-weight:600;font-size:clamp(24px,4vw,38px);margin:0;letter-spacing:-.01em;}

/* before/after */
.ba{position:relative;border-radius:14px;overflow:hidden;border:1px solid var(--line);box-shadow:var(--shadow);user-select:none;touch-action:none;background:#0b0e0d;}
.ba .after img{display:block;width:100%;height:auto;}
.ba .before{position:absolute;inset:0;clip-path:inset(0 calc(100% - var(--pos,52%)) 0 0);}
.ba .before img{display:block;width:100%;height:100%;object-fit:cover;}
.ba .line{position:absolute;top:0;bottom:0;left:var(--pos,52%);width:2px;background:#fff;box-shadow:0 0 0 1px rgba(0,0,0,.2);pointer-events:none;transform:translateX(-1px);}
.ba .line::after{content:"";position:absolute;top:50%;left:50%;width:40px;height:40px;transform:translate(-50%,-50%);border-radius:50%;background:#fff;box-shadow:0 2px 12px rgba(0,0,0,.4);background-image:linear-gradient(90deg,transparent 34%,rgba(25,27,26,.5) 34%,rgba(25,27,26,.5) 40%,transparent 40%,transparent 60%,rgba(25,27,26,.5) 60%,rgba(25,27,26,.5) 66%,transparent 66%);}
.ba input{position:absolute;inset:0;width:100%;height:100%;margin:0;opacity:0;cursor:ew-resize;}
.ba input:focus-visible ~ .line{box-shadow:0 0 0 2px var(--accent);}
.ba .tag{position:absolute;bottom:14px;font-size:11px;letter-spacing:.1em;text-transform:uppercase;color:#fff;background:rgba(10,14,13,.6);backdrop-filter:blur(4px);padding:5px 11px;border-radius:999px;pointer-events:none;}
.ba .tag.l{left:14px;} .ba .tag.r{right:14px;}
.room + .room{margin-top:28px;}
.room h3{font-size:13px;letter-spacing:.12em;text-transform:uppercase;color:var(--ink-soft);margin:0 0 10px;font-weight:600;}
.disclosure{margin-top:18px;font-size:13.5px;color:var(--ink-soft);display:flex;gap:9px;align-items:flex-start;}
.disclosure svg{flex:none;margin-top:2px;}

/* enquiry */
.enquire{background:var(--sand);border-radius:20px;padding:clamp(26px,5vw,52px);display:grid;grid-template-columns:1fr 1fr;gap:clamp(24px,4vw,48px);align-items:start;}
.enquire .pitch h2{font-family:var(--serif);font-weight:600;font-size:clamp(26px,4vw,40px);margin:0 0 12px;letter-spacing:-.01em;}
.enquire .pitch p{color:var(--ink-soft);margin:0;max-width:36ch;}
form{display:flex;flex-direction:column;gap:14px;}
label{display:flex;flex-direction:column;gap:6px;font-size:12px;letter-spacing:.08em;text-transform:uppercase;color:var(--ink-soft);font-weight:600;}
input.f,textarea.f{font:inherit;color:var(--ink);background:var(--surface);border:1px solid var(--line);border-radius:10px;padding:12px 14px;text-transform:none;letter-spacing:normal;}
input.f:focus,textarea.f:focus{outline:2px solid var(--accent);outline-offset:1px;border-color:transparent;}
textarea.f{resize:vertical;min-height:86px;}
button{font:inherit;font-weight:700;cursor:pointer;background:var(--accent);color:var(--on-accent);border:none;border-radius:10px;padding:14px 18px;letter-spacing:.01em;}
button:hover{background:var(--accent-ink);}
.err{display:none;color:#d1544a;font-size:13.5px;margin:2px 0 0;}
.thanks{display:none;background:var(--surface);border:1px solid var(--line);border-radius:12px;padding:22px;}
.thanks h3{margin:0 0 6px;font-family:var(--serif);font-size:22px;}
.thanks p{margin:0;color:var(--ink-soft);}
.enquire.done form{display:none;} .enquire.done .thanks{display:block;}

footer{border-top:1px solid var(--line);padding:34px 0;color:var(--ink-soft);font-size:13px;}
footer .mark{font-family:var(--serif);font-size:18px;color:var(--ink);font-weight:600;}
footer .fine{margin-top:8px;max-width:70ch;}
@media (max-width:720px){ .enquire{grid-template-columns:1fr;} }
@media (prefers-reduced-motion:no-preference){ button,input.f,textarea.f{transition:.15s ease;} }
</style>

  <header class="hero">
    <video src="{{.HeroClip}}" autoplay muted loop playsinline></video>
    <div class="veil"></div>
    <span class="aitag">AI visualization of potential</span>
    <div class="cnt"><div class="wrap">
      <div class="eyebrow">See the potential</div>
      <h1>{{.Title}}</h1>
      <div class="loc">{{.Location}}</div>
    </div></div>
  </header>

  <div class="facts"><div class="wrap"><div class="row">
    <div class="price">{{.Price}}</div>
    <div class="fact"><span class="n">{{.Beds}}</span><span class="l">Bedrooms</span></div>
    <div class="fact"><span class="n">{{.Baths}}</span><span class="l">Bathrooms</span></div>
    <div class="fact"><span class="n">{{.Area}} m²</span><span class="l">Built area</span></div>
  </div></div></div>

  <section class="block"><div class="wrap">
    <p class="lead-serif">This home is sold as it is today. We show you what it could become.</p>
    <p class="blurb">{{.Blurb}}</p>
  </div></section>

  <section class="block" style="padding-top:0"><div class="wrap">
    <div class="shead"><span class="eyebrow">Now → Potential</span><h2>See the potential</h2></div>
    {{range $i, $r := .Rooms}}
    <div class="room">
      <h3>{{$r.Label}}</h3>
      <div class="ba" style="--pos:52%">
        <div class="after"><img src="{{$r.After}}" alt="Re-staged {{$r.Label}}"></div>
        <div class="before"><img src="{{$r.Before}}" alt="Current {{$r.Label}}"></div>
        <div class="line" aria-hidden="true"></div>
        <input type="range" min="0" max="100" value="52" aria-label="Drag to compare current and potential {{$r.Label}}">
        <span class="tag l">Now</span><span class="tag r">Potential</span>
      </div>
    </div>
    {{end}}
    <p class="disclosure">
      <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/><path d="M8 7v4M8 4.6v.2" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
      <span>The “potential” views are AI-generated visualizations. The architecture — walls, windows, room layout — is unchanged; furnishings and finishes are illustrative, to show how the space could be staged.</span>
    </p>
  </div></section>

  <section class="block" style="padding-top:0"><div class="wrap">
    <div class="enquire" id="enquire" data-endpoint="{{.LeadEndpoint}}">
      <div class="pitch">
        <h2>Enquire about this home</h2>
        <p>Buying from abroad? Send your details and {{.Agent}} will be in touch with floor plans, viewing times, and the full renovation picture.</p>
      </div>
      <div>
        <form id="lead">
          <label>Name<input class="f" name="name" required></label>
          <label>Email<input class="f" type="email" name="email" required></label>
          <label>Phone (optional)<input class="f" name="phone"></label>
          <label>Message<textarea class="f" name="message" placeholder="I'm interested in this property…"></textarea></label>
          <button type="submit">Register interest</button>
          <p class="err" id="lead-error" role="alert">Sorry, something went wrong sending your enquiry. Please try again.</p>
        </form>
        <div class="thanks">
          <h3>Thank you.</h3>
          <p>Your enquiry is in. {{.Agent}} will be in touch shortly.</p>
        </div>
      </div>
    </div>
  </div></section>

  <footer><div class="wrap">
    <div class="mark">Latent&nbsp;Frame</div>
    <p class="fine">Interactive property portal · latentframe.ai. Staged views are AI-generated visualizations of the property's potential and are not a representation of the property's current condition.</p>
  </div></footer>

<script>
(function(){
  document.querySelectorAll('.ba').forEach(function(ba){
    var input=ba.querySelector('input');
    input.addEventListener('input',function(){ba.style.setProperty('--pos',input.value+'%');});
  });
  var box=document.getElementById('enquire');
  var form=document.getElementById('lead');
  var err=document.getElementById('lead-error');
  form.addEventListener('submit',function(e){
    e.preventDefault();
    var data={};new FormData(form).forEach(function(v,k){data[k]=v;});
    var ep=box.getAttribute('data-endpoint');
    function done(){box.classList.add('done');}
    function fail(){ if(err){ err.style.display='block'; } }
    if(ep){
      if(err){ err.style.display='none'; }
      fetch(ep,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)})
        .then(function(r){ r.ok ? done() : fail(); })
        .catch(fail);
    }else{ done(); }
  });
})();
</script>`
