"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Command } from "cmdk";
import {
  Home,
  Receipt,
  Webhook,
  ShieldCheck,
  Users,
  Plus,
  LogOut,
} from "lucide-react";
import { formatMoney } from "@/lib/format";

type RecentIntent = { id: string; amount: string; currency: string };

const NAV_COMMANDS = [
  { label: "Overview", href: "/dashboard", icon: Home },
  { label: "Transactions", href: "/dashboard/transactions", icon: Receipt },
  { label: "New payment", href: "/dashboard/transactions/new", icon: Plus },
  { label: "Webhooks", href: "/dashboard/webhooks", icon: Webhook },
  { label: "Register webhook", href: "/dashboard/webhooks/new", icon: Plus },
  { label: "Risk decisions", href: "/dashboard/risk", icon: ShieldCheck },
  { label: "Team", href: "/dashboard/team", icon: Users },
];

// A cmd+k palette earns its place in a payments dashboard specifically
// because merchants jump between a payment intent's detail page and its
// webhook deliveries/risk decision constantly — this makes that a
// keystroke instead of three clicks through list pages.
export function CommandPalette({
  open,
  onOpenChange,
  recentIntents,
  onLogout,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  recentIntents: RecentIntent[];
  onLogout: () => Promise<void>;
}) {
  const router = useRouter();

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onOpenChange(!open);
      }
      if (e.key === "Escape") onOpenChange(false);
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [open, onOpenChange]);

  function go(href: string) {
    onOpenChange(false);
    router.push(href);
  }

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 pt-[15vh]"
      onClick={() => onOpenChange(false)}
    >
      <Command
        className="w-full max-w-md overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border)] bg-[var(--surface)] shadow-2xl"
        onClick={(e) => e.stopPropagation()}
        loop
      >
        <Command.Input
          autoFocus
          placeholder="Jump to a page, or paste a payment intent ID…"
          className="w-full border-b border-[var(--border)] bg-transparent px-4 py-3 text-sm outline-none placeholder:text-[var(--muted)]"
        />
        <Command.List className="max-h-80 overflow-y-auto p-2">
          <Command.Empty className="p-4 text-center text-sm text-[var(--muted)]">
            No matches.
          </Command.Empty>

          <Command.Group
            heading="Navigate"
            className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-[var(--muted)]"
          >
            {NAV_COMMANDS.map((c) => (
              <Command.Item
                key={c.href}
                onSelect={() => go(c.href)}
                className="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] px-2 py-2 text-sm data-[selected=true]:bg-[var(--surface-elevated)]"
              >
                <c.icon
                  className="size-4 text-[var(--muted)]"
                  strokeWidth={1.75}
                />
                {c.label}
              </Command.Item>
            ))}
          </Command.Group>

          {recentIntents.length > 0 && (
            <Command.Group
              heading="Recent payment intents"
              className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-[var(--muted)]"
            >
              {recentIntents.map((pi) => (
                <Command.Item
                  key={pi.id}
                  value={pi.id}
                  onSelect={() => go(`/dashboard/transactions/${pi.id}`)}
                  className="flex cursor-pointer items-center justify-between rounded-[var(--radius-sm)] px-2 py-2 text-sm data-[selected=true]:bg-[var(--surface-elevated)]"
                >
                  <span className="font-mono text-xs text-[var(--muted)]">
                    {pi.id}
                  </span>
                  <span className="tabular-nums">
                    {formatMoney(pi.amount, pi.currency)}
                  </span>
                </Command.Item>
              ))}
            </Command.Group>
          )}

          <Command.Group
            heading="Session"
            className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5 [&_[cmdk-group-heading]]:text-xs [&_[cmdk-group-heading]]:text-[var(--muted)]"
          >
            <Command.Item
              onSelect={async () => {
                onOpenChange(false);
                await onLogout();
              }}
              className="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] px-2 py-2 text-sm data-[selected=true]:bg-[var(--surface-elevated)]"
            >
              <LogOut
                className="size-4 text-[var(--muted)]"
                strokeWidth={1.75}
              />
              Log out
            </Command.Item>
          </Command.Group>
        </Command.List>
      </Command>
    </div>
  );
}
