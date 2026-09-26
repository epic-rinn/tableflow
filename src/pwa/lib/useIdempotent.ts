"use client";

import { useRef } from "react";
import type { ApiResult } from "@/lib/api/client";

// Runs a business mutation with an Idempotency-Key. If the outcome is
// unknown (network failure or 5xx), the same key is kept so an explicit
// retry of the same action can never apply it twice; a definitive answer
// (success or 4xx) clears it. The key is scoped to one action identity.
export function useIdempotent() {
  const pending = useRef(new Map<string, string>());
  return async function run<T>(action: string, send: (key: string) => Promise<ApiResult<T>>): Promise<ApiResult<T>> {
    const key = pending.current.get(action) ?? crypto.randomUUID();
    pending.current.set(action, key);
    const res = await send(key);
    if (res.ok || (res.status >= 400 && res.status < 500)) pending.current.delete(action);
    return res;
  };
}
