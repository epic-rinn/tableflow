// Wire types for guest routes in specs/api/openapi.yaml.
export type CapabilityKind = "queue" | "visit";

export type GuestSession = {
  branch_id: string;
  kind: CapabilityKind;
  resource_id: string;
  expires_at: string;
};

export type Member = {
  member_id: string;
  email: string;
  locale: "th" | "en";
  email_verified: boolean;
  session_expires_at?: string;
};

export type Need = "accessible" | "high_chair";

export type Ticket = {
  id: string;
  display_number: number;
  state: "waiting" | "called" | "seated" | "cancelled" | "no_show";
  party_size: number;
  needs: Need[];
  seating_group: { label: string } | null;
  parties_ahead: number | null;
  called_until: string | null;
  called_table_label: string | null;
  version: number;
  server_time: string;
};

export type Visit = {
  id: string;
  branch_id: string;
  state: "open" | "paid" | "departed" | "closed";
  party_size: number;
  table: { id: string; label: string };
  version: number;
};

export type MenuOption = { id: string; name_th: string; name_en: string; price_delta_satang: number };
export type MenuGroup = { id: string; name_th: string; name_en: string; min_choices: number; max_choices: number; options: MenuOption[] };
export type MenuItem = { id: string; name_th: string; name_en: string; price_satang: number; sold_out: boolean; option_groups: MenuGroup[] };
export type Menu = { branch_id: string; revision: number; categories: { id: string; name_th: string; name_en: string; items: MenuItem[] }[] };
export type OrderLine = {
  id: string; name_th: string; name_en: string; options: { name_th: string; name_en: string }[]; quantity: number;
  line_total_satang: number; state: string; chargeable: boolean; reason: string | null;
};
export type OrdersPage = { items: { id: string; created_at: string; lines: OrderLine[] }[]; chargeable_total_satang: number };
export type Assistance = { id: string; topic: "help" | "allergy" | "checkout"; state: "open" | "acknowledged" | "resolved"; note: string | null };
