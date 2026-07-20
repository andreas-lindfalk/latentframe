// The showcase is data-driven: this module loads the property manifest
// (data/property.json) and exposes it typed. The manifest is emitted by the
// `render showcase` pipeline command (nano-banana restages, honesty-gated) —
// swap the JSON to render a different property; no component changes needed.

import manifest from "@/data/property.json";

export type StyleId = "mediterranean" | "scandinavian" | "coastal";

export interface Style {
  id: StyleId;
  name: string;
  tagline: string;
  /** Accent hex, mirrored in globals.css [data-style]; kept here for JS-side use. */
  accent: string;
}

export interface Product {
  name: string;
  retailer: string;
  price: string;
  url: string;
}

export interface Room {
  id: string;
  name: string;
  /** Tour position — the walkthrough order, not a ranking. */
  index: string;
  blurb: string;
  before: string;
  after: Record<StyleId, string>;
  /** Curated shop-the-look list, per style. Only styles with a list are shown. */
  shop: Partial<Record<StyleId, Product[]>>;
}

export interface PropertyMeta {
  name: string;
  location: string;
  kicker: string;
  lede: string;
}

interface Manifest {
  version: number;
  property: PropertyMeta;
  styles: Style[];
  rooms: Room[];
}

const data = manifest as unknown as Manifest;

export const PROPERTY: PropertyMeta = data.property;
export const STYLES: Style[] = data.styles;
export const ROOMS: Room[] = data.rooms;
