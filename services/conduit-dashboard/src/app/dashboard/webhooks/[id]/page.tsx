import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
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
      <h1 className="text-2xl font-semibold">Delivery log</h1>
      <Card>
        {deliveries.length === 0 ? (
          <p className="p-6 text-sm text-[var(--muted)]">
            No deliveries recorded yet.
          </p>
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
              {deliveries.map((d) => (
                <tr
                  key={d.id}
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
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}
