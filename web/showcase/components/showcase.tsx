"use client";

import { Camera, ShieldCheck, ShoppingBag, ArrowUpRight } from "lucide-react";
import { PROPERTY } from "@/lib/property";
import { StyleProvider } from "./style-context";
import { Hero } from "./hero";
import { StyleTabs } from "./style-tabs";
import { RoomTour } from "./room-tour";
import { Reveal } from "./reveal";

const BENEFITS = [
  {
    icon: Camera,
    title: "From the listing photos",
    body: "No new filming, no physical staging. The agent's existing photos are all it takes.",
  },
  {
    icon: ShieldCheck,
    title: "Honest by design",
    body: "Same walls, windows, layout and light. A gate rejects any render that changes a buyable fact.",
  },
  {
    icon: ShoppingBag,
    title: "Shoppable",
    body: "Every look is a real shopping list — the pieces on screen link straight to where you buy them.",
  },
];

export function Showcase() {
  return (
    <StyleProvider>
      <Hero />
      <StyleTabs />

      <main>
        {/* Benefits — the "how" behind the tour */}
        <section className="mx-auto max-w-6xl px-5 py-16 sm:px-8 sm:py-20">
          <Reveal>
            <p className="eyebrow mb-8 text-muted">Why it holds up</p>
          </Reveal>
          <div className="grid gap-8 sm:grid-cols-3">
            {BENEFITS.map((b, i) => (
              <Reveal key={b.title} delay={i * 0.08} className="flex flex-col gap-3">
                <b.icon size={22} style={{ color: "var(--brand)" }} />
                <h3 className="font-display text-xl font-normal tracking-tight">{b.title}</h3>
                <p className="text-[15px] leading-relaxed text-muted">{b.body}</p>
              </Reveal>
            ))}
          </div>
        </section>

        <RoomTour />

        {/* Closing CTA */}
        <section className="border-t border-line">
          <div className="mx-auto max-w-6xl px-5 py-24 sm:px-8 sm:py-32">
            <Reveal>
              <p className="eyebrow mb-6" style={{ color: "var(--brand)" }}>
                {PROPERTY.name}
              </p>
              <h2 className="max-w-[18ch] font-display text-[clamp(2.25rem,6vw,4.5rem)] font-light leading-[0.98] tracking-[-0.02em] text-balance">
                Every listing you have, seen at its best.
              </h2>
              <p className="mt-6 max-w-[54ch] text-lg leading-relaxed text-muted">
                Same rooms, same honesty — three ways to imagine them, and a shopping list to make it
                real. Restaged from the photos you already took.
              </p>
              <div className="mt-9 flex flex-wrap items-center gap-4">
                <a
                  href="#"
                  className="inline-flex items-center gap-2 rounded-full bg-ink px-6 py-3.5 text-[15px] font-medium text-canvas transition-transform duration-200 hover:-translate-y-0.5 cursor-pointer"
                >
                  Restage a listing
                  <ArrowUpRight size={17} />
                </a>
                <a
                  href="#"
                  className="inline-flex items-center gap-2 rounded-full border border-line px-6 py-3.5 text-[15px] font-medium text-ink transition-colors duration-200 hover:border-[color:var(--brand)] hover:text-[color:var(--brand)] cursor-pointer"
                >
                  Book a viewing
                </a>
              </div>
            </Reveal>
          </div>
        </section>

        <footer className="border-t border-line">
          <div className="mx-auto flex max-w-6xl flex-col gap-2 px-5 py-10 sm:flex-row sm:items-center sm:justify-between sm:px-8">
            <span className="eyebrow text-muted">Latent Frame · honest AI potential</span>
            <span className="text-sm text-muted">
              Restaged from the listing photos — same walls, same windows, only the possibility added.
            </span>
          </div>
        </footer>
      </main>
    </StyleProvider>
  );
}
