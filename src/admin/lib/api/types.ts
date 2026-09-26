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
  state: "open" | "settling" | "paid" | "departed" | "closed";
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

export type Policy = { version: number; tax_mode: "exclusive" | "inclusive"; tax_bp: number; service_bp: number; configured: boolean };
export type ChargePolicy = Policy & { updated_at: string | null };
export type BillLine = {
  id: string; name_th: string; name_en: string; options: OptionSnapshot[];
  unit_price_satang: number; quantity: number; line_total_satang: number; state: LineState;
};
export type Totals = {
  gross_satang: number; discount_bp: number; discount_satang: number; net_satang: number;
  service_satang: number; tax_satang: number; total_satang: number;
};
export type SettlementRef = { id: string; receipt_reference: string; paid_at: string };
export type Bill = Totals & {
  visit_id: string; branch_id: string; table_label: string; visit_state: Visit["state"]; bill_version: number;
  frozen: boolean; policy: Policy; lines: BillLine[]; unresolved_lines: number; settlement: SettlementRef | null; server_time: string;
  member_claim: MemberClaim | null;
};
export type Tier = "base" | "silver" | "gold";
export const TIER_LABELS: Record<Tier, string> = { base: "Base", silver: "Silver", gold: "Gold" };
export type MemberClaim = { claimed: boolean; masked_email?: string; tier?: Tier };
export type LoyaltyPolicy = {
  version: number; satang_per_point: number; silver_threshold_satang: number; silver_discount_bp: number;
  gold_threshold_satang: number; gold_discount_bp: number; configured: boolean;
};
export type PaymentMethod = "cash" | "bank_transfer" | "card" | "other";
export type Settlement = { id: string; visit_id: string; receipt_reference: string; amount_satang: number; method: PaymentMethod; paid_at: string; points_earned: number | null; bill: Bill };
export type Refund = { id: string; amount_satang: number; reason: string; external_reference: string; recorded_by: string; created_at: string };
export type Receipt = Totals & {
  id: string; visit_id: string; receipt_reference: string; table_label: string; visit_state: "paid" | "departed";
  amount_satang: number; method: PaymentMethod; verification_note: string; external_reference: string | null;
  confirmed_by: string; paid_at: string; bill_version: number; policy: Policy; lines: BillLine[]; refund: Refund | null;
  member: { tier: Tier; points_earned: number; eligible_satang: number } | null;
};
export type ReceiptSummary = { id: string; receipt_reference: string; amount_satang: number; method: PaymentMethod; paid_at: string; table_label: string; refunded: boolean };
export type ReceiptPage = { items: ReceiptSummary[]; next_cursor: string | null };

export type Amount = { count: number; satang: number };
export type ReportDay = {
  date?: string; tickets_joined: number; tickets_seated: number; no_shows: number; tickets_cancelled: number; visits_opened: number;
  sales: Amount; sales_by_method: Record<string, Amount>; refunds: Amount; refunds_by_method: Record<string, Amount>;
  net_satang: number; member_settlements: number; points_earned: number; points_reversed: number;
};
export type DailyReport = { branch_id: string; timezone: string; from: string; to: string; days: ReportDay[]; totals: ReportDay };
export type AuditEvent = {
  id: string; occurred_at: string; actor: string; action: string; resource_type: string; resource_id: string | null;
  reason: string | null; request_id: string; details: Record<string, unknown>;
};
export type AuditPage = { items: AuditEvent[]; next_cursor: string | null };
