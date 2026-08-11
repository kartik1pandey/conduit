"use client";

import { useEffect, useRef } from "react";
import { toast } from "sonner";

type ToastableState = { error: string } | { ok: true } | undefined;

// Every mutating action in this app already surfaces success/error via
// inline text (kept for accessibility, and so Playwright assertions
// don't depend on a toast's timing) — this adds a toast as a louder,
// secondary signal for the same state transition, never a replacement.
export function useActionToast(state: ToastableState, successMessage?: string) {
  const handled = useRef<ToastableState>(undefined);

  useEffect(() => {
    if (state === handled.current) return;
    handled.current = state;
    if (!state) return;
    if ("error" in state) {
      toast.error(state.error);
    } else if ("ok" in state && successMessage) {
      toast.success(successMessage);
    }
  }, [state, successMessage]);
}
