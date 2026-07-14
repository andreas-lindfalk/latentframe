import os, json, base64, urllib.request

SP = os.environ["SP"]; KEY = os.environ["GEMINI_API_KEY"]
SRC = os.environ["SRC"]; OUT = os.environ["OUT"]; REFS = os.environ["REFS"].split(",")
ROOM = os.environ.get("ROOM", "room")

def part_img(path):
    d = open(path, "rb").read()
    mime = "image/png" if path.lower().endswith(".png") else "image/jpeg"
    return {"inline_data": {"mime_type": mime, "data": base64.b64encode(d).decode()}}

INSTR = os.environ.get("INSTR") or ("IMAGE 1 is a REAL " + ROOM + " to re-stage; the remaining images are STYLE REFERENCES. "
  "Re-stage IMAGE 1 to show its full potential, keeping its ARCHITECTURE EXACTLY — same walls, windows, doors, "
  "openings, ceiling, proportions and camera viewpoint. Do NOT borrow any architecture, layout or window/door "
  "positions from the reference images; those are ONLY for style. Remove the dated furniture and refurnish the room "
  "adopting the AESTHETIC of the style references: warm Spanish-Mediterranean / boutique-riad — limewash warm-white "
  "walls, terracotta or warm-stone floor, warm oak and reclaimed wood, rattan and cane, natural stone, handmade/zellige "
  "tile, aged brass, earthenware pottery, slipcovered linen/cream seating, jute and textural rugs, abundant olive trees "
  "and greenery, warm golden sun-washed light. Cozy, relaxed, tactile, unpretentious luxury. Keep every window and door "
  "clear and honest; if sky shows through a window, make it vivid blue. Photorealistic, editorial, aspirational.")

parts = [{"text": "IMAGE 1 (the real room to re-stage — keep its architecture):"}, part_img(SRC)]
for i, r in enumerate(REFS):
    parts.append({"text": "Style reference " + str(i + 1) + " (style only, not architecture):"})
    parts.append(part_img(r.strip()))
parts.append({"text": INSTR})

url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent?key=" + KEY
body = json.dumps({"contents": [{"parts": parts}]}).encode()
req = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"})
r = json.load(urllib.request.urlopen(req, timeout=180))
for p in r["candidates"][0]["content"]["parts"]:
    d = p.get("inlineData") or p.get("inline_data")
    if d:
        open(OUT, "wb").write(base64.b64decode(d["data"])); print("wrote", OUT); break
else:
    print("NO IMAGE:", json.dumps(r)[:700])
