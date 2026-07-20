// Shared types + helpers for the grouped v2 property manifest emitted by
// `render showcase` — each property is loaded per-slug via lib/properties.ts.

import type { StyleId } from "./property";

export interface FullSpace {
  id: string;
  name: string;
  category: "interior" | "outdoor_private" | "shared";
  showcase: "hero" | "strong" | "supporting";
  potential: string;
  before: string;
  after?: Partial<Record<StyleId, string>>; // primary variant per style
  variants?: Partial<Record<StyleId, string[]>>; // all variants per style
  descriptions?: Partial<Record<StyleId, string>>; // per-style caption
  generated: boolean; // true once the styled afters actually exist
  context: boolean; // shared amenity — shown as-is
  video?: string; // single style-agnostic reel (fallback)
  videos?: Partial<Record<StyleId, string>>; // per-style reel (preferred)
}

export interface FullProperty {
  name: string;
  location: string;
  type: string;
  sleeps_estimate: number;
}

export interface FullManifest {
  version: number;
  property: FullProperty;
  styles: { id: StyleId; name: string; tagline: string; accent: string }[];
  spaces: FullSpace[];
}

export const GROUPS: { key: FullSpace["category"]; label: string; sub: string }[] = [
  { key: "interior", label: "Inside", sub: "Room by room" },
  { key: "outdoor_private", label: "Outside", sub: "Terraces, balcony & solarium" },
  { key: "shared", label: "The setting", sub: "The community" },
];

/** The hero space — prefer a standout outdoor space, else any hero, else the first. */
export function heroSpace(m: FullManifest): FullSpace {
  return (
    m.spaces.find((s) => s.category === "outdoor_private" && s.showcase === "hero") ??
    m.spaces.find((s) => s.showcase === "hero") ??
    m.spaces[0]
  );
}
