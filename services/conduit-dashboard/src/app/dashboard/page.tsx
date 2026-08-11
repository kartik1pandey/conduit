import { auth } from "@/auth";
import { Card } from "@/components/ui/Card";

export default async function DashboardOverviewPage() {
  const session = await auth();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Overview</h1>
      <Card className="p-6">
        <p className="text-sm text-[var(--muted)]">Signed in as</p>
        <p className="mt-1 font-medium">{session?.user.email}</p>
        <p className="mt-4 text-sm text-[var(--muted)]">Merchant</p>
        <p className="mt-1 font-mono text-sm">{session?.user.merchantId}</p>
        <p className="mt-4 text-sm text-[var(--muted)]">Role</p>
        <p className="mt-1 font-medium capitalize">{session?.user.role}</p>
      </Card>
    </div>
  );
}
