import { SignJWT } from "jose";

// Mirrors conduit-core's internal/authn/dashboard_jwt.go exactly: a
// 60-second HS256 JWT carrying merchant_id, signed fresh per call rather
// than cached and reused — the same "minted fresh, never cached" rule
// SignInternalJWT/SignDashboardSession follow on the Go side, so a leaked
// token is only useful for a minute.
async function signDashboardSession(merchantId: string): Promise<string> {
  const secret = process.env.DASHBOARD_SESSION_SECRET;
  if (!secret) {
    throw new Error("DASHBOARD_SESSION_SECRET is required");
  }
  return new SignJWT({ merchant_id: merchantId })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime("60s")
    .sign(new TextEncoder().encode(secret));
}

function coreBaseUrl(): string {
  const url = process.env.CONDUIT_CORE_URL;
  if (!url) {
    throw new Error("CONDUIT_CORE_URL is required");
  }
  return url;
}

async function callCore<T>(
  merchantId: string,
  method: string,
  path: string,
  body?: unknown,
  extraHeaders?: Record<string, string>,
): Promise<T> {
  const token = await signDashboardSession(merchantId);
  const resp = await fetch(`${coreBaseUrl()}${path}`, {
    method,
    headers: {
      "X-Dashboard-Session": token,
      "Content-Type": "application/json",
      ...extraHeaders,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(
      `conduit-core ${method} ${path} returned ${resp.status}: ${text}`,
    );
  }
  if (resp.status === 204) {
    return undefined as T;
  }
  return resp.json() as Promise<T>;
}

export type PaymentIntent = {
  id: string;
  merchant_id: string;
  amount: string;
  currency: string;
  status: "created" | "pending" | "succeeded" | "failed" | "refunded";
  description: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
};

export async function listPaymentIntents(
  merchantId: string,
  opts: { limit?: number; offset?: number } = {},
): Promise<PaymentIntent[]> {
  const params = new URLSearchParams();
  if (opts.limit) params.set("limit", String(opts.limit));
  if (opts.offset) params.set("offset", String(opts.offset));
  const query = params.size ? `?${params}` : "";
  return callCore<PaymentIntent[]>(
    merchantId,
    "GET",
    `/v1/payment_intents${query}`,
  );
}

export async function getPaymentIntent(
  merchantId: string,
  paymentIntentId: string,
): Promise<PaymentIntent> {
  return callCore<PaymentIntent>(
    merchantId,
    "GET",
    `/v1/payment_intents/${paymentIntentId}`,
  );
}

// createPaymentIntent takes idempotencyKey as a caller-supplied argument
// rather than generating one internally — Stripe's own recommended pattern
// (and the one every write in this codebase follows): the key is minted
// once when the form is first rendered and carried through as a hidden
// field, so a double-click or a network retry of the SAME submission
// reuses the SAME key and replays core's original response, while two
// genuinely separate visits to "New Payment" get two different keys and
// create two different payment intents, exactly as intended.
export async function createPaymentIntent(
  merchantId: string,
  idempotencyKey: string,
  input: { amount: string; currency: string; description?: string },
): Promise<PaymentIntent> {
  return callCore<PaymentIntent>(
    merchantId,
    "POST",
    "/v1/payment_intents",
    input,
    { "Idempotency-Key": idempotencyKey },
  );
}

// confirmPaymentIntent's key is deterministic per payment intent, same
// reasoning as refundPaymentIntent below — confirming is conceptually a
// single action on an existing resource, not a new one each time.
export async function confirmPaymentIntent(
  merchantId: string,
  paymentIntentId: string,
): Promise<PaymentIntent> {
  return callCore<PaymentIntent>(
    merchantId,
    "POST",
    `/v1/payment_intents/${paymentIntentId}/confirm`,
    {},
    { "Idempotency-Key": `dashboard:confirm:${paymentIntentId}` },
  );
}

// refundPaymentIntent's Idempotency-Key is deterministic per payment
// intent, not randomized per call — every write endpoint in this project
// requires one (CLAUDE.md non-negotiables), and the whole point is that a
// double-click or a retried request reuses the SAME key and gets back
// core's cached original response instead of being reprocessed. (The
// ledger posting itself is separately protected by its own deterministic
// key inside postRefundToLedger regardless of what key this call sends —
// this key governs HTTP-level replay, not the financial safety property.)
export async function refundPaymentIntent(
  merchantId: string,
  paymentIntentId: string,
): Promise<PaymentIntent> {
  return callCore<PaymentIntent>(
    merchantId,
    "POST",
    `/v1/payment_intents/${paymentIntentId}/refund`,
    {},
    { "Idempotency-Key": `dashboard:refund:${paymentIntentId}` },
  );
}

export type WebhookEndpoint = {
  id: string;
  merchant_id: string;
  url: string;
  created_at: string;
};

export type WebhookEndpointWithSecret = WebhookEndpoint & { secret: string };

// createWebhookEndpoint's idempotency key follows the same "minted at form
// render, carried as a hidden field" pattern as createPaymentIntent — see
// its comment above.
export async function createWebhookEndpoint(
  merchantId: string,
  idempotencyKey: string,
  url: string,
): Promise<WebhookEndpointWithSecret> {
  return callCore<WebhookEndpointWithSecret>(
    merchantId,
    "POST",
    "/v1/webhook_endpoints",
    { url },
    { "Idempotency-Key": idempotencyKey },
  );
}

export async function listWebhookEndpoints(
  merchantId: string,
): Promise<WebhookEndpoint[]> {
  return callCore<WebhookEndpoint[]>(
    merchantId,
    "GET",
    "/v1/webhook_endpoints",
  );
}

export type WebhookDelivery = {
  id: string;
  webhook_event_id: string;
  webhook_endpoint_id: string;
  status: "pending" | "delivered" | "retrying" | "dead_lettered";
  attempt_count: number;
  last_response_status?: number;
  created_at: string;
};

export async function listWebhookDeliveries(
  merchantId: string,
  endpointId: string,
): Promise<WebhookDelivery[]> {
  return callCore<WebhookDelivery[]>(
    merchantId,
    "GET",
    `/v1/webhook_endpoints/${endpointId}/deliveries`,
  );
}

export type RiskDecision = {
  id: string;
  payment_intent_id: string;
  amount: string;
  currency: string;
  decision: "allow" | "decline";
  risk_score: number;
  stage: "rules" | "model";
  reasons: string[];
  created_at: string;
};

export async function listRiskDecisions(
  merchantId: string,
): Promise<RiskDecision[]> {
  return callCore<RiskDecision[]>(merchantId, "GET", "/v1/risk_decisions");
}
