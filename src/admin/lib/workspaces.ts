import type { Role, StaffIdentity } from "./api/types";

export type Workspace = { href: string; label: string; role: Role };

// Navigation only; Go authorizes every API call regardless of what is shown.
export const WORKSPACES: readonly Workspace[] = [
  { href: "/host", label: "Queue & tables", role: "host" },
  { href: "/kitchen", label: "Kitchen", role: "kitchen" },
  { href: "/cashier", label: "Cashier", role: "cashier" },
  { href: "/staff", label: "Staff", role: "manager" },
];

export function workspacesFor(identity: StaffIdentity): Workspace[] {
  return WORKSPACES.filter((w) => identity.roles.includes(w.role));
}
