"use client";

import { ReactNode, useState } from "react";
import Lenis from "lenis/react";

// Wraps native scroll with inertia so the scroll-linked reveals below
// feel deliberate rather than jerky. Marketing page only — the dashboard
// itself never uses this, since a payments tool should feel
// instantaneous, not cinematic. Disabled entirely for
// prefers-reduced-motion, same as every scroll-triggered effect on this
// page. Computed once in the lazy initializer rather than an effect —
// window is unavailable during SSR, but this component only ever
// renders on the client.
export function SmoothScroll({ children }: { children: ReactNode }) {
  const [enabled] = useState(() =>
    typeof window === "undefined"
      ? true
      : !window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  );

  if (!enabled) return <>{children}</>;
  return <Lenis root>{children}</Lenis>;
}
