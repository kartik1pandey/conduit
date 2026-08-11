import { auth } from "@/auth";
import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { can } from "@/lib/policy";
import { listUsersForMerchant } from "@/lib/users";
import { InviteForm } from "./InviteForm";

export default async function TeamPage() {
  const session = await auth();
  const users = await listUsersForMerchant(session!.user.merchantId);
  const canInvite = await can(session!.user.role, "invite_user");

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Team</h1>

      <Card>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--border)] text-left text-[var(--muted)]">
              <th className="px-4 py-3 font-medium">Email</th>
              <th className="px-4 py-3 font-medium">Role</th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr
                key={u.id}
                className="border-b border-[var(--border)] last:border-0"
              >
                <td className="px-4 py-3">{u.email}</td>
                <td className="px-4 py-3">
                  <Badge>{u.role}</Badge>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {canInvite && (
        <Card className="p-6">
          <h2 className="text-sm font-medium">Invite a teammate</h2>
          <p className="mt-1 text-xs text-[var(--muted)]">
            Set a password to share with them directly, or leave it blank and
            they can log in with GitHub using this email instead.
          </p>
          <InviteForm />
        </Card>
      )}
    </div>
  );
}
