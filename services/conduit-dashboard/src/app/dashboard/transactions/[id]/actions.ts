"use server";

import { revalidatePath } from "next/cache";
import { auth } from "@/auth";
import { confirmPaymentIntent, refundPaymentIntent } from "@/lib/coreClient";
import { can } from "@/lib/policy";

export type RefundState = { error: string } | { ok: true } | undefined;
export type ConfirmState = { error: string } | { ok: true } | undefined;

// confirm is what actually drives the risk-scoring + ledger-posting flow
// live — gated under the same "create_payment" policy action as creating
// the payment intent in the first place, since both are steps in the same
// merchant-initiated workflow, not independently privileged operations.
export async function confirm(
  paymentIntentId: string,
  _prevState: ConfirmState,
): Promise<ConfirmState> {
  const session = await auth();
  if (!session) {
    return { error: "Not authenticated." };
  }

  const allowed = await can(session.user.role, "create_payment");
  if (!allowed) {
    return { error: "403: your role does not permit confirming payments." };
  }

  try {
    await confirmPaymentIntent(session.user.merchantId, paymentIntentId);
  } catch (err) {
    return { error: `Could not confirm: ${(err as Error).message}` };
  }

  // The detail page below needs to re-fetch and show the post-confirm
  // status (succeeded/failed + risk reasons) rather than the stale
  // "created" snapshot the page originally rendered with.
  revalidatePath(`/dashboard/transactions/${paymentIntentId}`);
  return { ok: true };
}

// refund is the ACTUAL enforcement point for Checkpoint 4.3 ("a read-only
// session gets 403 on a refund action") — not the Button being disabled in
// the UI, which a read-only user could bypass by hitting this action
// directly (browser devtools, a raw POST, a stale cached page rendered
// before a role change). The role check happens here, server-side, against
// the session Auth.js verified, every single time — never trusted from
// anything the client sends.
export async function refund(
  paymentIntentId: string,
  _prevState: RefundState,
): Promise<RefundState> {
  const session = await auth();
  if (!session) {
    return { error: "Not authenticated." };
  }

  const allowed = await can(session.user.role, "refund");
  if (!allowed) {
    return { error: "403: your role does not permit refunds." };
  }

  try {
    await refundPaymentIntent(session.user.merchantId, paymentIntentId);
  } catch (err) {
    return { error: `Could not refund: ${(err as Error).message}` };
  }

  return { ok: true };
}
