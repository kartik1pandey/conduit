const TONES = {
  success: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  danger: "bg-rose-500/10 text-rose-600 dark:text-rose-400",
  neutral: "bg-slate-500/10 text-[var(--muted)]",
  warning: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
} as const;

export function Badge({
  children,
  tone = "neutral",
}: {
  children: React.ReactNode;
  tone?: keyof typeof TONES;
}) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${TONES[tone]}`}
    >
      {children}
    </span>
  );
}

// statusTone maps this project's own status vocabulary (payment intent
// status, webhook delivery status, risk decision) to a badge tone — kept
// as one lookup so a new status value added anywhere in this project only
// needs one line here, not a scattered set of if/else chains per page.
export function statusTone(status: string): keyof typeof TONES {
  switch (status) {
    case "succeeded":
    case "allow":
    case "delivered":
      return "success";
    case "failed":
    case "decline":
    case "dead_lettered":
      return "danger";
    case "refunded":
    case "pending":
    case "retrying":
      return "warning";
    default:
      return "neutral";
  }
}
