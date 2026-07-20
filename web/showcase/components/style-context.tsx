"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { STYLES, type StyleId } from "@/lib/property";

interface StyleState {
  style: StyleId;
  /** User-initiated selection — also locks the ambient auto-tour. */
  select: (s: StyleId) => void;
  /** True once the user has taken control (clicked a tab or scrolled). */
  locked: boolean;
}

const StyleCtx = createContext<StyleState>({ style: "mediterranean", select: () => {}, locked: false });

export const useStyle = () => useContext(StyleCtx);

export function StyleProvider({ children }: { children: React.ReactNode }) {
  const [style, setStyle] = useState<StyleId>("mediterranean");
  const [locked, setLocked] = useState(false);
  const lockedRef = useRef(false);

  const lock = useCallback(() => {
    lockedRef.current = true;
    setLocked(true);
  }, []);

  const select = useCallback(
    (s: StyleId) => {
      lock();
      setStyle(s);
    },
    [lock],
  );

  // Drive the style-aware accent from the document root so the body vignette and
  // every descendant inherit --brand. Kept out of JSX to avoid hydration mismatch.
  useEffect(() => {
    document.documentElement.setAttribute("data-style", style);
  }, [style]);

  // Ambient auto-tour: cross-dissolve through the three visions until the user
  // takes control. Skipped entirely under reduced-motion.
  useEffect(() => {
    if (locked) return;
    if (typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const id = setInterval(() => {
      setStyle((cur) => {
        const i = STYLES.findIndex((s) => s.id === cur);
        return STYLES[(i + 1) % STYLES.length].id;
      });
    }, 4200);
    return () => clearInterval(id);
  }, [locked]);

  // The first real scroll locks the tour so room sections stay put while reading.
  useEffect(() => {
    if (locked) return;
    const onScroll = () => {
      if (window.scrollY > 40) lock();
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [locked, lock]);

  return <StyleCtx.Provider value={{ style, select, locked }}>{children}</StyleCtx.Provider>;
}
