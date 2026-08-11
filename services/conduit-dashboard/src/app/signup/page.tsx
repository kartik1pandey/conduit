"use client";

import Link from "next/link";
import { useActionState } from "react";
import { AuthShell } from "@/components/auth/AuthShell";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Reveal } from "@/components/ui/Reveal";
import { signup } from "./actions";

export default function SignupPage() {
  const [state, formAction, pending] = useActionState(signup, undefined);

  return (
    <AuthShell>
      <Reveal>
        <Card className="w-full max-w-sm p-8">
          <h1 className="font-display text-xl font-semibold">
            Set up your dashboard
          </h1>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Enter your merchant secret key once to claim owner access.
          </p>

          <form action={formAction} className="mt-6 space-y-4">
            <Field
              label="Secret key"
              name="secretKey"
              type="password"
              placeholder="sk_test_..."
            />
            <Field label="Email" name="email" type="email" />
            <Field label="Password" name="password" type="password" />

            {state?.error && (
              <p className="text-sm text-[var(--danger)]">{state.error}</p>
            )}

            <Button type="submit" disabled={pending} className="w-full">
              {pending ? "Creating account…" : "Create account"}
            </Button>
          </form>

          <p className="mt-4 text-sm text-[var(--muted)]">
            Already have an account?{" "}
            <Link href="/login" className="text-[var(--accent)]">
              Log in
            </Link>
          </p>
        </Card>
      </Reveal>
    </AuthShell>
  );
}

function Field({
  label,
  name,
  type,
  placeholder,
}: {
  label: string;
  name: string;
  type: string;
  placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium">{label}</span>
      <input
        name={name}
        type={type}
        required
        placeholder={placeholder}
        className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
      />
    </label>
  );
}
