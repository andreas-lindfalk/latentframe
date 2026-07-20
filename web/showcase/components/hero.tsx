"use client";

import { AnimatePresence, motion, useReducedMotion, type Variants } from "framer-motion";
import { ChevronDown, Camera } from "lucide-react";
import { PROPERTY, ROOMS, STYLES } from "@/lib/property";
import { useStyle } from "./style-context";
import { MorphImage } from "./morph-image";
import { ThemeToggle } from "./theme-toggle";

const living = ROOMS[0];

// Headline split into words for a kinetic entrance; `br` forces a line break.
const WORDS: { t?: string; em?: boolean; br?: boolean }[] = [
  { t: "One" },
  { t: "home," },
  { br: true },
  { t: "three", em: true },
  { t: "visions.", em: true },
];

const container: Variants = { hidden: {}, show: { transition: { staggerChildren: 0.09, delayChildren: 0.15 } } };
const word: Variants = {
  hidden: { opacity: 0, y: "0.5em" },
  show: { opacity: 1, y: 0, transition: { duration: 0.7, ease: [0.16, 1, 0.3, 1] } },
};

export function Hero() {
  const { style, locked } = useStyle();
  const reduce = useReducedMotion();
  const styleName = STYLES.find((s) => s.id === style)?.name ?? "";

  return (
    <header className="relative flex min-h-[100svh] flex-col overflow-hidden">
      {/* Auto-touring hero photograph (living room, cross-dissolving through styles) */}
      <div className="absolute inset-0">
        <motion.div
          className="absolute inset-0"
          animate={reduce ? {} : { scale: [1, 1.08] }}
          transition={{ duration: 18, ease: "linear", repeat: Infinity, repeatType: "reverse" }}
        >
          <MorphImage
            srcKey={style}
            src={living.after[style]}
            alt={`${PROPERTY.name} living room, ${styleName} styling`}
            sizes="100vw"
            priority
          />
        </motion.div>
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/25 to-black/45" />
        <div className="absolute inset-0 bg-gradient-to-r from-black/60 to-transparent" />
      </div>

      {/* Top bar */}
      <div className="relative z-10 mx-auto flex w-full max-w-6xl items-center justify-between px-5 py-5 sm:px-8">
        <span className="eyebrow text-white/90">Latent&nbsp;Frame</span>
        <ThemeToggle />
      </div>

      {/* Headline block */}
      <div className="relative z-10 mx-auto mt-auto w-full max-w-6xl px-5 pb-16 sm:px-8 sm:pb-20">
        <motion.p
          initial={reduce ? false : { opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: [0.16, 1, 0.3, 1] }}
          className="eyebrow mb-5 flex flex-wrap items-center gap-x-3 gap-y-1 text-white/85"
        >
          <span style={{ color: "var(--brand)" }}>{PROPERTY.kicker}</span>
          <span className="text-white/40">/</span>
          <span>{PROPERTY.location}</span>
        </motion.p>

        {reduce ? (
          <h1 className="max-w-[15ch] font-display text-[clamp(3rem,9vw,7rem)] font-light leading-[0.92] tracking-[-0.03em] text-white">
            One home,
            <br />
            <em className="font-medium italic" style={{ color: "var(--brand)" }}>
              three visions.
            </em>
          </h1>
        ) : (
          <motion.h1
            variants={container}
            initial="hidden"
            animate="show"
            className="flex max-w-[15ch] flex-wrap font-display text-[clamp(3rem,9vw,7rem)] font-light leading-[0.92] tracking-[-0.03em] text-white"
          >
            {WORDS.map((w, i) =>
              w.br ? (
                <span key={i} className="basis-full" aria-hidden />
              ) : (
                <motion.span
                  key={i}
                  variants={word}
                  className={`inline-block ${w.em ? "font-medium italic" : "font-light"}`}
                  style={w.em ? { color: "var(--brand)" } : undefined}
                >
                  {w.t}&nbsp;
                </motion.span>
              ),
            )}
          </motion.h1>
        )}

        <motion.p
          initial={reduce ? false : { opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.55 }}
          className="mt-6 max-w-[52ch] text-lg leading-relaxed text-white/80 text-balance"
        >
          {PROPERTY.lede}
        </motion.p>

        <motion.div
          initial={reduce ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1, delay: 0.8 }}
          className="mt-9 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-6"
        >
          <span className="flex items-start gap-2 text-sm text-white/70">
            <Camera size={15} className="mt-0.5 shrink-0 opacity-80" />
            <span className="max-w-[42ch]">Generated from the listing photos — no filming, no staging</span>
          </span>
          <span className="eyebrow inline-flex items-center gap-2 text-white/55">
            <span>{locked ? "Viewing" : "Touring"}</span>
            <AnimatePresence mode="wait">
              <motion.span
                key={style}
                initial={{ opacity: 0, y: 4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -4 }}
                transition={{ duration: 0.4 }}
                style={{ color: "var(--brand)" }}
              >
                {styleName}
              </motion.span>
            </AnimatePresence>
            {!locked && <ChevronDown size={14} className="animate-bounce" />}
          </span>
        </motion.div>
      </div>
    </header>
  );
}
