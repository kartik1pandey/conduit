"use server";

import { AuthError } from "next-auth";
import { signIn } from "@/auth";

export type LoginState = { error: string } | undefined;

export async function loginWithCredentials(
  _prevState: LoginState,
  formData: FormData,
): Promise<LoginState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");

  try {
    await signIn("credentials", { email, password, redirectTo: "/dashboard" });
  } catch (err) {
    // AuthError covers a rejected authorize() call (wrong email/password) —
    // anything else, including the redirect Next.js throws internally on
    // success, must propagate rather than be swallowed here.
    if (err instanceof AuthError) {
      return { error: "Invalid email or password." };
    }
    throw err;
  }
}

export async function loginWithGitHub() {
  await signIn("github", { redirectTo: "/dashboard" });
}
