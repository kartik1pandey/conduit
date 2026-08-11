"use server";

import { auth } from "@/auth";
import { hashPassword } from "@/lib/passwords";
import { can } from "@/lib/policy";
import { EmailAlreadyExistsError, inviteUser } from "@/lib/users";

export type InviteState = { error: string } | { ok: true } | undefined;

// Same enforcement rule as refund (see transactions/[id]/actions.ts): the
// role check happens here, server-side, against the verified session —
// never inferred from which page rendered a form.
export async function invite(
  _prevState: InviteState,
  formData: FormData,
): Promise<InviteState> {
  const session = await auth();
  if (!session) {
    return { error: "Not authenticated." };
  }

  const allowed = await can(session.user.role, "invite_user");
  if (!allowed) {
    return { error: "403: your role does not permit inviting teammates." };
  }

  const email = String(formData.get("email") ?? "")
    .trim()
    .toLowerCase();
  const role = String(formData.get("role") ?? "");
  const password = String(formData.get("password") ?? "");
  if (!email || (role !== "developer" && role !== "read-only")) {
    return { error: "A valid email and role are required." };
  }
  if (password && password.length < 8) {
    return {
      error:
        "Password must be at least 8 characters, or left blank for GitHub-only.",
    };
  }

  try {
    const passwordHash = password ? await hashPassword(password) : undefined;
    await inviteUser(session.user.merchantId, email, role, passwordHash);
  } catch (err) {
    if (err instanceof EmailAlreadyExistsError) {
      return { error: err.message };
    }
    return { error: "Could not invite that email." };
  }

  return { ok: true };
}
