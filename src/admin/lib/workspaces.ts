import type { Role, StaffIdentity } from "./api/types";

export type Workspace = { href: string; label: string; role: Role; group: "service" | "manage" };

// Navigation only; Go authorizes every API call regardless of what is shown.
export const WORKSPACES: readonly Workspace[] = [
  { href: "/host", label: "Queue & tables", role: "host", group: "service" },
  { href: "/host", label: "Queue & tables", role: "manager", group: "service" },
  { href: "/kitchen", label: "Kitchen", role: "kitchen", group: "service" },
  { href: "/kitchen", label: "Kitchen", role: "manager", group: "service" },
  { href: "/cashier", label: "Cashier", role: "cashier", group: "service" },
  { href: "/cashier", label: "Cashier", role: "manager", group: "service" },
  { href: "/receipts", label: "Receipts", role: "cashier", group: "service" },
  { href: "/receipts", label: "Receipts", role: "manager", group: "service" },
  { href: "/reports", label: "Reports", role: "manager", group: "manage" },
  { href: "/audit", label: "Audit log", role: "manager", group: "manage" },
  { href: "/configuration", label: "Tables & groups", role: "manager", group: "manage" },
  { href: "/menu", label: "Menu", role: "manager", group: "manage" },
  { href: "/charges", label: "Charges & tax", role: "manager", group: "manage" },
  { href: "/loyalty", label: "Loyalty", role: "manager", group: "manage" },
  { href: "/staff", label: "Staff", role: "manager", group: "manage" },
];

// A workspace may be listed for several roles; show each link once.
export function workspacesFor(identity: StaffIdentity): Workspace[] {
  const out: Workspace[] = [];
  for (const w of WORKSPACES) {
    if (identity.roles.includes(w.role) && !out.some((o) => o.href === w.href)) out.push(w);
  }
  return out;
}
