"use client";

import Image from "next/image";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";

// Crossfades between images as `srcKey` changes — the "shared-element morph" from
// the design system, done as an opacity dissolve (robust in React vs. GSAP Flip).
export function MorphImage({
  srcKey,
  src,
  alt,
  sizes,
  priority,
}: {
  srcKey: string;
  src: string;
  alt: string;
  sizes: string;
  priority?: boolean;
}) {
  const reduce = useReducedMotion();
  return (
    <div className="absolute inset-0 overflow-hidden">
      <AnimatePresence initial={false} mode="sync">
        <motion.div
          key={srcKey}
          className="absolute inset-0"
          initial={{ opacity: reduce ? 1 : 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: reduce ? 0 : 0.7, ease: [0.16, 1, 0.3, 1] }}
        >
          <Image src={src} alt={alt} fill sizes={sizes} priority={priority} className="object-cover" />
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
