"use client";

import { motion } from "framer-motion";
import {
  KeyRound,
  ShieldCheck,
  Scale,
  Webhook,
  Building2,
  Gauge,
} from "lucide-react";

const FEATURES = [
  {
    icon: KeyRound,
    title: "Idempotency, from the first endpoint",
    detail:
      "Every write requires an Idempotency-Key. A duplicate returns the original response instead of reprocessing — checked in Redis before Postgres.",
    span: "sm:col-span-2",
  },
  {
    icon: Scale,
    title: "Double-entry ledger",
    detail:
      "Debits equal credits, enforced at the database layer. Entries are append-only — nothing is ever updated in place.",
    span: "",
  },
  {
    icon: ShieldCheck,
    title: "Real-time risk scoring",
    detail:
      "A two-stage classifier plus an OPA/Rego policy layer, called synchronously on confirm. A decline blocks the charge before any ledger entry exists.",
    span: "",
  },
  {
    icon: Webhook,
    title: "Reliable webhook delivery",
    detail:
      "HMAC-SHA256 signed payloads, exponential backoff with jitter, and a dead-letter state — proven against a flaky-receiver chaos suite, not just the happy path.",
    span: "sm:col-span-2",
  },
  {
    icon: Building2,
    title: "Multi-tenancy at the query layer",
    detail:
      "Every query is scoped to the authenticated merchant_id — not just checked once at the top of the request.",
    span: "",
  },
  {
    icon: Gauge,
    title: "Per-key rate limiting",
    detail:
      "A token-bucket limiter in Redis means one noisy API key can't degrade service for everyone else.",
    span: "",
  },
];

export function FeatureGrid() {
  return (
    <section className="px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <h2 className="font-display text-2xl font-semibold text-[var(--l-foreground)] sm:text-3xl">
          Built on the parts that are easy to skip.
        </h2>
        <p className="mt-2 max-w-lg text-sm text-[var(--l-muted)]">
          None of this is a feature tour — it&apos;s the set of guarantees a
          payments system doesn&apos;t get to be wrong about.
        </p>

        <div className="mt-10 grid gap-4 sm:grid-cols-2">
          {FEATURES.map((f, i) => (
            <motion.div
              key={f.title}
              initial={{ opacity: 0, y: 12 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-10% 0px" }}
              transition={{ duration: 0.4, delay: (i % 2) * 0.08 }}
              className={`rounded-[var(--radius-md)] border border-[var(--l-border)] bg-[var(--l-surface)] p-6 ${f.span}`}
            >
              <f.icon
                className="size-5 text-[var(--l-accent)]"
                strokeWidth={1.75}
              />
              <p className="mt-4 font-medium text-[var(--l-foreground)]">
                {f.title}
              </p>
              <p className="mt-1.5 text-sm text-[var(--l-muted)]">{f.detail}</p>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
