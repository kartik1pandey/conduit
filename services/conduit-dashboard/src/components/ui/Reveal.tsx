"use client";

import { ReactNode } from "react";
import { motion } from "framer-motion";

const VARIANTS = {
  hidden: { opacity: 0, y: 8 },
  visible: { opacity: 1, y: 0 },
};

function transition(delay: number) {
  return { duration: 0.4, delay, ease: "easeOut" as const };
}

// Shared fade+translateY entrance — used both for row-by-row list
// reveals on data load (delay driven by index) and for scroll-triggered
// sections on the marketing page (viewport-based via whileInView). Table
// rows and list items need their own tag (a <div> can't be a <tr>'s or
// <ul>'s direct child without breaking HTML semantics), so each context
// gets its own thin wrapper sharing the same variants/transition shape.
export function Reveal({
  children,
  delay = 0,
  className = "",
  onScroll = false,
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
  onScroll?: boolean;
}) {
  const visibleProps = onScroll
    ? { whileInView: "visible", viewport: { once: true, margin: "-10% 0px" } }
    : { animate: "visible" };

  return (
    <motion.div
      initial="hidden"
      {...visibleProps}
      variants={VARIANTS}
      transition={transition(delay)}
      className={className}
    >
      {children}
    </motion.div>
  );
}

export function RevealLi({
  children,
  delay = 0,
  className = "",
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
}) {
  return (
    <motion.li
      initial="hidden"
      animate="visible"
      variants={VARIANTS}
      transition={transition(delay)}
      className={className}
    >
      {children}
    </motion.li>
  );
}

export function RevealTr({
  children,
  delay = 0,
  className = "",
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
}) {
  return (
    <motion.tr
      initial="hidden"
      animate="visible"
      variants={VARIANTS}
      transition={transition(delay)}
      className={className}
    >
      {children}
    </motion.tr>
  );
}
