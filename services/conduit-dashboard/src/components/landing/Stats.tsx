import { AnimatedNumber } from "@/components/ui/AnimatedNumber";

const STATS = [
  { value: 6, suffix: "", label: "Independently deployable services" },
  {
    value: 100,
    suffix: "%",
    label: "Of write endpoints require an idempotency key",
  },
  { value: 0, suffix: "", label: "Floating-point operations in ledger math" },
  { value: 24, suffix: "h", label: "Idempotency key TTL in Redis" },
];

// Real architectural facts, not growth-hacked marketing numbers — every
// one of these is a non-negotiable from this project's own CLAUDE.md,
// stated plainly rather than dressed up, since the honest version is
// more credible to the audience this page is actually for.
export function Stats() {
  return (
    <section className="border-y border-[var(--l-border)] bg-[var(--l-surface)] px-6 py-14">
      <div className="mx-auto grid max-w-5xl grid-cols-2 gap-8 sm:grid-cols-4">
        {STATS.map((s) => (
          <div key={s.label} className="text-center">
            <p className="font-display text-3xl font-semibold text-[var(--l-foreground)] sm:text-4xl">
              <AnimatedNumber value={s.value} />
              {s.suffix}
            </p>
            <p className="mt-2 text-xs text-[var(--l-muted)]">{s.label}</p>
          </div>
        ))}
      </div>
    </section>
  );
}
