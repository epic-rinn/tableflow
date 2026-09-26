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
