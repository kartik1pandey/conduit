import { expect, test } from "@playwright/test";

const CORE_URL = process.env.CONDUIT_CORE_URL ?? "http://localhost:8000";

async function createMerchant(
  name: string,
): Promise<{ id: string; secret_key: string }> {
  const resp = await fetch(`${CORE_URL}/v1/merchants`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  return resp.json();
}

async function createAndConfirmPaymentIntent(
  secretKey: string,
): Promise<string> {
  const createResp = await fetch(`${CORE_URL}/v1/payment_intents`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${secretKey}`,
      "Content-Type": "application/json",
      "Idempotency-Key": crypto.randomUUID(),
    },
    body: JSON.stringify({ amount: "10.00", currency: "usd" }),
  });
  const { id } = await createResp.json();

  await fetch(`${CORE_URL}/v1/payment_intents/${id}/confirm`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${secretKey}`,
      "Content-Type": "application/json",
      "Idempotency-Key": crypto.randomUUID(),
    },
    body: "{}",
  });

  return id;
}

// Checkpoint 4.4's exact verification: "merchant A's dashboard never shows
// merchant B's data." Both merchants' payment intents exist in the same
// conduit-core database at the same time; the only thing standing between
// merchant A's browser session and merchant B's rows is the merchant_id
// carried in the signed dashboard-session JWT (see
// authn.RequireMerchantContext on the core side).
test("merchant A's dashboard shows its own transaction and never merchant B's", async ({
  page,
}) => {
  const merchantA = await createMerchant(`Tenant A ${Date.now()}`);
  const merchantB = await createMerchant(`Tenant B ${Date.now()}`);

  const piIdA = await createAndConfirmPaymentIntent(merchantA.secret_key);
  const piIdB = await createAndConfirmPaymentIntent(merchantB.secret_key);

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchantA.secret_key);
  await page.getByLabel("Email").fill(`tenant-a-${Date.now()}@example.com`);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto("/dashboard/transactions");
  await expect(page.getByText(piIdA)).toBeVisible();
  await expect(page.getByText(piIdB)).not.toBeVisible();

  // Directly hitting merchant A's transaction detail page by merchant B's
  // payment intent ID must not leak merchant B's data either — the lookup
  // is scoped by the session's own merchant_id, not by whatever id
  // appears in the URL.
  await page.goto(`/dashboard/transactions/${piIdB}`);
  const status = await page.request
    .get(`/dashboard/transactions/${piIdB}`)
    .then((r) => r.status());
  expect(status).not.toBe(200);
});
