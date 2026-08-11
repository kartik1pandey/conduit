"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/Button";
import { useActionToast } from "@/lib/useActionToast";
import { createPayment } from "./actions";

export function NewPaymentForm({ idempotencyKey }: { idempotencyKey: string }) {
  const [state, formAction, pending] = useActionState(createPayment, undefined);
  useActionToast(state);

  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="idempotencyKey" value={idempotencyKey} />

      <label className="block">
        <span className="mb-1 block text-sm font-medium">Amount</span>
        <input
          name="amount"
          type="text"
          inputMode="decimal"
          placeholder="25.00"
          required
          className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        />
      </label>

      <label className="block">
        <span className="mb-1 block text-sm font-medium">Currency</span>
        <select
          name="currency"
          defaultValue="usd"
          className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        >
          <option value="usd">USD</option>
          <option value="ngn">NGN</option>
          <option value="xof">XOF</option>
        </select>
      </label>

      <label className="block">
        <span className="mb-1 block text-sm font-medium">
          Description (optional)
        </span>
        <input
          name="description"
          type="text"
          placeholder="Widget purchase"
          className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        />
      </label>

      {state?.error && (
        <p className="text-sm text-[var(--danger)]">{state.error}</p>
      )}

      <Button type="submit" disabled={pending} className="w-full">
        {pending ? "Creating…" : "Create payment intent"}
      </Button>
    </form>
  );
}
