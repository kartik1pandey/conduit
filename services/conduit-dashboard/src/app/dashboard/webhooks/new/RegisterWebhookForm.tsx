"use client";

import Link from "next/link";
import { useActionState } from "react";
import { Button } from "@/components/ui/Button";
import { registerWebhook } from "./actions";

export function RegisterWebhookForm({
  idempotencyKey,
}: {
  idempotencyKey: string;
}) {
  const [state, formAction, pending] = useActionState(
    registerWebhook,
    undefined,
  );

  if (state && "ok" in state) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-emerald-600 dark:text-emerald-400">
          Endpoint registered. Save this signing secret now — it won&apos;t be
          shown again.
        </p>
        <div className="rounded-lg border border-[var(--border)] bg-[var(--background)] p-3">
          <code data-testid="webhook-secret" className="break-all text-xs">
            {state.secret}
          </code>
        </div>
        <p className="text-xs text-[var(--muted)]">
          Use it to verify the <code>Conduit-Signature</code> header on every
          delivery to this URL.
        </p>
        <Link
          href="/dashboard/webhooks"
          className="text-sm text-[var(--accent)] hover:underline"
        >
          Back to webhook endpoints
        </Link>
      </div>
    );
  }

  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="idempotencyKey" value={idempotencyKey} />

      <label className="block">
        <span className="mb-1 block text-sm font-medium">Endpoint URL</span>
        <input
          name="url"
          type="url"
          placeholder="https://your-app.example.com/webhooks/conduit"
          required
          className="w-full rounded-lg border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        />
      </label>

      {state && "error" in state && (
        <p className="text-sm text-rose-500">{state.error}</p>
      )}

      <Button type="submit" disabled={pending} className="w-full">
        {pending ? "Registering…" : "Register endpoint"}
      </Button>
    </form>
  );
}
