"use client";

import { useActionState } from "react";
import { Button } from "@/components/ui/Button";
import { useActionToast } from "@/lib/useActionToast";
import { invite } from "./actions";

export function InviteForm() {
  const [state, formAction, pending] = useActionState(invite, undefined);
  useActionToast(state, "Invited.");

  return (
    <form action={formAction} className="mt-4 flex items-end gap-3">
      <label className="flex-1">
        <span className="mb-1 block text-sm font-medium">Email</span>
        <input
          name="email"
          type="email"
          required
          className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        />
      </label>
      <label>
        <span className="mb-1 block text-sm font-medium">Role</span>
        <select
          name="role"
          className="rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        >
          <option value="developer">developer</option>
          <option value="read-only">read-only</option>
        </select>
      </label>
      <label>
        <span className="mb-1 block text-sm font-medium">
          Password (optional)
        </span>
        <input
          name="password"
          type="password"
          placeholder="Leave blank for GitHub-only"
          className="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
        />
      </label>
      <Button type="submit" disabled={pending}>
        {pending ? "Inviting…" : "Invite"}
      </Button>
      {state && "error" in state && (
        <p className="text-sm text-[var(--danger)]">{state.error}</p>
      )}
      {state && "ok" in state && (
        <p className="text-sm text-[var(--success)]">Invited.</p>
      )}
    </form>
  );
}
