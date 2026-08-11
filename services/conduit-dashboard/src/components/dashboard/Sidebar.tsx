"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Home,
  Receipt,
  Webhook,
  ShieldCheck,
  Users,
  Command as CommandIcon,
} from "lucide-react";

const NAV_ITEMS = [
  { href: "/dashboard", label: "Overview", icon: Home },
  { href: "/dashboard/transactions", label: "Transactions", icon: Receipt },
  { href: "/dashboard/webhooks", label: "Webhooks", icon: Webhook },
  { href: "/dashboard/risk", label: "Risk", icon: ShieldCheck },
  { href: "/dashboard/team", label: "Team", icon: Users },
];

export function Sidebar({ onOpenPalette }: { onOpenPalette: () => void }) {
  const pathname = usePathname();

  return (
    <aside className="flex h-screen w-60 shrink-0 flex-col border-r border-[var(--border)] bg-[var(--surface)]">
      <div className="flex items-center gap-2 px-5 py-5">
        <div className="flex size-7 items-center justify-center rounded-[var(--radius-sm)] bg-[var(--accent)] font-display text-sm font-semibold text-[var(--accent-foreground)]">
          C
        </div>
        <span className="font-display text-lg font-semibold">Conduit</span>
      </div>

      <nav className="flex-1 space-y-1 px-3">
        {NAV_ITEMS.map((item) => {
          const active =
            item.href === "/dashboard"
              ? pathname === "/dashboard"
              : pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-sm font-medium transition ${
                active
                  ? "bg-[var(--surface-elevated)] text-[var(--foreground)]"
                  : "text-[var(--muted)] hover:bg-[var(--surface-elevated)] hover:text-[var(--foreground)]"
              }`}
            >
              <item.icon className="size-4" strokeWidth={1.75} />
              {item.label}
            </Link>
          );
        })}
      </nav>

      <button
        type="button"
        onClick={onOpenPalette}
        className="mx-3 mb-5 flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] px-3 py-2 text-xs text-[var(--muted)] transition hover:bg-[var(--surface-elevated)]"
      >
        <span className="flex items-center gap-2">
          <CommandIcon className="size-3.5" strokeWidth={1.75} />
          Quick jump
        </span>
        <kbd className="rounded border border-[var(--border)] bg-[var(--surface-elevated)] px-1.5 py-0.5 font-mono text-[10px]">
          ⌘K
        </kbd>
      </button>
    </aside>
  );
}
