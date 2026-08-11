import Link from "next/link";
import { auth } from "@/auth";
import { Badge, statusTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { getPaymentIntent } from "@/lib/coreClient";
import { formatDate, formatMoney } from "@/lib/format";
import { ConfirmButton } from "./ConfirmButton";
import { RefundButton } from "./RefundButton";

export default async function TransactionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const session = await auth();
  const pi = await getPaymentIntent(session!.user.merchantId, id);

  return (
    <div className="space-y-6">
      <div>
        <Link
          href="/dashboard/transactions"
          className="text-sm text-[var(--muted)] hover:text-[var(--foreground)]"
        >
          ← Transactions
        </Link>
        <h1 className="mt-1 text-2xl font-semibold">Transaction</h1>
      </div>
      <Card className="p-6">
        <dl className="grid grid-cols-2 gap-4 text-sm">
          <Field
            label="ID"
            value={<span className="font-mono text-xs">{pi.id}</span>}
          />
          <Field
            label="Status"
            value={<Badge tone={statusTone(pi.status)}>{pi.status}</Badge>}
          />
          <Field label="Amount" value={formatMoney(pi.amount, pi.currency)} />
          <Field label="Created" value={formatDate(pi.created_at)} />
          {pi.description && (
            <Field label="Description" value={pi.description} />
          )}
          {pi.failure_reason && (
            <Field
              label="Failure reason"
              value={pi.failure_reason.replaceAll(",", ", ")}
            />
          )}
        </dl>

        {(pi.status === "created" || pi.status === "pending") && (
          <div className="mt-6 border-t border-[var(--border)] pt-6">
            <p className="mb-3 text-sm text-[var(--muted)]">
              Confirming scores this payment with conduit-risk and, if allowed,
              posts a balanced entry to conduit-ledger — live, not simulated.
            </p>
            <ConfirmButton paymentIntentId={pi.id} />
          </div>
        )}

        {pi.status === "succeeded" && (
          <div className="mt-6 border-t border-[var(--border)] pt-6">
            <RefundButton paymentIntentId={pi.id} />
          </div>
        )}
      </Card>
    </div>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-[var(--muted)]">{label}</dt>
      <dd className="mt-1">{value}</dd>
    </div>
  );
}
