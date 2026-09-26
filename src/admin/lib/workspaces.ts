import type { Role, StaffIdentity } from "./api/types";

export type Workspace = { href: string; label: string; role: Role };

// Navigation only; Go authorizes every API call regardless of what is shown.
export const WORKSPACES: readonly Workspace[] = [
  { href: "/host", label: "Queue & tables", role: "host" },
  { href: "/host", label: "Queue & tables", role: "manager" },
  { href: "/kitchen", label: "Kitchen", role: "kitchen" },
  { href: "/kitchen", label: "Kitchen", role: "manager" },
  { href: "/cashier", label: "Cashier", role: "cashier" },
  { href: "/cashier", label: "Cashier", role: "manager" },
  { href: "/receipts", label: "Receipts", role: "cashier" },
  { href: "/receipts", label: "Receipts", role: "manager" },
  { href: "/configuration", label: "Tables & groups", role: "manager" },
  { href: "/menu", label: "Menu", role: "manager" },
  { href: "/charges", label: "Charges & tax", role: "manager" },
  { href: "/loyalty", label: "Loyalty", role: "manager" },
  { href: "/staff", label: "Staff", role: "manager" },
];

// A workspace may be listed for several roles; show each link once.
export function workspacesFor(identity: StaffIdentity): Workspace[] {
  const out: Workspace[] = [];
  for (const w of WORKSPACES) {
    if (identity.roles.includes(w.role) && !out.some((o) => o.href === w.href)) out.push(w);
  }
  return out;
}
