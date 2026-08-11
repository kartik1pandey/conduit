import Link from "next/link";
import { redirect } from "next/navigation";
import { auth } from "@/auth";
import { logout } from "./actions";

const NAV_ITEMS = [
  { href: "/dashboard", label: "Overview" },
  { href: "/dashboard/transactions", label: "Transactions" },
  { href: "/dashboard/webhooks", label: "Webhooks" },
  { href: "/dashboard/risk", label: "Risk" },
  { href: "/dashboard/team", label: "Team" },
];

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();
  if (!session) {
    redirect("/login");
  }

  return (
    <div className="min-h-screen">
      <nav className="border-b border-[var(--border)] bg-[var(--surface)]">
        <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
          <div className="flex items-center gap-6">
            <span className="font-semibold">Conduit</span>
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="text-sm text-[var(--muted)] transition hover:text-[var(--foreground)]"
              >
                {item.label}
              </Link>
            ))}
          </div>
          <div className="flex items-center gap-3 text-sm text-[var(--muted)]">
            <span>
              {session.user.email} ·{" "}
              <span className="font-medium">{session.user.role}</span>
            </span>
            <form action={logout}>
              <button
                type="submit"
                className="text-[var(--accent)] hover:underline"
              >
                Log out
              </button>
            </form>
          </div>
        </div>
      </nav>
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </div>
  );
}
