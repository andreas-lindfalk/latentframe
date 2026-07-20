"use client";

import { motion } from "framer-motion";
import { STYLES } from "@/lib/property";
import { useStyle } from "./style-context";

export function StyleTabs() {
  const { style, select } = useStyle();

  return (
    <div className="sticky top-0 z-30 border-b border-line bg-canvas/80 backdrop-blur-xl">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-5 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-8">
        <span className="eyebrow text-muted">Choose the look</span>
        <div className="flex flex-wrap gap-1.5" role="tablist" aria-label="Decoration style">
          {STYLES.map((s) => {
            const active = s.id === style;
            return (
              <button
                key={s.id}
                role="tab"
                aria-selected={active}
                onClick={() => select(s.id)}
                className={`relative flex flex-col items-start rounded-xl px-4 py-2 text-left transition-colors duration-200 cursor-pointer ${
                  active ? "text-ink" : "text-muted hover:text-ink"
                }`}
              >
                {active && (
                  <motion.span
                    layoutId="tab-pill"
                    className="absolute inset-0 -z-10 rounded-xl border"
                    style={{ background: "var(--brand-soft)", borderColor: "var(--brand)" }}
                    transition={{ type: "spring", stiffness: 380, damping: 32 }}
                  />
                )}
                <span className="text-[15px] font-semibold leading-tight tracking-tight">{s.name}</span>
                <span className="eyebrow mt-0.5 text-[0.6rem] opacity-70">{s.tagline}</span>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
