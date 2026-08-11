"use client";

import { ReactNode, useState } from "react";
import { LogOut } from "lucide-react";
import { Sidebar } from "./Sidebar";
import { CommandPalette } from "./CommandPalette";

type RecentIntent = { id: string; amount: string; currency: string };

export function DashboardShell({
  email,
  role,
  recentIntents,
  onLogout,
  children,
}: {
  email: string;
  role: string;
  recentIntents: RecentIntent[];
  onLogout: () => Promise<void>;
  children: ReactNode;
}) {
  const [paletteOpen, setPaletteOpen] = useState(false);

  return (
    <div className="flex min-h-screen">
      <Sidebar onOpenPalette={() => setPaletteOpen(true)} />

      <div className="flex flex-1 flex-col">
        <header className="flex items-center justify-end gap-3 border-b border-[var(--border)] px-6 py-3 text-sm text-[var(--muted)]">
          <span>
            {email} · <span className="font-medium">{role}</span>
          </span>
          <form action={onLogout}>
            <button
              type="submit"
              className="flex items-center gap-1.5 text-[var(--accent)] transition hover:opacity-80"
            >
              <LogOut className="size-3.5" strokeWidth={1.75} />
              Log out
            </button>
          </form>
        </header>
        <main className="mx-auto w-full max-w-5xl flex-1 px-6 py-8">
          {children}
        </main>
      </div>

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        recentIntents={recentIntents}
        onLogout={onLogout}
      />
    </div>
  );
}
