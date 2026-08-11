import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { RevealTr } from "@/components/ui/Reveal";
import { listWebhookDeliveries } from "@/lib/coreClient";
import { formatDate } from "@/lib/format";

export default async function WebhookDeliveriesPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const session = await auth();
  const deliveries = await listWebhookDeliveries(session!.user.merchantId, id);

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/dashboard/webhooks"
          className="text-sm text-[var(--muted)] hover:text-[var(--foreground)]"
        >
          ← Webhook endpoints
        </Link>
        <h1 className="mt-1 text-2xl font-semibold">Delivery log</h1>
      </div>
      <Card>
        {deliveries.length === 0 ? (
          <EmptyState
            title="No deliveries recorded yet"
            description="This endpoint will receive a signed delivery the next time a payment succeeds, fails, or gets refunded for this merchant."
            actionHref="/dashboard/transactions/new"
            actionLabel="Create a payment"
          />
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] text-left text-[var(--muted)]">
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 font-medium">Attempts</th>
                <th className="px-4 py-3 font-medium">Last response</th>
                <th className="px-4 py-3 font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {deliveries.map((d, i) => (
                <RevealTr
                  key={d.id}
                  delay={Math.min(i, 12) * 0.03}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <td className="px-4 py-3">
                    <Badge tone={statusTone(d.status)}>{d.status}</Badge>
                  </td>
                  <td className="px-4 py-3">{d.attempt_count}</td>
                  <td className="px-4 py-3">{d.last_response_status ?? "—"}</td>
                  <td className="px-4 py-3 text-[var(--muted)]">
                    {formatDate(d.created_at)}
                  </td>
                </RevealTr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}
