"use client";

import { useRouter } from "next/navigation";
import { useActionState, useEffect } from "react";
import { Button } from "@/components/ui/Button";
import { useActionToast } from "@/lib/useActionToast";
import { confirm, type ConfirmState } from "./actions";

export function ConfirmButton({
  paymentIntentId,
}: {
  paymentIntentId: string;
}) {
  const router = useRouter();
  const [state, formAction, pending] = useActionState<ConfirmState, FormData>(
    async (prevState) => confirm(paymentIntentId, prevState),
    undefined,
  );
  useActionToast(state, "Payment confirmed.");

  // Confirming is the interesting moment — risk scoring and the ledger
  // posting actually happen here. Refreshing the server-rendered detail
  // page (rather than just flashing a static "done" message) is what lets
  // the merchant see the real outcome: succeeded, or failed with the
  // actual decline reasons conduit-risk returned.
  useEffect(() => {
    if (state && "ok" in state) {
      router.refresh();
    }
  }, [state, router]);

  return (
    <form action={formAction}>
      <Button type="submit" disabled={pending}>
        {pending ? "Confirming…" : "Confirm payment"}
      </Button>
      {state && "error" in state && (
        <p className="mt-2 text-sm text-[var(--danger)]">{state.error}</p>
      )}
    </form>
  );
}
