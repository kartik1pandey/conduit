"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/Button";
import { refund, type RefundState } from "./actions";

export function RefundButton({ paymentIntentId }: { paymentIntentId: string }) {
  const [state, formAction, pending] = useActionState<RefundState, FormData>(
    async (prevState) => refund(paymentIntentId, prevState),
    undefined,
  );

  if (state && "ok" in state) {
    return (
      <p className="text-sm text-emerald-600 dark:text-emerald-400">
        Refund issued.
      </p>
    );
  }

  return (
    <form action={formAction}>
      <Button type="submit" variant="danger" disabled={pending}>
        {pending ? "Refunding…" : "Refund"}
      </Button>
      {state?.error && (
        <p className="mt-2 text-sm text-rose-500">{state.error}</p>
      )}
    </form>
  );
}
