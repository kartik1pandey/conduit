const TONES = {
  success:
    "bg-[color-mix(in_srgb,var(--success)_12%,transparent)] text-[var(--success)]",
  danger:
    "bg-[color-mix(in_srgb,var(--danger)_12%,transparent)] text-[var(--danger)]",
  neutral: "bg-[var(--surface-elevated)] text-[var(--muted)]",
  warning:
    "bg-[color-mix(in_srgb,var(--warning)_12%,transparent)] text-[var(--warning)]",
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
      className={`inline-flex items-center rounded-[var(--radius-xs)] px-2.5 py-0.5 text-xs font-medium ${TONES[tone]}`}
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
