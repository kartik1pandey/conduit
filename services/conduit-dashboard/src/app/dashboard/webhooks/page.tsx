import Link from "next/link";
import { auth } from "@/auth";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { listWebhookEndpoints } from "@/lib/coreClient";
import { formatDate } from "@/lib/format";

export default async function WebhooksPage() {
  const session = await auth();
  const endpoints = await listWebhookEndpoints(session!.user.merchantId);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Webhook endpoints</h1>
        <Link
          href="/dashboard/webhooks/new"
          className="inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
        >
          Register endpoint
        </Link>
      </div>
      <Card>
        {endpoints.length === 0 ? (
          <EmptyState
            title="No webhook endpoints yet"
            description="Register a URL to receive signed payment.succeeded/failed/refunded events as they happen."
            actionHref="/dashboard/webhooks/new"
            actionLabel="Register your first endpoint"
          />
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[var(--border)] text-left text-[var(--muted)]">
                <th className="px-4 py-3 font-medium">URL</th>
                <th className="px-4 py-3 font-medium">Registered</th>
              </tr>
            </thead>
            <tbody>
              {endpoints.map((ep) => (
                <tr
                  key={ep.id}
                  className="border-b border-[var(--border)] last:border-0"
                >
                  <td className="px-4 py-3">
                    <Link
                      href={`/dashboard/webhooks/${ep.id}`}
                      className="text-[var(--accent)] hover:underline"
                    >
                      {ep.url}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-[var(--muted)]">
                    {formatDate(ep.created_at)}
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
