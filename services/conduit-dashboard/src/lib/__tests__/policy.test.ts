import { describe, expect, it } from "vitest";
import { can } from "@/lib/policy";

const skip = !process.env.DASHBOARD_OPA_URL;

// Distinct from policies/dashboard_test.rego's native `opa test` coverage:
// those verify the policy itself computes the right decision in isolation.
// This verifies lib/policy.ts's HTTP client actually talks to a real OPA
// server correctly (right endpoint, right input shape, right response
// parsing) — the same "both are worth having" rationale conduit-risk's
// app/policy.py tests follow for policies/risk.rego.
describe.skipIf(skip)("policy.can", () => {
  it("read-only cannot refund", async () => {
    expect(await can("read-only", "refund")).toBe(false);
  });

  it("owner can refund", async () => {
    expect(await can("owner", "refund")).toBe(true);
  });

  it("developer can refund but not invite_user", async () => {
    expect(await can("developer", "refund")).toBe(true);
    expect(await can("developer", "invite_user")).toBe(false);
  });

  it("every role can view", async () => {
    expect(await can("owner", "view")).toBe(true);
    expect(await can("developer", "view")).toBe(true);
    expect(await can("read-only", "view")).toBe(true);
  });
});
