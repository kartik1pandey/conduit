"use server";

import { signIn } from "@/auth";
import { hashPassword } from "@/lib/passwords";
import { createOwner, getUserByEmail } from "@/lib/users";

export type SignupState = { error: string } | undefined;

// signup is Checkpoint 4.3's account-provisioning path: proving ownership
// of a merchant's sk_test_... key (via conduit-core's verify-secret
// endpoint) is what authorizes creating the FIRST dashboard user for that
// merchant, always as "owner" — see internal/merchant/handlers.go's
// verifySecret for the other half of this contract. The raw secret key is
// used once here and never stored; only the resulting merchant_id is kept.
export async function signup(
  _prevState: SignupState,
  formData: FormData,
): Promise<SignupState> {
  const secretKey = String(formData.get("secretKey") ?? "").trim();
  const email = String(formData.get("email") ?? "")
    .trim()
    .toLowerCase();
  const password = String(formData.get("password") ?? "");

  if (!secretKey || !email || !password) {
    return { error: "All fields are required." };
  }
  if (password.length < 8) {
    return { error: "Password must be at least 8 characters." };
  }

  const coreBaseUrl = process.env.CONDUIT_CORE_URL;
  if (!coreBaseUrl) {
    return { error: "Server misconfigured: CONDUIT_CORE_URL is not set." };
  }

  if (await getUserByEmail(email)) {
    return {
      error: "An account with this email already exists — log in instead.",
    };
  }

  let merchantId: string;
  try {
    const resp = await fetch(`${coreBaseUrl}/v1/merchants/verify-secret`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ secret_key: secretKey }),
    });
    if (!resp.ok) {
      return {
        error: "That secret key isn't valid. Double-check it and try again.",
      };
    }
    const body = (await resp.json()) as { merchant_id: string };
    merchantId = body.merchant_id;
  } catch {
    return { error: "Could not reach conduit-core. Try again in a moment." };
  }

  const passwordHash = await hashPassword(password);
  await createOwner(merchantId, email, passwordHash);

  await signIn("credentials", { email, password, redirectTo: "/dashboard" });
}
