import Link from "next/link";

// A dead end ("No X yet.") teaches a merchant nothing about what to do
// next — every list page in this app that can be empty points at the
// action that fills it instead.
export function EmptyState({
  title,
  description,
  actionHref,
  actionLabel,
}: {
  title: string;
  description: string;
  actionHref?: string;
  actionLabel?: string;
}) {
  return (
    <div className="flex flex-col items-center gap-3 p-12 text-center">
      <p className="font-medium">{title}</p>
      <p className="max-w-sm text-sm text-[var(--muted)]">{description}</p>
      {actionHref && actionLabel && (
        <Link
          href={actionHref}
          className="mt-2 inline-flex items-center justify-center rounded-lg bg-[var(--accent)] px-4 py-2 text-sm font-medium text-white transition hover:opacity-90"
        >
          {actionLabel}
        </Link>
      )}
    </div>
  );
}
