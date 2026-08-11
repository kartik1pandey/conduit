"use server";

import { auth } from "@/auth";
import { refundPaymentIntent } from "@/lib/coreClient";
import { can } from "@/lib/policy";

export type RefundState = { error: string } | { ok: true } | undefined;

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
