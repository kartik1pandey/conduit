"use client";

import { motion } from "framer-motion";
import { CreditCard, ShieldCheck, BookOpen, Webhook } from "lucide-react";

const STEPS = [
  {
    icon: CreditCard,
    label: "Payment intent",
    detail: "created, idempotency key checked",
  },
  {
    icon: ShieldCheck,
    label: "Risk score",
    detail: "AEGIS classifier + OPA policy, synchronous",
  },
  {
    icon: BookOpen,
    label: "Ledger entry",
    detail: "debits = credits, enforced at the DB",
  },
  {
    icon: Webhook,
    label: "Webhook delivery",
    detail: "HMAC-signed, retried with backoff",
  },
];

// A hand-drawn stand-in for a product screenshot: the line "draws
// itself" via pathLength as the section scrolls into view, and each
// node fades in behind it — this is the one place on the page allowed
// to look like a diagram rather than a working screen, since it's
// explaining the shape of a request, not a page a merchant clicks
// through.
export function FlowDiagram() {
  return (
    <div className="relative py-6">
      <svg
        className="absolute left-0 top-9 hidden h-2 w-full sm:block"
        viewBox="0 0 100 2"
        preserveAspectRatio="none"
      >
        <motion.line
          x1="6"
          y1="1"
          x2="94"
          y2="1"
          stroke="var(--l-accent)"
          strokeWidth="0.4"
          vectorEffect="non-scaling-stroke"
          initial={{ pathLength: 0, opacity: 0 }}
          whileInView={{ pathLength: 1, opacity: 1 }}
          viewport={{ once: true, margin: "-20% 0px" }}
          transition={{ duration: 1.4, ease: "easeInOut" }}
        />
      </svg>

      <div className="relative grid grid-cols-2 gap-8 sm:grid-cols-4">
        {STEPS.map((step, i) => (
          <motion.div
            key={step.label}
            className="flex flex-col items-center gap-3 text-center"
            initial={{ opacity: 0, y: 12 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: "-20% 0px" }}
            transition={{ duration: 0.5, delay: 0.3 + i * 0.25 }}
          >
            <div className="flex size-14 items-center justify-center rounded-full border border-[var(--l-border)] bg-[var(--l-surface)]">
              <step.icon
                className="size-6 text-[var(--l-accent)]"
                strokeWidth={1.5}
              />
            </div>
            <div>
              <p className="text-sm font-medium text-[var(--l-foreground)]">
                {step.label}
              </p>
              <p className="mt-1 max-w-32 text-xs text-[var(--l-muted)]">
                {step.detail}
              </p>
            </div>
          </motion.div>
        ))}
      </div>
    </div>
  );
}
