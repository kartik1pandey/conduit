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

test("signup claims a merchant via its secret key and lands on the dashboard", async ({
  page,
}) => {
  const merchant = await createMerchant(`Playwright Merchant ${Date.now()}`);
  const email = `owner-${Date.now()}@example.com`;

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByText(email).first()).toBeVisible();
  await expect(page.getByText(merchant.id)).toBeVisible();
});

test("login rejects a wrong password without revealing which field was wrong", async ({
  page,
}) => {
  const merchant = await createMerchant(`Playwright Merchant ${Date.now()}`);
  const email = `owner2-${Date.now()}@example.com`;

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("totally-wrong-password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByText("Invalid email or password.")).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});

test("an unauthenticated visitor hitting the dashboard directly is redirected to login", async ({
  page,
}) => {
  await page.goto("/dashboard");
  await expect(page).toHaveURL(/\/login$/);
});

test("signup rejects an invalid secret key", async ({ page }) => {
  await page.goto("/signup");
  await page.getByLabel("Secret key").fill("sk_test_this_does_not_exist");
  await page.getByLabel("Email").fill(`bogus-${Date.now()}@example.com`);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("That secret key isn't valid.")).toBeVisible();
  await expect(page).toHaveURL(/\/signup$/);
});
