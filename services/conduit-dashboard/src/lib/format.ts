// amount is always a decimal string end-to-end (never a float) — this
// formats it for display without ever parsing it into a JS number, the
// same "money never touches float arithmetic" rule every service in this
// project follows.
export function formatMoney(amount: string, currency: string): string {
  return `${amount} ${currency.toUpperCase()}`;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString();
}

// sumMoney adds decimal-string amounts (always exactly 2 places, per
// StringFixed(2) on the Go side) via integer-cent arithmetic — the same
// "never let money touch float arithmetic" rule applied to a client-side
// aggregate, not just server-side storage. parseFloat().toFixed(2) would
// technically work for a display-only sum too, but this project's own
// rule is "NUMERIC/decimal types only," not "except for aggregates
// nobody's going to notice" — the cost of doing it right here is one
// small function, not a real tradeoff.
export function sumMoney(amounts: string[]): string {
  const totalCents = amounts.reduce((sum, amount) => {
    const [whole, fraction = "0"] = amount.split(".");
    const cents =
      Number(whole) * 100 + Number(fraction.padEnd(2, "0").slice(0, 2));
    return sum + cents;
  }, 0);
  const sign = totalCents < 0 ? "-" : "";
  const abs = Math.abs(totalCents);
  return `${sign}${Math.floor(abs / 100)}.${String(abs % 100).padStart(
    2,
    "0",
  )}`;
}
