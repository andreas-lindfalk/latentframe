// Multi-property registry. Each property is a self-contained manifest under data/,
// with its renders namespaced under public/renders/<slug>/. Add a property by dropping
// its manifest in data/ and adding one line here.
import { heroSpace, type FullManifest } from "./full";
import zeniamar from "@/data/zeniamar-v.full.json";
import villa from "@/data/villa.full.json";

export const PROPERTIES: Record<string, FullManifest> = {
  "zeniamar-v": zeniamar as unknown as FullManifest,
  villa: villa as unknown as FullManifest,
};

export const PROPERTY_SLUGS = Object.keys(PROPERTIES);

/** Card data for the landing picker — property meta + a representative hero image. */
export function propertyCards() {
  return PROPERTY_SLUGS.map((slug) => {
    const m = PROPERTIES[slug];
    const hero = heroSpace(m);
    // Prefer a styled "after"; fall back to the honest "before".
    const img = hero.after?.mediterranean ?? hero.after?.coastal ?? hero.after?.scandinavian ?? hero.before;
    return { slug, property: m.property, spaceCount: m.spaces.length, img };
  });
}
