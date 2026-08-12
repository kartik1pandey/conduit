"use client";

import Link from "next/link";
import { useActionState } from "react";
import { AuthShell } from "@/components/auth/AuthShell";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Reveal } from "@/components/ui/Reveal";
import { loginWithCredentials, loginWithGitHub } from "./actions";

export default function LoginPage() {
  const [state, formAction, pending] = useActionState(
    loginWithCredentials,
    undefined,
  );

  return (
    <AuthShell>
      <Reveal>
        <Card className="w-full max-w-sm p-8">
          <h1 className="font-display text-xl font-semibold">
            Log in to Conduit
          </h1>

          <form action={formAction} className="mt-6 space-y-4">
            <label className="block">
              <span className="mb-1 block text-sm font-medium">Email</span>
              <input
                name="email"
                type="email"
                required
                className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-sm font-medium">Password</span>
              <input
                name="password"
                type="password"
                required
                className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
              />
            </label>

            {state?.error && (
              <p className="text-sm text-[var(--danger)]">{state.error}</p>
            )}

            <Button type="submit" disabled={pending} className="w-full">
              {pending ? "Logging in…" : "Log in"}
            </Button>
          </form>

          <div className="my-4 flex items-center gap-3 text-xs text-[var(--muted)]">
            <div className="h-px flex-1 bg-[var(--border)]" />
            or
            <div className="h-px flex-1 bg-[var(--border)]" />
          </div>

          <form action={loginWithGitHub}>
            <Button type="submit" variant="secondary" className="w-full">
              Sign in with GitHub
            </Button>
          </form>

          <p className="mt-4 text-sm text-[var(--muted)]">
            Need to set up dashboard access?{" "}
            <Link href="/signup" className="text-[var(--accent)] underline">
              Claim your merchant
            </Link>
          </p>
        </Card>
      </Reveal>
    </AuthShell>
  );
}
