"use client";

import { useCallback } from "react";
import { Freshness } from "@/components/Freshness";
import { api } from "@/lib/api/client";
import type { Visit } from "@/lib/api/types";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 10_000;

export function VisitSummary({ visitId }: { visitId: string }) {
  const poll = usePolling(useCallback((signal: AbortSignal) => api<Visit>(`/visits/${visitId}`, { signal }), [visitId]), POLL_MS);
  const v = poll.data;
  if (!v) return poll.error ? <p role="alert">{poll.error.message}</p> : null;
  return (
    <section aria-labelledby="visit-title">
      <h2 id="visit-title">Table {v.table.label}</h2>
      <p>Party of {v.party_size}. Ordering opens here soon.</p>
      <Freshness updatedAt={poll.updatedAt} error={poll.error} intervalMs={POLL_MS} />
    </section>
  );
}
