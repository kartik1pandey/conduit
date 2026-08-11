"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/Button";
import { useActionToast } from "@/lib/useActionToast";
import { refund, type RefundState } from "./actions";

export function RefundButton({ paymentIntentId }: { paymentIntentId: string }) {
  const [state, formAction, pending] = useActionState<RefundState, FormData>(
    async (prevState) => refund(paymentIntentId, prevState),
    undefined,
  );
  useActionToast(state, "Refund issued.");

  if (state && "ok" in state) {
    return <p className="text-sm text-[var(--success)]">Refund issued.</p>;
  }

  return (
    <form action={formAction}>
      <Button type="submit" variant="danger" disabled={pending}>
        {pending ? "Refunding…" : "Refund"}
      </Button>
      {state?.error && (
        <p className="mt-2 text-sm text-[var(--danger)]">{state.error}</p>
      )}
    </form>
  );
}
