"use client";

import { useState } from "react";
import Image from "next/image";
import { MoveHorizontal } from "lucide-react";

// Before/after reveal. The "after" (the potential) is the base layer; the "before"
// (now) is clipped over the left `pos%`. Driven by a transparent range input so it
// works with pointer, touch and keyboard out of the box.
export function BeforeAfter({
  before,
  after,
  alt,
}: {
  before: string;
  after: React.ReactNode;
  alt: string;
}) {
  const [pos, setPos] = useState(48);

  return (
    <div className="group relative aspect-[4/3] w-full select-none overflow-hidden rounded-2xl border border-line bg-canvas shadow-[0_24px_60px_-30px_rgba(0,0,0,0.5)]">
      {/* AFTER — base layer (crossfades between styles) */}
      {after}

      {/* BEFORE — clipped to the left portion */}
      <div className="absolute inset-0" style={{ clipPath: `inset(0 ${100 - pos}% 0 0)` }}>
        <Image src={before} alt={`${alt} — before`} fill sizes="(max-width: 900px) 100vw, 55vw" className="object-cover" />
        <div className="absolute inset-0 bg-gradient-to-r from-black/20 to-transparent" />
      </div>

      {/* Labels */}
      <span className="eyebrow absolute left-4 top-4 rounded-full bg-black/55 px-3 py-1.5 text-white/90 backdrop-blur-sm">
        Now
      </span>
      <span className="eyebrow absolute right-4 top-4 rounded-full px-3 py-1.5 text-white backdrop-blur-sm" style={{ background: "var(--brand)" }}>
        The potential
      </span>

      {/* Divider + handle */}
      <div className="pointer-events-none absolute inset-y-0" style={{ left: `${pos}%` }}>
        <div className="absolute inset-y-0 w-px -translate-x-1/2 bg-white/80 shadow-[0_0_12px_rgba(0,0,0,0.4)]" />
        <div className="absolute top-1/2 grid h-11 w-11 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full bg-white text-neutral-900 shadow-xl ring-1 ring-black/5 transition-transform duration-200 group-hover:scale-105">
          <MoveHorizontal size={18} strokeWidth={2.2} />
        </div>
      </div>

      {/* Interaction surface: full-bleed transparent range for drag + a11y */}
      <input
        type="range"
        min={0}
        max={100}
        value={pos}
        onChange={(e) => setPos(Number(e.target.value))}
        aria-label={`${alt}: drag to compare now and the potential`}
        className="absolute inset-0 h-full w-full cursor-ew-resize opacity-0"
      />
    </div>
  );
}
