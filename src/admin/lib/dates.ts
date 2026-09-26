// Business dates are calendar dates in the branch timezone (Asia/Bangkok in
// the pilot); the API validates ranges (≤31 days).
const BRANCH_TZ = "Asia/Bangkok";

export function businessDate(offsetDays = 0): string {
  const d = new Date(Date.now() + offsetDays * 86_400_000);
  return new Intl.DateTimeFormat("en-CA", { timeZone: BRANCH_TZ, year: "numeric", month: "2-digit", day: "2-digit" }).format(d);
}

export const isDate = (s: string | undefined): s is string => !!s && /^\d{4}-\d{2}-\d{2}$/.test(s);
