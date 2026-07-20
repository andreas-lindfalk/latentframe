"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { AnimatePresence, motion } from "framer-motion";
import { Sparkles } from "lucide-react";
import type { FullSpace } from "@/lib/full";
import { useStyle } from "./style-context";
import { MorphImage } from "./morph-image";
import { BeforeAfter } from "./before-after";
import { Reveal } from "./reveal";

const SIZES = "(max-width: 900px) 100vw, 60vw";

export function SpaceBlock({ space, index }: { space: FullSpace; index: string }) {
  const { style } = useStyle();
  const [variant, setVariant] = useState(0);
  // Reset to the first variant whenever the look changes.
  useEffect(() => setVariant(0), [style]);
  const variants = space.variants?.[style] ?? (space.after?.[style] ? [space.after[style]!] : []);
  const cur = Math.min(variant, Math.max(0, variants.length - 1));
  const afterSrc = variants[cur];
  // Per-style reel when present, else the single style-agnostic clip.
  const reel = space.videos?.[style] ?? space.video;

  return (
    <section className="border-t border-line py-14 sm:py-20">
      <Reveal className="mb-6 grid gap-x-10 gap-y-3 md:grid-cols-[auto_1fr] md:items-end">
        <div className="flex items-baseline gap-4">
          <span className="font-display text-3xl font-light tabular-nums sm:text-4xl" style={{ color: "var(--brand)" }}>
            {index}
          </span>
          <h3 className="font-display text-2xl font-light tracking-tight sm:text-3xl">{space.name}</h3>
        </div>
        <p className="max-w-[56ch] text-[15px] leading-relaxed text-muted md:justify-self-end md:text-right">
          <AnimatePresence mode="wait" initial={false}>
            <motion.span
              key={space.descriptions?.[style] ?? space.potential}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.35 }}
              className="block"
            >
              {space.descriptions?.[style] ?? space.potential}
            </motion.span>
          </AnimatePresence>
        </p>
      </Reveal>

      <Reveal delay={0.05}>
        {reel ? (
          // Hero space with a reel — the moving styled reveal is the wow.
          // Keyed on the URL so the clip reloads when the chosen look changes.
          <figure className="relative aspect-[4/3] w-full overflow-hidden rounded-2xl border border-line bg-canvas">
            <video
              key={reel}
              src={reel}
              autoPlay
              loop
              muted
              playsInline
              className="absolute inset-0 h-full w-full object-cover"
            />
            <span
              className="eyebrow absolute right-4 top-4 rounded-full px-3 py-1.5 text-white"
              style={{ background: "var(--brand)" }}
            >
              The potential · reel
            </span>
          </figure>
        ) : space.context ? (
          // Shared amenity — shown as-is, never restaged or implied private.
          <figure className="relative aspect-[4/3] w-full overflow-hidden rounded-2xl border border-line">
            <Image src={space.before} alt={space.name} fill sizes={SIZES} className="object-cover" />
            <span className="eyebrow absolute left-4 top-4 rounded-full bg-black/55 px-3 py-1.5 text-white/90 backdrop-blur-sm">
              Shared · as it is
            </span>
          </figure>
        ) : space.generated && afterSrc ? (
          // Restyled — before/after reveal, morphing across styles AND variants.
          <div>
            <BeforeAfter
              before={space.before}
              alt={space.name}
              after={<MorphImage srcKey={`${style}-${cur}`} src={afterSrc} alt={`${space.name}, restyled`} sizes={SIZES} />}
            />
            {variants.length > 1 && (
              <div className="mt-3 flex items-center justify-center gap-2.5">
                <span className="eyebrow mr-1 text-muted">The potential</span>
                {variants.map((_, i) => (
                  <button
                    key={i}
                    onClick={() => setVariant(i)}
                    aria-label={`Show variant ${i + 1} of ${variants.length}`}
                    className="h-2 rounded-full transition-all duration-200 cursor-pointer"
                    style={{ width: i === cur ? 22 : 8, background: i === cur ? "var(--brand)" : "var(--line)" }}
                  />
                ))}
                <span className="eyebrow ml-1 tabular-nums text-muted">
                  {cur + 1}/{variants.length}
                </span>
              </div>
            )}
          </div>
        ) : (
          // Selected to restyle, not yet rendered — show the honest "now".
          <figure className="relative aspect-[4/3] w-full overflow-hidden rounded-2xl border border-line">
            <Image src={space.before} alt={space.name} fill sizes={SIZES} className="object-cover" />
            <div className="absolute inset-0 bg-gradient-to-t from-black/55 to-transparent" />
            <span className="eyebrow absolute left-4 top-4 rounded-full bg-black/55 px-3 py-1.5 text-white/90 backdrop-blur-sm">
              Now
            </span>
            <span
              className="eyebrow absolute right-4 top-4 inline-flex items-center gap-1.5 rounded-full px-3 py-1.5 text-white"
              style={{ background: "var(--brand)" }}
            >
              <Sparkles size={12} /> potential pending
            </span>
          </figure>
        )}
      </Reveal>
    </section>
  );
}
