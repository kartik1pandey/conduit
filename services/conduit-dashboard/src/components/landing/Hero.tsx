"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight } from "lucide-react";

export function Hero() {
  return (
    <section className="bg-grain relative overflow-hidden px-6 pb-24 pt-20 sm:pt-28">
      <div className="bg-mesh absolute inset-0 opacity-70" />
      <div className="relative mx-auto max-w-3xl text-center">
        <motion.span
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
          className="inline-flex items-center rounded-full border border-[var(--l-border)] bg-[var(--l-surface)] px-3 py-1 text-xs font-medium text-[var(--l-muted)]"
        >
          Test mode · no real money ever moves
        </motion.span>

        <motion.h1
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.1 }}
          className="mt-6 font-display text-4xl font-semibold tracking-tight text-[var(--l-foreground)] sm:text-6xl"
        >
          Payments infrastructure,
          <br />
          built like it has to be right.
        </motion.h1>

        <motion.p
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
          className="mx-auto mt-6 max-w-xl text-base text-[var(--l-muted)]"
        >
          Conduit scores every payment for risk in real time, writes it to an
          immutable double-entry ledger, and confirms it with a
          reliably-delivered, signed webhook — the same shape as a real
          processor, minus the real money.
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.3 }}
          className="mt-8 flex flex-wrap items-center justify-center gap-3"
        >
          <Link
            href="/signup"
            className="inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] bg-[var(--l-accent)] px-5 py-2.5 text-sm font-medium text-white transition hover:opacity-90"
          >
            Get started
            <ArrowRight className="size-4" strokeWidth={1.75} />
          </Link>
          <a
            href="#flow"
            className="inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--l-border)] px-5 py-2.5 text-sm font-medium text-[var(--l-foreground)] transition hover:bg-[var(--l-surface)]"
          >
            See how it works
          </a>
        </motion.div>
      </div>
    </section>
  );
}
