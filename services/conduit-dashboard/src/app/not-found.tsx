import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-3 px-4 text-center">
      <p className="font-display text-sm font-medium text-[var(--accent)]">
        404
      </p>
      <h1 className="font-display text-2xl font-semibold">
        This page doesn&apos;t exist.
      </h1>
      <p className="max-w-sm text-sm text-[var(--muted)]">
        The link might be broken, or the page may have moved.
      </p>
      <Link
        href="/"
        className="mt-4 inline-flex items-center justify-center rounded-[var(--radius-sm)] bg-[var(--accent)] px-4 py-2 text-sm font-medium text-[var(--accent-foreground)] transition hover:opacity-90"
      >
        Back home
      </Link>
    </div>
  );
}
