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

export type Need = "accessible" | "high_chair";
export const NEEDS: readonly Need[] = ["accessible", "high_chair"];
export const NEED_LABELS: Record<Need, string> = { accessible: "Accessible", high_chair: "High chair" };

export type Group = { id?: string; label: string; min_party: number; max_party: number };

export type Table = {
  id: string;
  label: string;
  capacity: number;
  needs: Need[];
  state: "available" | "held" | "occupied" | "cleaning";
  active: boolean;
  version: number;
  claim: null | { kind: "hold" | "visit"; queue_ticket_id?: string; display_number?: number; visit_id?: string };
};

export type Ticket = {
  id: string;
  branch_id: string;
  display_number: number;
  business_date: string;
  state: "waiting" | "called" | "seated" | "cancelled" | "no_show";
  party_size: number;
  needs: Need[];
  source: "guest" | "staff";
  seating_group: Group | null;
  parties_ahead: number | null;
  called_until: string | null;
  called_table_label: string | null;
  overdue: boolean;
  version: number;
  created_at: string;
  server_time: string;
};

export type TicketPage = { items: Ticket[]; next_cursor: string | null; server_time: string };

export type Visit = {
  id: string;
  branch_id: string;
  state: "open" | "paid" | "departed" | "closed";
  party_size: number;
  needs: Need[];
  table: { id: string; label: string };
  version: number;
  opened_at: string;
};

export type SeatResult = { visit: Visit; dining: { token: string } };
export type JoinResult = { ticket: Ticket; tracking: { token: string } };

export type MenuOption = { id: string; name_th: string; name_en: string; price_delta_satang: number };
export type MenuGroup = { id: string; name_th: string; name_en: string; min_choices: number; max_choices: number; options: MenuOption[] };
export type MenuItem = { id: string; name_th: string; name_en: string; price_satang: number; sold_out: boolean; version: number; option_groups: MenuGroup[] };
export type MenuCategory = { id: string; name_th: string; name_en: string; items: MenuItem[] };
export type Menu = { branch_id: string; revision: number; currency: "THB"; categories: MenuCategory[] };

export type OptionSnapshot = { group_name_en: string; name_th: string; name_en: string; price_delta_satang: number };
export type LineState = "submitted" | "accepted" | "preparing" | "ready" | "served" | "rejected" | "cancelled";
export type OrderLine = {
  id: string; order_id: string; item_id: string; name_th: string; name_en: string; options: OptionSnapshot[];
  unit_price_satang: number; quantity: number; line_total_satang: number; note: string | null;
  state: LineState; chargeable: boolean; reason: string | null; version: number;
};
export type Order = { id: string; visit_id: string; actor: "guest" | "staff"; created_at: string; lines: OrderLine[] };
export type OrdersPage = { items: Order[]; next_cursor: string | null; chargeable_total_satang: number; chargeable_lines: number; server_time: string };
export type KitchenLine = {
  id: string; order_id: string; visit_id: string; table_label: string; name_th: string; name_en: string;
  options: OptionSnapshot[]; quantity: number; note: string | null; state: LineState; version: number; created_at: string;
};
export type KitchenPage = { items: KitchenLine[]; next_cursor: string | null; server_time: string };
export type Assistance = {
  id: string; visit_id: string; table_label: string; topic: "help" | "allergy" | "checkout"; note: string | null;
  state: "open" | "acknowledged" | "resolved"; version: number; created_at: string; acknowledged_at: string | null; resolved_at: string | null;
};
