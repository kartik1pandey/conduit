import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { listPaymentIntents } from "@/lib/coreClient";
import { formatDate, formatMoney } from "@/lib/format";

export default async function TransactionsPage() {
  const session = await auth();
  const intents = await listPaymentIntents(session!.user.merchantId, {
    limit: 50,
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Transactions</h1>
        <Link
          href="/dashboard/transactions/new"
          className="inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
        >
          New payment
        </Link>
      </div>
      <Card>
        {intents.length === 0 ? (
          <EmptyState
            title="No payment intents yet"
            description="Create a test payment to see it move through risk scoring and the ledger in real time."
            actionHref="/dashboard/transactions/new"
            actionLabel="Create your first payment"
          />
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] text-left text-[var(--muted)]">
                <th className="px-4 py-3 font-medium">ID</th>
                <th className="px-4 py-3 font-medium">Amount</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {intents.map((pi) => (
                <tr
                  key={pi.id}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <td className="px-4 py-3">
                    <Link
                      href={`/dashboard/transactions/${pi.id}`}
                      className="font-mono text-xs text-[var(--accent)] hover:underline"
                    >
                      {pi.id}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    {formatMoney(pi.amount, pi.currency)}
                  </td>
                  <td className="px-4 py-3">
                    <Badge tone={statusTone(pi.status)}>{pi.status}</Badge>
                  </td>
                  <td className="px-4 py-3 text-[var(--muted)]">
                    {formatDate(pi.created_at)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}
