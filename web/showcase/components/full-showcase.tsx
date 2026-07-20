"use client";

import { motion, useReducedMotion } from "framer-motion";
import Image from "next/image";
import { ChevronDown, Camera, ArrowUpRight } from "lucide-react";
import { GROUPS, heroSpace, type FullManifest, type FullSpace, type FullProperty } from "@/lib/full";
import { StyleProvider, useStyle } from "./style-context";
import { StyleTabs } from "./style-tabs";
import { SpaceBlock } from "./space-block";
import { Reveal } from "./reveal";
import { ThemeToggle } from "./theme-toggle";
import { Logo } from "./logo";
import { MorphImage } from "./morph-image";

function FullHero({ hero, property }: { hero: FullSpace; property: FullProperty }) {
  const { style } = useStyle();
  const reduce = useReducedMotion();
  const afterSrc = hero.after?.[style];

  return (
    <header className="relative flex min-h-[100svh] flex-col overflow-hidden">
      <div className="absolute inset-0">
        <motion.div
          className="absolute inset-0"
          animate={reduce ? {} : { scale: [1, 1.08] }}
          transition={{ duration: 18, ease: "linear", repeat: Infinity, repeatType: "reverse" }}
        >
          {hero.generated && afterSrc ? (
            <MorphImage srcKey={style} src={afterSrc} alt={hero.name} sizes="100vw" priority />
          ) : (
            <Image src={hero.before} alt={hero.name} fill sizes="100vw" priority className="object-cover" />
          )}
        </motion.div>
        <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/25 to-black/45" />
        <div className="absolute inset-0 bg-gradient-to-r from-black/60 to-transparent" />
      </div>

      <div className="relative z-10 mx-auto flex w-full max-w-6xl items-center justify-between px-5 py-5 sm:px-8">
        <Logo className="text-white" />
        <ThemeToggle />
      </div>

      <div className="relative z-10 mx-auto mt-auto w-full max-w-6xl px-5 pb-16 sm:px-8 sm:pb-20">
        <motion.p
          initial={reduce ? false : { opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: [0.16, 1, 0.3, 1] }}
          className="eyebrow mb-5 flex flex-wrap items-center gap-x-3 gap-y-1 text-white/85"
        >
          <span style={{ color: "var(--brand)" }}>The potential, honestly staged</span>
          <span className="text-white/40">/</span>
          <span>{property.location}</span>
        </motion.p>
        <motion.h1
          initial={reduce ? false : { opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.1 }}
          className="max-w-[16ch] font-display text-[clamp(2.75rem,8vw,6rem)] font-light leading-[0.95] tracking-[-0.03em] text-white text-balance"
        >
          {property.name},{" "}
          <em className="font-medium italic" style={{ color: "var(--brand)" }}>
            at its best.
          </em>
        </motion.h1>
        <motion.p
          initial={reduce ? false : { opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.25 }}
          className="mt-6 max-w-[54ch] text-lg leading-relaxed text-white/80 text-balance"
        >
          <span className="capitalize">{property.type}</span> · sleeps ~{property.sleeps_estimate}. Every space reimagined three complete ways —
          inside and out — from the listing photos alone.
        </motion.p>
        <motion.div
          initial={reduce ? false : { opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 1, delay: 0.5 }}
          className="mt-9 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-6"
        >
          <span className="flex items-start gap-2 text-sm text-white/70">
            <Camera size={15} className="mt-0.5 shrink-0 opacity-80" />
            <span className="max-w-[42ch]">Restaged from the listing photos — no filming, no staging</span>
          </span>
          <span className="eyebrow inline-flex items-center gap-1.5 text-white/55">
            Scroll to explore <ChevronDown size={14} className="animate-bounce" />
          </span>
        </motion.div>
      </div>
    </header>
  );
}

export function FullShowcase({ data }: { data: FullManifest }) {
  const hero = heroSpace(data);
  const property = data.property;
  return (
    <StyleProvider>
      <FullHero hero={hero} property={property} />
      <StyleTabs />

      <main>
        {GROUPS.map((g) => {
          const spaces = data.spaces.filter((s) => s.category === g.key);
          if (!spaces.length) return null;
          return (
            <div key={g.key} className="mx-auto max-w-6xl px-5 sm:px-8">
              <Reveal className="pt-16 sm:pt-24">
                <p className="eyebrow text-muted">{g.sub}</p>
                <h2 className="mt-2 font-display text-[clamp(2rem,5vw,3.5rem)] font-light tracking-tight">{g.label}</h2>
              </Reveal>
              {spaces.map((s, i) => (
                <SpaceBlock key={s.id} space={s} index={String(i + 1).padStart(2, "0")} />
              ))}
            </div>
          );
        })}

        <section className="border-t border-line">
          <div className="mx-auto max-w-6xl px-5 py-24 sm:px-8 sm:py-32">
            <Reveal>
              <p className="eyebrow mb-6" style={{ color: "var(--brand)" }}>
                {property.name}
              </p>
              <h2 className="max-w-[18ch] font-display text-[clamp(2.25rem,6vw,4.5rem)] font-light leading-[0.98] tracking-[-0.02em] text-balance">
                The whole home, seen at its best.
              </h2>
              <p className="mt-6 max-w-[54ch] text-lg leading-relaxed text-muted">
                Inside and out, three ways — restaged from the photos you already have.
              </p>
              <div className="mt-9 flex flex-wrap items-center gap-4">
                <a
                  href="#"
                  className="inline-flex items-center gap-2 rounded-full bg-ink px-6 py-3.5 text-[15px] font-medium text-canvas transition-transform duration-200 hover:-translate-y-0.5 cursor-pointer"
                >
                  Book a viewing
                  <ArrowUpRight size={17} />
                </a>
              </div>
            </Reveal>
          </div>
        </section>

        <footer className="border-t border-line">
          <div className="mx-auto flex max-w-6xl flex-col gap-3 px-5 py-10 text-ink sm:flex-row sm:items-center sm:justify-between sm:px-8">
            <Logo />
            <span className="text-sm text-muted">
              Restaged from the listing photos — same walls, same windows, only the possibility added.
            </span>
          </div>
        </footer>
      </main>
    </StyleProvider>
  );
}
