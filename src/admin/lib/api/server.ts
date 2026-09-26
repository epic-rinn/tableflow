import "server-only";
import { cookies } from "next/headers";
import { cache } from "react";
import type { StaffIdentity } from "./types";

export const STAFF_COOKIE = "__Host-tf_staff";

export type SessionResult =
  | { state: "authenticated"; identity: StaffIdentity }
  | { state: "anonymous" }
  | { state: "unavailable" };

// Reads the current staff session from Go on the server, forwarding only the
// staff cookie. Go remains the authority; this only decides what to render.
export const getSession = cache(async function getSession(): Promise<SessionResult> {
  const token = (await cookies()).get(STAFF_COOKIE)?.value;
  if (!token) return { state: "anonymous" };
  const base = process.env.API_INTERNAL_URL;
  if (!base) return { state: "unavailable" };
  try {
    const res = await fetch(new URL("/api/v1/sessions/current", base), {
      headers: { Cookie: `${STAFF_COOKIE}=${token}` },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (res.status === 401) return { state: "anonymous" };
    if (!res.ok) return { state: "unavailable" };
    return { state: "authenticated", identity: (await res.json()) as StaffIdentity };
  } catch {
    return { state: "unavailable" };
  }
});
