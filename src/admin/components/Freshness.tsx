"use client";

import { useEffect, useState } from "react";
import type { ApiError } from "@/lib/api/client";

// Shows when data was last refreshed and warns when it may be stale.
export function Freshness({ updatedAt, error, intervalMs }: { updatedAt: Date | null; error: ApiError | null; intervalMs: number }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  const stale = !!error || (updatedAt !== null && now - updatedAt.getTime() > intervalMs * 3);
  return (
    <p className="freshness" role="status" aria-live="polite">
      {updatedAt ? `Updated ${updatedAt.toLocaleTimeString()}` : "Loading…"}
      {stale && <strong> — data may be out of date{error ? ` (${error.message})` : ""}</strong>}
    </p>
  );
}
