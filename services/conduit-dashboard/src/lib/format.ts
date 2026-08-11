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
