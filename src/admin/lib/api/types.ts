// Wire types for the staff routes in specs/api/openapi.yaml.
export type Role = "cashier" | "host" | "kitchen" | "manager";

export const ROLES: readonly Role[] = ["host", "kitchen", "cashier", "manager"];

export const ROLE_LABELS: Record<Role, string> = {
  host: "Host / server",
  kitchen: "Kitchen",
  cashier: "Cashier",
  manager: "Manager",
};

export type StaffIdentity = {
  staff_id: string;
  branch_id: string;
  email: string;
  display_name: string;
  roles: Role[];
  session_expires_at?: string;
};

export type Staff = {
  id: string;
  branch_id: string;
  email: string;
  display_name: string;
  status: "invited" | "active" | "disabled";
  roles: Role[];
  version: number;
  created_at: string;
};

export type StaffPage = { items: Staff[]; next_cursor: string | null; server_time: string };

export type Activation = { token: string; expires_at: string };

export type ApiError = {
  code: string;
  message: string;
  request_id: string;
  fields: Record<string, string>;
};
