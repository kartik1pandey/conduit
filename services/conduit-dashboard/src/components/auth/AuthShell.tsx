import Link from "next/link";
import { ReactNode } from "react";

// Split layout (branding panel + form) rather than a bare centered card
// on a flat background — the pattern Stripe/Mercury/Ramp all use for
// auth, and a cheap way to give the login/signup pages an identity
// instead of looking like an unstyled form.
export function AuthShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen">
      <div className="bg-grain relative hidden w-[42%] shrink-0 overflow-hidden bg-[var(--l-bg)] lg:flex lg:flex-col lg:justify-between lg:p-10">
        <div className="bg-mesh absolute inset-0 opacity-60" />
        <Link
          href="/"
          className="relative z-10 font-display text-lg font-semibold text-[var(--l-foreground)]"
        >
          Conduit
        </Link>
        <div className="relative z-10 max-w-sm">
          <p className="font-display text-2xl leading-snug text-[var(--l-foreground)]">
            Payment intents, real-time risk, and a double-entry ledger that
            always nets to zero.
          </p>
          <p className="mt-3 text-sm text-[var(--l-muted)]">
            Test mode. Every write is idempotent, every service authenticates
            with a signed JWT, and every ledger transaction is enforced balanced
            at the database layer.
          </p>
        </div>
      </div>
      <div className="flex flex-1 items-center justify-center px-4">
        {children}
      </div>
    </div>
  );
}
