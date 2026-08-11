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

// Checkpoint 4.3's exact verification: "a read-only session gets 403 on a
// refund action." Signs up an owner, invites a read-only teammate with a
// password (so this test can log in as them without a real GitHub OAuth
// flow), then drives the actual refund button as that read-only user.
test("a read-only session gets 403 on a refund action", async ({ page }) => {
  const merchant = await createMerchant(`RBAC Merchant ${Date.now()}`);
  const ownerEmail = `owner-${Date.now()}@example.com`;
  const readOnlyEmail = `viewer-${Date.now()}@example.com`;

  const piId = await createAndConfirmPaymentIntent(merchant.secret_key);

  // Owner signs up (claims the merchant) and invites a read-only teammate.
  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(ownerEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto("/dashboard/team");
  await page.getByLabel("Email").fill(readOnlyEmail);
  await page.getByLabel("Role").selectOption("read-only");
  await page
    .getByLabel("Password (optional)")
    .fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByText("Invited.")).toBeVisible();

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login$/);

  // Log in as the read-only teammate and attempt the refund directly.
  await page.getByLabel("Email").fill(readOnlyEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto(`/dashboard/transactions/${piId}`);
  await page.getByRole("button", { name: "Refund" }).click();
  await expect(
    page.getByText("403: your role does not permit refunds."),
  ).toBeVisible();

  // And the payment intent must still read succeeded, not refunded — the
  // rejected action must never have reached conduit-core at all.
  const resp = await fetch(`${CORE_URL}/v1/payment_intents/${piId}`, {
    headers: { Authorization: `Bearer ${merchant.secret_key}` },
  });
  const pi = await resp.json();
  expect(pi.status).toBe("succeeded");
});

// The other half of the same policy: owner and developer CAN refund.
test("an owner session can refund a succeeded payment intent", async ({
  page,
}) => {
  const merchant = await createMerchant(`RBAC Owner Merchant ${Date.now()}`);
  const ownerEmail = `owner2-${Date.now()}@example.com`;
  const piId = await createAndConfirmPaymentIntent(merchant.secret_key);

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(ownerEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto(`/dashboard/transactions/${piId}`);
  await page.getByRole("button", { name: "Refund" }).click();
  await expect(page.getByText("Refund issued.")).toBeVisible();
});

// Checkpoint 4.3's other half: a non-owner cannot invite teammates either.
test("a developer cannot invite a teammate", async ({ page }) => {
  const merchant = await createMerchant(`RBAC Dev Merchant ${Date.now()}`);
  const ownerEmail = `owner3-${Date.now()}@example.com`;
  const devEmail = `dev-${Date.now()}@example.com`;

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(ownerEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto("/dashboard/team");
  await page.getByLabel("Email").fill(devEmail);
  await page.getByLabel("Role").selectOption("developer");
  await page
    .getByLabel("Password (optional)")
    .fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByText("Invited.")).toBeVisible();

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("Email").fill(devEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  // A developer can view the team page but the invite form must be absent
  // entirely — enforced server-side (page.tsx checks can(role, "invite_user")
  // before ever rendering <InviteForm>), not just hidden with CSS.
  await page.goto("/dashboard/team");
  await expect(page.getByRole("button", { name: "Invite" })).toHaveCount(0);
});
