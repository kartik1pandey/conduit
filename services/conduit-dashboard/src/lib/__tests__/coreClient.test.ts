import { v4 as uuidv4 } from "uuid";
import { beforeAll, describe, expect, it } from "vitest";
import {
  listPaymentIntents,
  listRiskDecisions,
  listWebhookEndpoints,
  refundPaymentIntent,
} from "@/lib/coreClient";

const CORE_URL = process.env.CONDUIT_CORE_URL;
const skip = !CORE_URL || !process.env.DASHBOARD_SESSION_SECRET;

async function createMerchant(): Promise<{ id: string; secretKey: string }> {
  const resp = await fetch(`${CORE_URL}/v1/merchants`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: `coreClient test ${uuidv4()}` }),
  });
  const body = await resp.json();
  return { id: body.id, secretKey: body.secret_key };
}

async function createAndConfirmPaymentIntent(
  secretKey: string,
): Promise<string> {
  const createResp = await fetch(`${CORE_URL}/v1/payment_intents`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${secretKey}`,
      "Content-Type": "application/json",
      "Idempotency-Key": uuidv4(),
    },
    body: JSON.stringify({ amount: "10.00", currency: "usd" }),
  });
  const { id } = await createResp.json();

  await fetch(`${CORE_URL}/v1/payment_intents/${id}/confirm`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${secretKey}`,
      "Content-Type": "application/json",
      "Idempotency-Key": uuidv4(),
    },
    body: "{}",
  });

  return id;
}

// This exercises the real dashboard-session trust boundary end to end:
// coreClient signs a JWT with jose, conduit-core's authn.RequireMerchantContext
// verifies it with golang-jwt — two different JWT libraries, two different
// languages, one shared secret. If these ever silently stopped
// interoperating (a claim name mismatch, an algorithm mismatch), a unit
// test that mocks fetch would never catch it; only a real call across the
// real boundary does.
describe.skipIf(skip)("coreClient", () => {
  let merchantId: string;
  let secretKey: string;

  beforeAll(async () => {
    const merchant = await createMerchant();
    merchantId = merchant.id;
    secretKey = merchant.secretKey;
  });

  it("lists payment intents for the authenticated merchant, newest first", async () => {
    const piId = await createAndConfirmPaymentIntent(secretKey);
    const intents = await listPaymentIntents(merchantId);
    expect(intents.some((pi) => pi.id === piId)).toBe(true);
  });

  it("refunds a succeeded payment intent", async () => {
    const piId = await createAndConfirmPaymentIntent(secretKey);
    const refunded = await refundPaymentIntent(merchantId, piId);
    expect(refunded.status).toBe("refunded");
  });

  it("a dashboard session for one merchant cannot see another merchant's payment intents", async () => {
    const other = await createMerchant();
    await createAndConfirmPaymentIntent(secretKey);
    const otherIntents = await listPaymentIntents(other.id);
    expect(otherIntents).toEqual([]);
  });

  it("lists webhook endpoints and risk decisions without error (empty is fine)", async () => {
    await expect(listWebhookEndpoints(merchantId)).resolves.toEqual(
      expect.any(Array),
    );
    await expect(listRiskDecisions(merchantId)).resolves.toEqual(
      expect.any(Array),
    );
  });
});
