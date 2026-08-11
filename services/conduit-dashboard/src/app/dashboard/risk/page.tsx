import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { listRiskDecisions } from "@/lib/coreClient";
import { formatDate, formatMoney } from "@/lib/format";

export default async function RiskPage() {
  const session = await auth();
  const decisions = await listRiskDecisions(session!.user.merchantId);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Risk decisions</h1>
      <Card>
        {decisions.length === 0 ? (
          <EmptyState
            title="No scoring history yet"
            description="Every payment confirmation is scored by conduit-risk in real time — confirm a payment to see the decision and reasons show up here."
            actionHref="/dashboard/transactions/new"
            actionLabel="Create a payment"
          />
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] text-left text-[var(--muted)]">
                <th className="px-4 py-3 font-medium">Payment intent</th>
                <th className="px-4 py-3 font-medium">Amount</th>
                <th className="px-4 py-3 font-medium">Decision</th>
                <th className="px-4 py-3 font-medium">Score</th>
                <th className="px-4 py-3 font-medium">Reasons</th>
                <th className="px-4 py-3 font-medium">When</th>
              </tr>
            </thead>
            <tbody>
              {decisions.map((d) => (
                <tr
                  key={d.id}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <td className="px-4 py-3">
                    <Link
                      href={`/dashboard/transactions/${d.payment_intent_id}`}
                      className="font-mono text-xs text-[var(--accent)] hover:underline"
                    >
                      {d.payment_intent_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    {formatMoney(d.amount, d.currency)}
                  </td>
                  <td className="px-4 py-3">
                    <Badge tone={statusTone(d.decision)}>{d.decision}</Badge>
                  </td>
                  <td className="px-4 py-3">
                    {d.risk_score.toFixed(2)}{" "}
                    <span className="text-[var(--muted)]">({d.stage})</span>
                  </td>
                  <td className="px-4 py-3 text-[var(--muted)]">
                    {d.reasons.length ? d.reasons.join(", ") : "—"}
                  </td>
                  <td className="px-4 py-3 text-[var(--muted)]">
                    {formatDate(d.created_at)}
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
