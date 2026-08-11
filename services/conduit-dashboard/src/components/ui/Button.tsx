"use client";

import { ButtonHTMLAttributes } from "react";
import { motion } from "framer-motion";

type Variant = "primary" | "secondary" | "danger";

const VARIANTS: Record<Variant, string> = {
  primary:
    "bg-[var(--accent)] text-[var(--accent-foreground)] hover:opacity-90",
  secondary:
    "bg-transparent border border-[var(--border)] text-[var(--foreground)] hover:bg-[var(--surface-elevated)]",
  danger: "bg-[var(--danger)] text-white hover:opacity-90",
};

// stiffness/damping tuned high enough that press feedback reads as
// "snappy tool," never "bouncy toy" — no overshoot on release.
const TAP_SPRING = { type: "spring" as const, stiffness: 400, damping: 25 };

// framer-motion's motion.button redefines onDrag*/onAnimation* with its
// own gesture signatures, which collide with the DOM event types React's
// ButtonHTMLAttributes expects — omit them since this component never
// uses drag gestures.
type NativeButtonProps = Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "onDrag" | "onDragStart" | "onDragEnd" | "onAnimationStart" | "onAnimationEnd"
>;

export function Button({
  variant = "primary",
  className = "",
  ...props
}: NativeButtonProps & { variant?: Variant }) {
  return (
    <motion.button
      whileTap={{ scale: 0.97 }}
      transition={TAP_SPRING}
      className={`inline-flex items-center justify-center gap-2 rounded-[var(--radius-sm)] px-4 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${VARIANTS[variant]} ${className}`}
      {...props}
    />
  );
}
