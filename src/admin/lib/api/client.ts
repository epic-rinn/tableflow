import type { ApiError } from "./types";

export type { ApiError };

export type ApiResult<T> =
  | { ok: true; status: number; data: T }
  | { ok: false; status: number; error: ApiError };

const offline: ApiError = {
  code: "NETWORK",
  message: "Cannot reach the server. Check the connection and try again.",
  request_id: "",
  fields: {},
};

// Same-origin call to the Go API through this app's /api/v1 proxy. The
// browser attaches the host-only staff cookie and an Origin header.
export type ApiInit = {
  method?: string;
  body?: unknown;
  /** Idempotency-Key for business mutations; reuse it when retrying. */
  key?: string;
  signal?: AbortSignal;
};

export async function api<T>(path: string, init: ApiInit = {}): Promise<ApiResult<T>> {
  let res: Response;
  try {
    res = await fetch(`/api/v1${path}`, {
      method: init.method ?? "GET",
      headers: {
        ...(init.body === undefined ? {} : { "Content-Type": "application/json" }),
        ...(init.key ? { "Idempotency-Key": init.key } : {}),
      },
      signal: init.signal,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
      cache: "no-store",
      credentials: "same-origin",
    });
  } catch (e) {
    if (e instanceof DOMException && e.name === "AbortError") return { ok: false, status: -1, error: { ...offline, code: "ABORTED" } };
    return { ok: false, status: 0, error: offline };
  }
  if (res.status === 204) return { ok: true, status: 204, data: undefined as T };
  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    // Non-JSON (e.g. proxy failure): fall through to a generic error.
  }
  if (res.ok) return { ok: true, status: res.status, data: payload as T };
  const error = (payload as { error?: ApiError } | null)?.error ?? {
    ...offline,
    code: "UNAVAILABLE",
    message: "The service is temporarily unavailable.",
  };
  return { ok: false, status: res.status, error };
}
