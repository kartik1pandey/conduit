import Link from "next/link";
import { Inbox, type LucideIcon } from "lucide-react";

// A dead end ("No X yet.") teaches a merchant nothing about what to do
// next — every list page in this app that can be empty points at the
// action that fills it instead.
export function EmptyState({
  title,
  description,
  actionHref,
  actionLabel,
  icon: Icon = Inbox,
}: {
  title: string;
  description: string;
  actionHref?: string;
  actionLabel?: string;
  icon?: LucideIcon;
}) {
  return (
    <div className="flex flex-col items-center gap-3 p-12 text-center">
      <div className="flex size-10 items-center justify-center rounded-full bg-[var(--surface-elevated)]">
        <Icon className="size-5 text-[var(--muted)]" strokeWidth={1.75} />
      </div>
      <p className="font-medium">{title}</p>
      <p className="max-w-sm text-sm text-[var(--muted)]">{description}</p>
      {actionHref && actionLabel && (
        <Link
          href={actionHref}
          className="mt-2 inline-flex items-center justify-center rounded-[var(--radius-sm)] bg-[var(--accent)] px-4 py-2 text-sm font-medium text-[var(--accent-foreground)] transition hover:opacity-90"
        >
          {actionLabel}
        </Link>
      )}
    </div>
  );
}
