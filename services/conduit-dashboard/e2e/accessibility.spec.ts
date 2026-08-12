import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const CORE_URL = process.env.CONDUIT_CORE_URL ?? "http://localhost:8000";

// Every page under test here has a Framer Motion fade-in on mount
// (Reveal / Hero's own entrance animation) — scanning immediately after
// goto() catches elements mid-fade at partial opacity, which axe reports
// as a false-positive contrast failure since the sampled color is a
// blend, not the element's resting color. Settling first makes this an
// assertion about the real, stable page.
const ANIMATION_SETTLE_MS = 700;

// Automated coverage for the checklist docs/learning/awwwards-design-reference.md
// calls out as what flashy sites frequently get wrong: focus-visible states,
// color contrast, and semantic structure. Scoped to WCAG 2 A/AA rules —
// this asserts the standard, not just documents an intention to meet it.
test("landing page has no serious accessibility violations", async ({
  page,
}) => {
  await page.goto("/");
  await page.waitForTimeout(ANIMATION_SETTLE_MS);
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});

test("login page has no serious accessibility violations", async ({ page }) => {
  await page.goto("/login");
  await page.waitForTimeout(ANIMATION_SETTLE_MS);
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});

test("dashboard overview has no serious accessibility violations", async ({
  page,
}) => {
  const resp = await fetch(`${CORE_URL}/v1/merchants`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: `A11y Merchant ${Date.now()}` }),
  });
  const merchant = await resp.json();

  await page.goto("/signup");
  await page.getByLabel("Secret key").fill(merchant.secret_key);
  await page.getByLabel("Email").fill(`a11y-${Date.now()}@example.com`);
  await page.getByLabel("Password").fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);

  // The overview page's stat cards fade in staggered, up to ~0.35s of
  // delay on top of the base animation duration — a bit more margin
  // than the other two pages need.
  await page.waitForTimeout(1000);

  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .analyze();
  expect(results.violations).toEqual([]);
});
