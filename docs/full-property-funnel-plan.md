# Full-property funnel — the basic slice (MVP)

The pixels are a commodity (Nano-Banana does the restage for ~$0.08). The business — and the
moat — is the **honest, shoppable, full-property funnel** on top. This phase builds the
thinnest end-to-end slice to test the riskiest *business* assumption before we invest in
scale or automation.

## The thesis to test
**Will real people engage with "renovation potential" content and convert** — click to shop
the look (affiliate) or raise their hand as a lead? We've proven we *can* make the content;
we have *not* proven anyone wants it or acts on it. Test that cheaply, with one property.

## The end-to-end slice
```
one listing's photos
  → restage the WHOLE property (Nano-Banana + inspire gate + best-of-N)   [mostly have]
  → honest before/after per room (+ optional i2v tour)                    [have]
  → SHOPPABLE product cards per room (real items, affiliate links)        [build — the moat]
  → a property "potential" PAGE (before/after + narrative + shop + CTA)   [build]
  → LEAD capture / shop click                                             [build]
  → MEASURE (views, card clicks, leads)                                   [build]
```

## Components & MVP scope
1. **Ingest** — one real listing's photos in a folder (manual for now). *Have the inputs.*
2. **Whole-property restage** — generalise `director beststage` / `coldrun` to a listing:
   classify each photo's room type (UNDERSTAND/Claude, or manual tags for MVP) → per-room
   prompt → best-of-N + inspire gate. *Mostly have; needs the per-listing orchestration.*
3. **Shoppable layer (the moat).** MVP: hand-curate 3–5 real Sklum/Leroy-Merlin products per
   room (image, price, affiliate link) that match the render → "shop this room" cards. This
   tests the *demand* without building auto-detection. **Defensible growth (not MVP):**
   reference-conditioned **product placement** — feed the product images to Nano-Banana (it
   takes reference images) so the render literally *features the buyable items*, making the
   whole picture shoppable by construction.
4. **Property page** — one generated page: hero, room-by-room before/after, optional tour,
   a short "here's the potential" narrative, the shoppable cards, one lead CTA. We already
   generate before/after galleries — productise that into a page template (Go-served in the
   monorepo `web/`, or a static deploy).
5. **Lead / CTA** — pick ONE conversion to test first (see open questions). Affiliate "shop
   the look" clicks are self-serve revenue; a single lead form ("make this real — get a
   renovation quote" or "book a viewing") captures intent. Store + count.
6. **Measure** — page views, shop-card clicks, lead submits. Put it in front of a small real
   audience (a few agents, a targeted post, or a tiny ad) and watch.

## Smallest testable version
**One real property → one honest shoppable potential page → in front of N real people →
measure.** Signal (clicks/leads) → build ingest + scale + auto-shoppable. No signal → we
learned it cheaply and pivot the offer, not the tech.

## Have vs build
- **Have:** the restage engine + inspire gate + best-of-N; before/after gallery generation;
  the i2v tour; UNDERSTAND (room classification + briefs).
- **Build:** per-listing orchestration + room classification; the shoppable catalog + cards;
  the property page (productised, hosted); lead capture + basic analytics.

## Defensible tech to grow into (later, not MVP)
Reference-conditioned product placement (shoppable-by-construction) · auto item→catalog→
affiliate mapping · portal ingest at scale · the honesty gate as the trust brand ·
Whisper/UNDERSTAND owner-intent ("lose the bidet, open this wall").

## Open questions (need Andreas)
1. **Which property** to test with — a cold Idealista listing, or Zeniamar?
2. **Which conversion first** — affiliate "shop the look", a buyer "book a viewing" lead, or
   a renovation "get a quote" lead?
3. **Affiliate access** — do we have Sklum / Leroy-Merlin affiliate links, or curate manually
   for the test?
4. **Test audience** — agents you know, a targeted post, a small ad?
5. **Hosting** — Go-served page in the monorepo, or a quick static deploy?

## Rough sequence
1. Whole-property restage of one real listing (room-classify → gate → collect).
2. Property-page template (before/after + narrative + CTA).
3. Curated shoppable cards + one CTA/lead form.
4. Ship it live for a small audience; measure.
5. Decide what to scale/automate based on the signal.
