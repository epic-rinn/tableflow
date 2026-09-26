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
import { useI18n } from "@/lib/i18n";

const POLL_MS = 10_000;

function time(iso: string | null) {
  return iso ? new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
}

// Live tracking for the guest's own ticket (QUE-002). Position is within the
// seating group and is not a promise of global order or wait time.
export function QueueTracking({ ticketId }: { ticketId: string }) {
  const poll = usePolling(useCallback((signal: AbortSignal) => api<Ticket>(`/queue-tickets/${ticketId}`, { signal }), [ticketId]), POLL_MS);
  const run = useIdempotent();
  const { t, errorText } = useI18n();
  const [confirming, setConfirming] = useState(false);
  const [message, setMessage] = useState("");
  const tk = poll.data;
  const offline = poll.error?.code === "NETWORK";

  async function cancel() {
    if (!tk) return;
    const res = await run(`cancel:${tk.id}:${tk.version}`, (key) =>
      api<Ticket>(`/queue-tickets/${tk.id}/cancel`, { method: "POST", key, body: { expected_version: tk.version } }),
    );
    setConfirming(false);
    setMessage(res.ok ? t("ticket.cancelledNow") : errorText(res.error));
    poll.refresh();
  }

  if (!tk) {
    return poll.error ? <Notice notice={{ role: "alert", text: errorText(poll.error) }} /> : <p role="status" className="py-6 text-center text-sm text-muted-foreground">{t("ticket.loading")}</p>;
  }
  const step = tk.state === "waiting" ? 0 : tk.state === "called" ? 1 : tk.state === "seated" ? 2 : -1;
  return (
    <section aria-labelledby="ticket-title" className="grid gap-4">
      <div className="grid justify-items-center gap-1 rounded-3xl border bg-card p-6 text-center shadow-sm">
        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{t("ticket.number")}</p>
        <h2 id="ticket-title" className="text-5xl font-extrabold tracking-tight tabular-nums">
          <span className="sr-only">{t("ticket.heading", { n: tk.display_number })}</span>
          <span aria-hidden>#{tk.display_number}</span>
        </h2>
        <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
          <Users className="size-4" aria-hidden /> {t("ticket.party", { n: tk.party_size })}
          {tk.seating_group ? t("ticket.group", { label: tk.seating_group.label }) : t("ticket.staffSeat")}
        </p>
      </div>

      {step >= 0 && (
        <ol className="grid grid-cols-3 gap-2" aria-label={t("ticket.progress")}>
          {[t("ticket.stepWaiting"), t("ticket.stepReady"), t("ticket.stepSeated")].map((label, i) => (
            <li key={label} aria-current={i === step ? "step" : undefined} className="grid justify-items-center gap-1.5 text-center text-xs">
              <span className={`h-1.5 w-full rounded-full ${i <= step ? "bg-primary" : "bg-muted"}`} aria-hidden />
              <span className={i === step ? "font-semibold text-foreground" : "text-muted-foreground"}>{label}</span>
            </li>
          ))}
        </ol>
      )}

      {tk.state === "waiting" && (
        <p className="rounded-2xl bg-accent p-4 text-sm text-accent-foreground">
          <strong className="block text-base">
            {tk.parties_ahead === 0 ? t("ticket.next") : tk.parties_ahead === 1 ? t("ticket.aheadOne") : t("ticket.aheadMany", { n: tk.parties_ahead ?? 0 })}
          </strong>
          <small>{t("ticket.positionNote")}</small>
        </p>
      )}
      {tk.state === "called" && (
        <p role="alert" className="flex gap-3 rounded-2xl bg-success-soft p-4 text-sm text-success">
          <BellRing className="mt-0.5 size-5 shrink-0" aria-hidden />
          <span>
            <strong className="block text-base">{t("ticket.readyTitle")}</strong>
            {t("ticket.readyBody", { table: tk.called_table_label ?? "", time: time(tk.called_until) })}
          </span>
        </p>
      )}
      {tk.state === "seated" && <p className="rounded-2xl bg-success-soft p-4 text-sm text-success">{t("ticket.seated")}</p>}
      {tk.state === "cancelled" && <p className="rounded-2xl bg-muted p-4 text-sm">{t("ticket.cancelled")}</p>}
      {tk.state === "no_show" && <p className="rounded-2xl bg-muted p-4 text-sm">{t("ticket.noShow")}</p>}

      <Freshness updatedAt={poll.updatedAt} error={poll.error} intervalMs={POLL_MS} className="justify-self-center" />
      <Notice notice={message ? { role: "status", text: message } : null} />
      {(tk.state === "waiting" || tk.state === "called") && (
        <>
          <Button type="button" variant="ghost" size="lg" disabled={offline} onClick={() => setConfirming(true)} className="h-12 text-destructive">
            {t("ticket.cancel")}
          </Button>
          <AlertDialog open={confirming} onOpenChange={setConfirming}>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{t("ticket.leaveTitle")}</AlertDialogTitle>
                <AlertDialogDescription>{t("ticket.leaveBody")}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{t("ticket.keep")}</AlertDialogCancel>
                <AlertDialogAction variant="destructive" disabled={offline} onClick={() => void cancel()}>
                  {t("ticket.confirmCancel")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      )}
    </section>
  );
}
