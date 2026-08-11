"use server";

import { auth } from "@/auth";
import { createWebhookEndpoint } from "@/lib/coreClient";
import { can } from "@/lib/policy";

export type RegisterWebhookState =
  | { error: string }
  | { ok: true; secret: string }
  | undefined;

// Same enforcement shape as every other mutating action in this app: the
// role check happens here, server-side. The endpoint's HMAC secret is
// returned to the client exactly once (see conduit-webhooks'
// CreateEndpoint) — this action's job is to hand it back for display, not
// to store it anywhere on the dashboard's own side.
export async function registerWebhook(
  _prevState: RegisterWebhookState,
  formData: FormData,
): Promise<RegisterWebhookState> {
  const session = await auth();
  if (!session) {
    return { error: "Not authenticated." };
  }

  const allowed = await can(session.user.role, "manage_webhooks");
  if (!allowed) {
    return {
      error: "403: your role does not permit registering webhook endpoints.",
    };
  }

  const url = String(formData.get("url") ?? "").trim();
  const idempotencyKey = String(formData.get("idempotencyKey") ?? "");

  if (!url || !/^https?:\/\//.test(url)) {
    return { error: "URL must start with http:// or https://" };
  }
  if (!idempotencyKey) {
    return {
      error: "Missing idempotency key — reload the page and try again.",
    };
  }

  try {
    const endpoint = await createWebhookEndpoint(
      session.user.merchantId,
      idempotencyKey,
      url,
    );
    return { ok: true, secret: endpoint.secret };
  } catch (err) {
    return { error: `Could not register endpoint: ${(err as Error).message}` };
  }
}
