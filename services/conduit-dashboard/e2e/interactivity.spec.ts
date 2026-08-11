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

async function signUpAsOwner(
  page: import("@playwright/test").Page,
  secretKey: string,
) {
  const email = `owner-${Date.now()}@example.com`;
  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(secretKey);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  return email;
}

// The actual thing this dashboard exists to prove: a merchant can create a
// payment, confirm it, and watch it flow through real risk scoring and a
// real ledger posting — entirely from the UI, never touching curl. This is
// the flow Checkpoint 4.4's "dashboard views" was supposed to make usable,
// not just readable.
test("creating and confirming a payment from the UI shows up everywhere it should", async ({
  page,
}) => {
  const merchant = await createMerchant(`Interactivity Merchant ${Date.now()}`);
  await signUpAsOwner(page, merchant.secret_key);

  await page.goto("/dashboard/transactions/new");
  await page.getByLabel("Amount").fill("42.00");
  await page.getByLabel("Description (optional)").fill("Playwright widget");
  await page.getByRole("button", { name: "Create payment intent" }).click();

  // Landed on the detail page for the newly created intent, in `created`
  // state, with a real Confirm button — not a redirect to a static
  // success screen.
  await expect(page).toHaveURL(/\/dashboard\/transactions\/[0-9a-f-]+$/);
  await expect(page.getByText("created", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Confirm payment" }).click();
  await expect(page.getByText("succeeded", { exact: true })).toBeVisible({
    timeout: 10_000,
  });

  // Shows up in the transactions list (the table shows amount/status/date,
  // not the description — checking what's actually rendered there).
  await page.goto("/dashboard/transactions");
  await expect(page.getByText("42.00 USD")).toBeVisible();

  // Shows up in the risk decisions view — confirming really did call
  // conduit-risk, not a stub.
  await page.goto("/dashboard/risk");
  await expect(page.getByText("allow")).toBeVisible();

  // Shows up on the overview page's stats and recent activity. Scoped to
  // the recent-activity row specifically — "42.00 USD" also legitimately
  // appears in the succeeded-volume card, so a bare text match is ambiguous.
  await page.goto("/dashboard");
  await expect(page.getByText("Succeeded volume")).toBeVisible();
  await expect(
    page.getByRole("link").filter({ hasText: "42.00 USD" }),
  ).toBeVisible();
});

test("registering a webhook endpoint from the UI shows the secret once and lists it", async ({
  page,
}) => {
  const merchant = await createMerchant(`Webhook UI Merchant ${Date.now()}`);
  await signUpAsOwner(page, merchant.secret_key);

  await page.goto("/dashboard/webhooks/new");
  await page
    .getByLabel("Endpoint URL")
    .fill("https://example.com/hooks/conduit");
  await page.getByRole("button", { name: "Register endpoint" }).click();

  await expect(page.getByText("won't be shown again")).toBeVisible();
  const secretBlock = page.getByTestId("webhook-secret");
  await expect(secretBlock).toContainText("whsec_");

  await page.getByRole("link", { name: "Back to webhook endpoints" }).click();
  await expect(page).toHaveURL(/\/dashboard\/webhooks$/);
  await expect(
    page.getByText("https://example.com/hooks/conduit"),
  ).toBeVisible();
});

// The same RBAC enforcement rule refund gets, extended to the two new
// mutating actions this pass added — a read-only session must not be able
// to create payments or register webhooks either.
test("a read-only session cannot create a payment", async ({ page }) => {
  const merchant = await createMerchant(
    `Interactivity RBAC Merchant ${Date.now()}`,
  );
  const readOnlyEmail = `viewer-${Date.now()}@example.com`;

  await signUpAsOwner(page, merchant.secret_key);

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
  await page.getByLabel("Email").fill(readOnlyEmail);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto("/dashboard/transactions/new");
  await page.getByLabel("Amount").fill("10.00");
  await page.getByRole("button", { name: "Create payment intent" }).click();
  await expect(
    page.getByText("403: your role does not permit creating payments."),
  ).toBeVisible();
});
