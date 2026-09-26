"use client";

import { BellRing, Users } from "lucide-react";
import { useCallback, useState } from "react";
import { Notice } from "@/components/common/Notice";
import { Freshness } from "@/components/Freshness";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
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
    return poll.error ? <Notice notice={{ role: "alert", text: poll.error.message }} /> : <p role="status" className="py-6 text-center text-sm text-muted-foreground">Loading your ticket…</p>;
  }
  const step = t.state === "waiting" ? 0 : t.state === "called" ? 1 : t.state === "seated" ? 2 : -1;
  return (
    <section aria-labelledby="ticket-title" className="grid gap-4">
      <div className="grid justify-items-center gap-1 rounded-3xl border bg-card p-6 text-center shadow-sm">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">Your queue number</p>
        <h2 id="ticket-title" className="text-5xl font-extrabold tracking-tight tabular-nums">
          <span className="sr-only">Ticket </span>
          <span aria-hidden>#</span>
          {t.display_number}
        </h2>
        <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Users className="size-4" aria-hidden /> Party of {t.party_size}
          {t.seating_group ? ` · group ${t.seating_group.label}` : " · staff will seat you"}
        </p>
      </div>

      {step >= 0 && (
        <ol className="grid grid-cols-3 gap-2" aria-label="Progress">
          {["Waiting", "Table ready", "Seated"].map((label, i) => (
            <li key={label} aria-current={i === step ? "step" : undefined} className="grid justify-items-center gap-1.5 text-center text-xs">
              <span className={`h-1.5 w-full rounded-full ${i <= step ? "bg-primary" : "bg-muted"}`} aria-hidden />
              <span className={i === step ? "font-semibold text-foreground" : "text-muted-foreground"}>{label}</span>
            </li>
          ))}
        </ol>
      )}

      {t.state === "waiting" && (
        <p className="rounded-2xl bg-accent p-4 text-sm text-accent-foreground">
          <strong className="block text-base">
            {t.parties_ahead === 0 ? "You are next in your group." : `${t.parties_ahead} ${t.parties_ahead === 1 ? "party" : "parties"} ahead in your group.`}
          </strong>
          <small>This is your place among parties of a similar size, not an exact order or wait time.</small>
        </p>
      )}
      {t.state === "called" && (
        <p role="alert" className="flex gap-3 rounded-2xl bg-success-soft p-4 text-sm text-success">
          <BellRing className="mt-0.5 size-5 shrink-0" aria-hidden />
          <span>
            <strong className="block text-base">Your table is ready</strong>
            Please come to the host stand for table {t.called_table_label} by {time(t.called_until)}.
          </span>
        </p>
      )}
      {t.state === "seated" && <p className="rounded-2xl bg-success-soft p-4 text-sm text-success">You have been seated. Enjoy your meal!</p>}
      {t.state === "cancelled" && <p className="rounded-2xl bg-muted p-4 text-sm">This ticket was cancelled. You can join again from the entrance QR code.</p>}
      {t.state === "no_show" && <p className="rounded-2xl bg-muted p-4 text-sm">We could not find you when your table was ready. Please ask staff or join again.</p>}

      <Freshness updatedAt={poll.updatedAt} error={poll.error} intervalMs={POLL_MS} className="justify-self-center" />
      <Notice notice={message ? { role: "status", text: message } : null} />
      {(t.state === "waiting" || t.state === "called") && (
        <>
          <Button type="button" variant="ghost" size="lg" disabled={offline} onClick={() => setConfirming(true)} className="h-12 text-destructive">
            Cancel ticket
          </Button>
          <AlertDialog open={confirming} onOpenChange={setConfirming}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Leave the queue?</AlertDialogTitle>
                <AlertDialogDescription>You will lose your place. You can join again from the entrance QR code.</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Keep my place</AlertDialogCancel>
                <AlertDialogAction variant="destructive" disabled={offline} onClick={() => void cancel()}>
                  Yes, cancel my ticket
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      )}
    </section>
  );
}
