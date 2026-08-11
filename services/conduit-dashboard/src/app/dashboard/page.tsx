import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { AnimatedNumber } from "@/components/ui/AnimatedNumber";
import { Reveal, RevealLi } from "@/components/ui/Reveal";
import {
  listPaymentIntents,
  listWebhookEndpoints,
  type PaymentIntent,
} from "@/lib/coreClient";
import { formatDate, formatMoney, sumMoney } from "@/lib/format";

// Grouped by currency rather than summed across all of them — adding a
// USD amount to an NGN amount would produce a number that means nothing,
// the same reason this project never lets a query span merchants: summing
// unlike things silently produces a wrong-but-plausible-looking answer.
function volumeByCurrency(
  intents: PaymentIntent[],
): { currency: string; total: string }[] {
  const byCurrency = new Map<string, string[]>();
  for (const pi of intents) {
    if (pi.status !== "succeeded") continue;
    const existing = byCurrency.get(pi.currency) ?? [];
    existing.push(pi.amount);
    byCurrency.set(pi.currency, existing);
  }
  return Array.from(byCurrency.entries()).map(([currency, amounts]) => ({
    currency,
    total: sumMoney(amounts),
  }));
}

export default async function DashboardOverviewPage() {
  const session = await auth();
  const merchantId = session!.user.merchantId;

  const [intents, endpoints] = await Promise.all([
    listPaymentIntents(merchantId, { limit: 100 }),
    listWebhookEndpoints(merchantId),
  ]);

  const succeeded = intents.filter((pi) => pi.status === "succeeded").length;
  const failed = intents.filter((pi) => pi.status === "failed").length;
  const refunded = intents.filter((pi) => pi.status === "refunded").length;
  const volumes = volumeByCurrency(intents);
  const recent = intents.slice(0, 5);

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Overview</h1>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Welcome back, {session?.user.email} ·{" "}
            <span className="font-mono text-xs">{merchantId}</span>
          </p>
        </div>
        <div className="flex gap-3">
          <Link
            href="/dashboard/webhooks/new"
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] px-4 py-2 text-sm font-medium transition hover:bg-[var(--surface-elevated)]"
          >
            Register webhook
          </Link>
          <Link
            href="/dashboard/transactions/new"
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] bg-[var(--accent)] px-4 py-2 text-sm font-medium text-[var(--accent-foreground)] transition hover:opacity-90"
          >
            New payment
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Reveal delay={0}>
          <StatCard label="Transactions" value={intents.length} />
        </Reveal>
        <Reveal delay={0.05}>
          <StatCard label="Succeeded" value={succeeded} tone="success" />
        </Reveal>
        <Reveal delay={0.1}>
          <StatCard label="Failed" value={failed} tone="danger" />
        </Reveal>
        <Reveal delay={0.15}>
          <StatCard label="Webhook endpoints" value={endpoints.length} />
        </Reveal>
      </div>

      {volumes.length > 0 && (
        <Reveal delay={0.2}>
          <Card className="p-6">
            <p className="mb-3 text-sm font-medium">Succeeded volume</p>
            <div className="flex flex-wrap gap-6">
              {volumes.map((v) => (
                <div key={v.currency}>
                  <p className="font-display text-2xl font-semibold tabular-nums">
                    {formatMoney(v.total, v.currency)}
                  </p>
                </div>
              ))}
            </div>
            {refunded > 0 && (
              <p className="mt-3 text-xs text-[var(--muted)]">
                {refunded} of the above {refunded === 1 ? "has" : "have"} since
                been refunded.
              </p>
            )}
          </Card>
        </Reveal>
      )}

      <div>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-medium text-[var(--muted)]">
            Recent activity
          </h2>
          {intents.length > 0 && (
            <Link
              href="/dashboard/transactions"
              className="text-sm text-[var(--accent)] hover:underline"
            >
              View all
            </Link>
          )}
        </div>
        <Card>
          {recent.length === 0 ? (
            <div className="flex flex-col items-center gap-3 p-12 text-center">
              <p className="font-medium">Nothing here yet</p>
              <p className="max-w-sm text-sm text-[var(--muted)]">
                Create a test payment to see it move through risk scoring and
                the ledger in real time.
              </p>
              <Link
                href="/dashboard/transactions/new"
                className="mt-2 inline-flex items-center justify-center rounded-[var(--radius-sm)] bg-[var(--accent)] px-4 py-2 text-sm font-medium text-[var(--accent-foreground)] transition hover:opacity-90"
              >
                Create your first payment
              </Link>
            </div>
          ) : (
            <ul>
              {recent.map((pi, i) => (
                <RevealLi
                  key={pi.id}
                  delay={i * 0.04}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <Link
                    href={`/dashboard/transactions/${pi.id}`}
                    className="flex items-center justify-between px-4 py-3 text-sm transition hover:bg-[var(--surface-elevated)]"
                  >
                    <span className="font-mono text-xs text-[var(--muted)]">
                      {pi.id}
                    </span>
                    <span className="tabular-nums">
                      {formatMoney(pi.amount, pi.currency)}
                    </span>
                    <Badge tone={statusTone(pi.status)}>{pi.status}</Badge>
                    <span className="text-[var(--muted)]">
                      {formatDate(pi.created_at)}
                    </span>
                  </Link>
                </RevealLi>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </div>
  );
}

function StatCard({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "success" | "danger";
}) {
  const valueColor =
    tone === "success"
      ? "text-[var(--success)]"
      : tone === "danger"
        ? "text-[var(--danger)]"
        : "";
  return (
    <Card className="p-4">
      <p className="text-xs text-[var(--muted)]">{label}</p>
      <p className={`mt-1 font-display text-2xl font-semibold ${valueColor}`}>
        <AnimatedNumber value={value} />
      </p>
    </Card>
  );
}
