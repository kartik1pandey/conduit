"use server";

import { redirect } from "next/navigation";
import { auth } from "@/auth";
import { createPaymentIntent } from "@/lib/coreClient";
import { can } from "@/lib/policy";

export type NewPaymentState = { error: string } | undefined;

// Same enforcement shape as refund (see transactions/[id]/actions.ts):
// the role check happens here, server-side, against the verified session
// — the idempotency key comes from a hidden form field seeded once when
// the page rendered (see page.tsx), not generated fresh per submit.
export async function createPayment(
  _prevState: NewPaymentState,
  formData: FormData,
): Promise<NewPaymentState> {
  const session = await auth();
  if (!session) {
    return { error: "Not authenticated." };
  }

  const allowed = await can(session.user.role, "create_payment");
  if (!allowed) {
    return { error: "403: your role does not permit creating payments." };
  }

  const amount = String(formData.get("amount") ?? "").trim();
  const currency = String(formData.get("currency") ?? "")
    .trim()
    .toLowerCase();
  const description = String(formData.get("description") ?? "").trim();
  const idempotencyKey = String(formData.get("idempotencyKey") ?? "");

  if (!amount || Number.isNaN(Number(amount)) || Number(amount) <= 0) {
    return { error: "Amount must be a positive number." };
  }
  if (!currency) {
    return { error: "Currency is required." };
  }
  if (!idempotencyKey) {
    return {
      error: "Missing idempotency key — reload the page and try again.",
    };
  }

  let paymentIntentId: string;
  try {
    const pi = await createPaymentIntent(
      session.user.merchantId,
      idempotencyKey,
      {
        amount,
        currency,
        description: description || undefined,
      },
    );
    paymentIntentId = pi.id;
  } catch (err) {
    return { error: `Could not create payment: ${(err as Error).message}` };
  }

  redirect(`/dashboard/transactions/${paymentIntentId}`);
}
