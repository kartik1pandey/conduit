import type { Role } from "@/lib/users";

export type Action = "view" | "refund" | "invite_user";

export class PolicyError extends Error {}

// can asks the real OPA server evaluating policies/dashboard.rego whether
// role is allowed to perform action — the same "ask OPA, don't
// reimplement the decision in application code" pattern conduit-risk's
// app/policy.py established for risk.rego. This is the ONLY place refund
// eligibility is decided; a UI element being hidden for a read-only user
// is a convenience, not the enforcement point (see
// dashboard/transactions/[id]/actions.ts).
export async function can(role: Role, action: Action): Promise<boolean> {
  const opaUrl = process.env.DASHBOARD_OPA_URL;
  if (!opaUrl) {
    throw new PolicyError("DASHBOARD_OPA_URL is required");
  }

  let resp: Response;
  try {
    resp = await fetch(`${opaUrl}/v1/data/conduit/dashboard/allow`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ input: { role, action } }),
    });
  } catch (err) {
    throw new PolicyError(`could not reach OPA: ${(err as Error).message}`);
  }
  if (!resp.ok) {
    throw new PolicyError(`OPA returned ${resp.status}`);
  }

  const body = (await resp.json()) as { result?: boolean };
  return body.result === true;
}
