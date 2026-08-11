import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
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
            className="inline-flex items-center justify-center rounded-lg border border-[var(--border)] px-4 py-2 text-sm font-medium transition hover:bg-[var(--border)]/40"
          >
            Register webhook
          </Link>
          <Link
            href="/dashboard/transactions/new"
            className="inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
          >
            New payment
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <StatCard label="Transactions" value={String(intents.length)} />
        <StatCard label="Succeeded" value={String(succeeded)} tone="success" />
        <StatCard label="Failed" value={String(failed)} tone="danger" />
        <StatCard label="Webhook endpoints" value={String(endpoints.length)} />
      </div>

      {volumes.length > 0 && (
        <Card className="p-6">
          <p className="mb-3 text-sm font-medium">Succeeded volume</p>
          <div className="flex flex-wrap gap-6">
            {volumes.map((v) => (
              <div key={v.currency}>
                <p className="text-2xl font-semibold">
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
                className="mt-2 inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
              >
                Create your first payment
              </Link>
            </div>
          ) : (
            <ul>
              {recent.map((pi) => (
                <li
                  key={pi.id}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <Link
                    href={`/dashboard/transactions/${pi.id}`}
                    className="flex items-center justify-between px-4 py-3 text-sm transition hover:bg-[var(--border)]/20"
                  >
                    <span className="font-mono text-xs text-[var(--muted)]">
                      {pi.id}
                    </span>
                    <span>{formatMoney(pi.amount, pi.currency)}</span>
                    <Badge tone={statusTone(pi.status)}>{pi.status}</Badge>
                    <span className="text-[var(--muted)]">
                      {formatDate(pi.created_at)}
                    </span>
                  </Link>
                </li>
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
  value: string;
  tone?: "success" | "danger";
}) {
  const valueColor =
    tone === "success"
      ? "text-emerald-600 dark:text-emerald-400"
      : tone === "danger"
        ? "text-rose-600 dark:text-rose-400"
        : "";
  return (
    <Card className="p-4">
      <p className="text-xs text-[var(--muted)]">{label}</p>
      <p className={`mt-1 text-2xl font-semibold ${valueColor}`}>{value}</p>
    </Card>
  );
}
