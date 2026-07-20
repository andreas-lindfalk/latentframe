# Page Override — Property Showcase (`web/showcase`)

> Overrides `../MASTER.md` for the Zeniamar three-style showcase page.
> Rationale is recorded here so the deviations from the generated Master are a
> documented decision, not drift.

## Why this deviates from Master

The generated Master proposed a generic **teal + professional-blue "trust"** palette
and a **skeuomorphic** style. Two deliberate changes:

1. **Palette → warm-charcoal editorial "gallery", not teal/blue.**
   The hero content is warm restaged photography (Mediterranean clay/cream/sand)
   plus a cool Scandinavian and a blue Coastal look. A fixed teal chrome fights all
   three. The UI chrome is a warm near-neutral so the *photography and each style's
   own palette lead*.

2. **Accent is STYLE-AWARE, not a single hue.** The page accent morphs with the
   selected decoration style — this both escapes the single-palette look and *is*
   the "one home, three visions" concept:
   - Mediterranean → clay/ochre `#BC6437`
   - Scandinavian → sage-stone `#6E7A5C`
   - Coastal → marine `#356381`

3. **Style → refined editorial, not heavy skeuomorphism.** Master itself flags
   skeuomorphism as poor for performance/readability; here the photos are the
   richness. Clean framing + tasteful depth, photography as the light source.

We also actively avoid the common AI-generated "warm cream + serif display +
terracotta accent" cliché: the ground is warm-charcoal (not cream), the accent is
style-driven (not a default terracotta), layout is left-aligned editorial (not
centered), icons are SVG (Lucide, no emoji).

## Tokens (implemented in `app/globals.css`)

| Role | Light | Dark |
|------|-------|------|
| canvas (page) | `#EEEAE2` | `#14110C` |
| surface | `#F7F4EE` | `#1C1811` |
| surface2 | `#FFFFFF` | `#241F17` |
| ink (text) | `#1B1710` | `#ECE5D8` |
| muted | `#6B6153` | `#9C9182` |
| line | `#DED6C8` | `#2E2820` |
| brand (accent) | style-aware (see above) | style-aware |

Both themes designed; default follows `prefers-color-scheme`, toggle stamps
`data-theme`. `--brand` is registered via `@property` so the accent glides on
style change.

## Typography

- **Display:** Fraunces (editorial, high-contrast; italic for accent words)
- **UI/body:** Inter Tight
- **Labels/data:** Geist Mono

## Kept from Master

- Pattern: **Immersive / Interactive** (full-screen hero → guided room tour →
  benefit reveal → CTA).
- Motion tier **Complex** — shared-element morph (implemented as Framer Motion
  crossfade rather than GSAP Flip), scroll reveals, before/after sliders.
- The full pre-delivery checklist (SVG icons, cursor-pointer, 150–300ms hovers,
  4.5:1 contrast, visible focus, `prefers-reduced-motion`, responsive 375–1440).
