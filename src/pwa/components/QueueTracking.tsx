"use client";

import { useCallback, useState } from "react";
import { Freshness } from "@/components/Freshness";
import { api } from "@/lib/api/client";
import type { Ticket } from "@/lib/api/types";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 10_000;

function time(iso: string | null) {
  return iso ? new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
}

// Live tracking for the guest's own ticket (QUE-002). Position is within the
// seating group and is not a promise of global order or wait time.
export function QueueTracking({ ticketId }: { ticketId: string }) {
  const poll = usePolling(useCallback((signal: AbortSignal) => api<Ticket>(`/queue-tickets/${ticketId}`, { signal }), [ticketId]), POLL_MS);
  const run = useIdempotent();
  const [confirming, setConfirming] = useState(false);
  const [message, setMessage] = useState("");
  const t = poll.data;
  const offline = poll.error?.code === "NETWORK";

  async function cancel() {
    if (!t) return;
    const res = await run(`cancel:${t.id}:${t.version}`, (key) =>
      api<Ticket>(`/queue-tickets/${t.id}/cancel`, { method: "POST", key, body: { expected_version: t.version } }),
    );
    setConfirming(false);
    setMessage(res.ok ? "Your ticket is cancelled." : res.error.message);
    poll.refresh();
  }

  if (!t) {
    return poll.error ? <p role="alert">{poll.error.message}</p> : <p role="status">Loading your ticket…</p>;
  }
  return (
    <section aria-labelledby="ticket-title">
      <h2 id="ticket-title">Ticket {t.display_number}</h2>
      <p>
        Party of {t.party_size}
        {t.seating_group ? ` · group ${t.seating_group.label}` : " · staff will seat you"}
      </p>
      {t.state === "waiting" && (
        <p className="position">
          {t.parties_ahead === 0 ? "You are next in your group." : `${t.parties_ahead} ${t.parties_ahead === 1 ? "party" : "parties"} ahead in your group.`}{" "}
          <small>This is your place among parties of a similar size, not an exact order or wait time.</small>
        </p>
      )}
      {t.state === "called" && (
        <p className="called" role="alert">
          Your table is ready: please come to the host stand for table {t.called_table_label} by {time(t.called_until)}.
        </p>
      )}
      {t.state === "seated" && <p>You have been seated. Enjoy your meal!</p>}
      {t.state === "cancelled" && <p>This ticket was cancelled. You can join again from the entrance QR code.</p>}
      {t.state === "no_show" && <p>We could not find you when your table was ready. Please ask staff or join again.</p>}
      <Freshness updatedAt={poll.updatedAt} error={poll.error} intervalMs={POLL_MS} />
      {message && <p role="status">{message}</p>}
      {(t.state === "waiting" || t.state === "called") &&
        (confirming ? (
          <p>
            Leave the queue?{" "}
            <button type="button" disabled={offline} onClick={cancel}>
              Yes, cancel my ticket
            </button>{" "}
            <button type="button" onClick={() => setConfirming(false)}>
              Keep my place
            </button>
          </p>
        ) : (
          <button type="button" disabled={offline} onClick={() => setConfirming(true)}>
            Cancel ticket
          </button>
        ))}
    </section>
  );
}
