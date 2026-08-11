import Link from "next/link";

export function Nav() {
  return (
    <header className="sticky top-0 z-40 border-b border-[var(--l-border)] bg-[var(--l-bg)]/80 backdrop-blur-md">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <span className="font-display text-lg font-semibold text-[var(--l-foreground)]">
          Conduit
        </span>
        <nav className="flex items-center gap-6">
          <a
            href="#flow"
            className="hidden text-sm text-[var(--l-muted)] transition hover:text-[var(--l-foreground)] sm:inline"
          >
            How it works
          </a>
          <Link
            href="/login"
            className="text-sm text-[var(--l-muted)] transition hover:text-[var(--l-foreground)]"
          >
            Sign in
          </Link>
          <Link
            href="/signup"
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] bg-[var(--l-accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
          >
            Get started
          </Link>
        </nav>
      </div>
    </header>
  );
}
