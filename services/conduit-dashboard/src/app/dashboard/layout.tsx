import { redirect } from "next/navigation";
import { auth } from "@/auth";
import { listPaymentIntents } from "@/lib/coreClient";
import { DashboardShell } from "@/components/dashboard/DashboardShell";
import { logout } from "./actions";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();
  if (!session) {
    redirect("/login");
  }

  const recent = await listPaymentIntents(session.user.merchantId, {
    limit: 8,
  });

  return (
    <DashboardShell
      email={session.user.email ?? ""}
      role={session.user.role}
      recentIntents={recent.map((pi) => ({
        id: pi.id,
        amount: pi.amount,
        currency: pi.currency,
      }))}
      onLogout={logout}
    >
      {children}
    </DashboardShell>
  );
}
