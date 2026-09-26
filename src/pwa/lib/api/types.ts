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
  state: "open" | "paid" | "departed" | "closed";
  party_size: number;
  table: { id: string; label: string };
  version: number;
};
