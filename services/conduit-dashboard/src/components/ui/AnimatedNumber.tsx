"use client";

import { useEffect, useRef } from "react";
import { useInView, useMotionValue, useSpring } from "framer-motion";

// Counts up to an integer stat (transaction counts, endpoint counts —
// never a money amount: animating a float toward a decimal value would
// mean interpolating cents mid-flight, which this project's own "never
// float for money" rule rules out even for a purely cosmetic effect).
export function AnimatedNumber({ value }: { value: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10% 0px" });
  const motionValue = useMotionValue(0);
  const spring = useSpring(motionValue, { stiffness: 90, damping: 20 });

  useEffect(() => {
    if (inView) motionValue.set(value);
  }, [inView, value, motionValue]);

  useEffect(() => {
    const unsubscribe = spring.on("change", (latest) => {
      if (ref.current) ref.current.textContent = String(Math.round(latest));
    });
    return unsubscribe;
  }, [spring]);

  return (
    <span ref={ref} className="tabular-nums">
      0
    </span>
  );
}
