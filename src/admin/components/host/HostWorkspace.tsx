"use client";

import { Accessibility, Baby, BellRing, Check, Copy, Printer, Users, X } from "lucide-react";
import Link from "next/link";
import { useCallback, useState } from "react";
import { NativeSelect } from "@/components/common/NativeSelect";
import { QrCode } from "@/components/common/QrCode";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { ReasonDialog } from "@/components/common/ReasonDialog";
import { StateBadge } from "@/components/common/StateBadge";
import { Freshness } from "@/components/Freshness";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, type ApiResult } from "@/lib/api/client";
import {
  type Assistance,
  NEED_LABELS,
  NEEDS,
  type JoinResult,
  type Need,
  type SeatResult,
  type Table,
  type Ticket,
  type TicketPage,
  type Visit,
} from "@/lib/api/types";
import { diningLink, trackingLink } from "@/lib/links";
import { usePolling } from "@/lib/usePolling";
import { useIdempotent } from "@/lib/useIdempotent";

const POLL_MS = 3000;

type Notice = { kind: "error" | "info"; text: string } | null;
type ShareLink = { label: string; url: string } | null;
type Override = { description: string; retry: (reason: string) => void } | null;

function fits(t: Table, party: number, needs: Need[]) {
  return t.active && t.capacity >= party && needs.every((n) => t.needs.includes(n));
}

function time(iso: string | null) {
  return iso ? new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : "";
}

export function HostWorkspace({ branchId, isManager }: { branchId: string; isManager: boolean }) {
  const run = useIdempotent();
  const [notice, setNotice] = useState<Notice>(null);
  const [link, setLink] = useState<ShareLink>(null);
  const [override, setOverride] = useState<Override>(null);

  const queue = usePolling(
    useCallback((signal: AbortSignal) => api<TicketPage>(`/branches/${branchId}/queue-tickets?limit=100`, { signal }), [branchId]),
    POLL_MS,
  );
  const floor = usePolling(
    useCallback((signal: AbortSignal) => api<{ items: Table[] }>(`/branches/${branchId}/tables`, { signal }), [branchId]),
    POLL_MS,
  );
  const tables = floor.data?.items ?? [];
  const tickets = queue.data?.items ?? [];
  const offline = !!queue.error && queue.error.code === "NETWORK";

  const refresh = () => {
    queue.refresh();
    floor.refresh();
  };

  // Runs a mutation; on success refreshes the boards, on a fairness bypass
  // offers managers an override with a reason.
  async function act<T>(action: string, send: (key: string) => Promise<ApiResult<T>>, onOk?: (data: T) => void,
    retryWithReason?: (reason: string) => void) {
    setNotice(null);
    setOverride(null);
    const res = await run(action, send);
    if (res.ok) {
      onOk?.(res.data);
      refresh();
      return;
    }
    if (res.error.code === "BYPASS_REQUIRES_OVERRIDE" && retryWithReason) {
      setOverride(isManager ? { description: res.error.message, retry: retryWithReason } : null);
      setNotice({ kind: "error", text: isManager ? res.error.message : `${res.error.message} Ask a manager.` });
      return;
    }
    const unknown = res.status === 0 || res.status >= 500;
    setNotice({ kind: "error", text: unknown ? `${res.error.message} The action may not have been applied — retry it to be sure.` : res.error.message });
    if (res.status === 409) refresh();
  }

  function call(t: Ticket, tableId: string, reason?: string) {
    void act(`call:${t.id}:${tableId}:${reason ?? ""}`,
      (key) => api<Ticket>(`/queue-tickets/${t.id}/call`, { method: "POST", key, body: { table_id: tableId, expected_version: t.version, ...(reason ? { override_reason: reason } : {}) } }),
      undefined, (r) => call(t, tableId, r));
  }

  function seat(tableId: string, version: number, ticket: Ticket | null, party: number, reason?: string) {
    const body = {
      branch_id: branchId,
      table_id: tableId,
      expected_table_version: version,
      ...(ticket ? { queue_ticket_id: ticket.id } : { party_size: party, needs: [] }),
      ...(reason ? { override_reason: reason } : {}),
    };
    void act(`seat:${tableId}:${ticket?.id ?? party}:${reason ?? ""}`, (key) => api<SeatResult>("/visits", { method: "POST", key, body }),
      (res) => setLink({ label: `Dining QR for table ${res.visit.table.label}`, url: diningLink(res.dining.token) }),
      (r) => seat(tableId, version, ticket, party, r));
  }

  function ticketAction(t: Ticket, path: string, extra: object = {}) {
    void act(`${path}:${t.id}:${t.version}`, (key) => api<Ticket>(`/queue-tickets/${t.id}/${path}`, { method: "POST", key, body: { expected_version: t.version, ...extra } }));
  }

  async function visitAction(visitId: string, path: string, extra: object = {}, onOk?: (d: SeatResult | Visit) => void) {
    const v = await api<Visit>(`/visits/${visitId}`);
    if (!v.ok) {
      setNotice({ kind: "error", text: v.error.message });
      return;
    }
    void act(`${path}:${visitId}:${v.data.version}`,
      (key) => api<SeatResult | Visit>(`/visits/${visitId}/${path}`, { method: "POST", key, body: { expected_version: v.data.version, ...extra } }), onOk);
  }

  function ready(t: Table) {
    void act(`ready:${t.id}:${t.version}`, (key) => api<Table>(`/tables/${t.id}/ready`, { method: "POST", key, body: { expected_version: t.version } }));
  }

  return (
    <>
      <PageHeader
        title="Queue & tables"
        description="Seat parties in join order, keep tables moving and answer requests."
        actions={<Freshness updatedAt={queue.updatedAt} error={queue.error ?? floor.error} intervalMs={POLL_MS} />}
      />
      <div className="mb-4 grid gap-2">
        {offline && <Notice notice={{ role: "alert", text: "Offline — actions are disabled until the connection returns." }} />}
        <Notice notice={notice && { role: notice.kind === "error" ? "alert" : "status", text: notice.text }} />
        {link && <SharePanel link={link} onDone={() => setLink(null)} />}
      </div>
      <OverrideDialog override={override} onCancel={() => setOverride(null)} />

      <AssistanceBoard branchId={branchId} disabled={offline} />

      <div className="grid gap-6 xl:grid-cols-[minmax(320px,380px)_1fr]">
        <div className="grid content-start gap-6">
          <AssistedJoin branchId={branchId} disabled={offline} onJoined={(r) => setLink({ label: `Ticket ${r.ticket.display_number} tracking link`, url: trackingLink(r.tracking.token) })} act={act} />

          <Card role="region" aria-labelledby="queue-title">
            <CardHeader>
              <CardTitle>
                <h2 id="queue-title">Queue ({tickets.length})</h2>
              </CardTitle>
              <CardDescription>Oldest compatible party first.</CardDescription>
            </CardHeader>
            <CardContent>
              {tickets.length === 0 && <p className="py-6 text-center text-sm text-muted-foreground">No one is waiting.</p>}
              <ol className="grid gap-3">
                {tickets.map((t) => (
                  <li key={`${t.id}:${t.version}`}>
                    <QueueRow ticket={t} tables={tables} disabled={offline}
                      onCall={(tableId) => call(t, tableId)}
                      onSeat={() => {
                        const held = tables.find((x) => x.claim?.queue_ticket_id === t.id);
                        if (held) seat(held.id, held.version, t, t.party_size);
                      }}
                      onNoShow={(reason) => ticketAction(t, "no-show", { reason })}
                      onCancel={() => ticketAction(t, "cancel")} />
                  </li>
                ))}
              </ol>
            </CardContent>
          </Card>
        </div>

        <section aria-labelledby="tables-title" className="grid content-start gap-3">
          <h2 id="tables-title" className="text-lg font-semibold">Tables</h2>
          <ul className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
            {tables.map((t) => (
              <li key={`${t.id}:${t.version}`}>
                <TableCard table={t} tables={tables} disabled={offline}
                  onSeatWalkIn={(party) => seat(t.id, t.version, null, party)}
                  onReady={() => ready(t)}
                  onMove={(dest) => {
                    const d = tables.find((x) => x.id === dest);
                    if (d && t.claim?.visit_id) void visitAction(t.claim.visit_id, "move", { table_id: dest, expected_table_version: d.version });
                  }}
                  onDepart={() => t.claim?.visit_id && void visitAction(t.claim.visit_id, "depart")}
                  onClose={(reason) => t.claim?.visit_id && void visitAction(t.claim.visit_id, "close-empty", { reason })}
                  onRotate={(reason) =>
                    t.claim?.visit_id &&
                    void visitAction(t.claim.visit_id, "rotate-access", { reason }, (d) => {
                      const r = d as SeatResult;
                      setLink({ label: `New dining QR for table ${r.visit.table.label}`, url: diningLink(r.dining.token) });
                    })
                  } />
              </li>
            ))}
          </ul>
        </section>
      </div>
    </>
  );
}

// One-time capability link (tracking or dining QR) to show, scan, print or
// read out. It is shown once; printing uses print-only styles.
function SharePanel({ link, onDone }: { link: { label: string; url: string }; onDone: () => void }) {
  const [copied, setCopied] = useState(false);
  return (
    <div role="status" className="print-area grid gap-3 rounded-xl border border-primary/30 bg-accent/60 p-4 sm:grid-cols-[auto_1fr]">
      <QrCode value={link.url} label="Scannable QR code" />
      <div className="grid content-start gap-2">
        <p className="text-sm font-medium">{link.label} — show, print or read out this link; it is not shown again:</p>
        <div className="flex flex-wrap gap-2 print:hidden">
          <Input aria-label={link.label} readOnly value={link.url} onFocus={(e) => e.currentTarget.select()} className="min-w-0 flex-1 font-mono text-xs" />
          <Button type="button" variant="outline" onClick={() => void navigator.clipboard?.writeText(link.url).then(() => setCopied(true))}>
            {copied ? <Check aria-hidden /> : <Copy aria-hidden />} {copied ? "Copied" : "Copy"}
          </Button>
          <Button type="button" variant="outline" onClick={() => window.print()}>
            <Printer aria-hidden /> Print
          </Button>
          <Button type="button" onClick={onDone}>
            Done
          </Button>
        </div>
        <p className="hidden text-lg font-semibold print:block">Scan to open your table · สแกนเพื่อเปิดโต๊ะของคุณ</p>
      </div>
    </div>
  );
}

type Act = <T>(action: string, send: (key: string) => Promise<ApiResult<T>>, onOk?: (d: T) => void) => Promise<void>;

function AssistedJoin({ branchId, disabled, onJoined, act }: { branchId: string; disabled: boolean; onJoined: (r: JoinResult) => void; act: Act }) {
  const [party, setParty] = useState(2);
  const [needs, setNeeds] = useState<Need[]>([]);
  return (
    <Card role="region" aria-labelledby="join-title">
      <CardHeader>
        <CardTitle>
          <h2 id="join-title">Add a party to the queue</h2>
        </CardTitle>
        <CardDescription>For guests without a phone, or who ask staff to join.</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void act(`join:${party}:${needs.join(",")}`,
              (key) => api<JoinResult>(`/branches/${branchId}/queue-tickets`, { method: "POST", key, body: { party_size: party, needs } }), onJoined);
          }}>
          <div className="grid gap-2">
            <Label htmlFor="join-party">Party size</Label>
            <Input id="join-party" type="number" min={1} max={50} value={party} onChange={(e) => setParty(Number(e.target.value))} className="w-28" />
          </div>
          <fieldset className="flex flex-wrap gap-4">
            <legend className="sr-only">Seating needs</legend>
            {NEEDS.map((n) => (
              <Label key={n} className="font-normal">
                <Checkbox checked={needs.includes(n)} onCheckedChange={(c) => setNeeds(c === true ? [...needs, n] : needs.filter((x) => x !== n))} />
                {NEED_LABELS[n]}
              </Label>
            ))}
          </fieldset>
          <Button type="submit" disabled={disabled} className="justify-self-start">
            <Users aria-hidden /> Add to queue
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function NeedIcons({ needs }: { needs: Need[] }) {
  return (
    <>
      {needs.includes("accessible") && <Accessibility className="size-4 text-info" aria-label="Accessible seating" />}
      {needs.includes("high_chair") && <Baby className="size-4 text-info" aria-label="High chair" />}
    </>
  );
}

function QueueRow({ ticket: t, tables, disabled, onCall, onSeat, onNoShow, onCancel }: {
  ticket: Ticket; tables: Table[]; disabled: boolean;
  onCall: (tableId: string) => void; onSeat: () => void; onNoShow: (reason: string) => void; onCancel: () => void;
}) {
  const options = tables.filter((x) => x.state === "available" && fits(x, t.party_size, t.needs));
  const [tableId, setTableId] = useState("");
  const [reason, setReason] = useState("");
  const chosen = options.some((o) => o.id === tableId) ? tableId : options[0]?.id ?? "";
  return (
    <article aria-label={`Ticket ${t.display_number}`} className="grid gap-3 rounded-lg border p-3">
      <div className="flex items-center gap-3">
        <span className="flex size-11 items-center justify-center rounded-lg bg-secondary text-lg font-bold tabular-nums">#{t.display_number}</span>
        <div className="grid flex-1 gap-0.5 text-sm">
          <span className="flex items-center gap-1.5 font-medium">
            Party of {t.party_size} <NeedIcons needs={t.needs} />
          </span>
          <span className="text-xs text-muted-foreground">
            {t.needs.length > 0 && `${t.needs.map((n) => NEED_LABELS[n]).join(", ")} · `}
            {t.seating_group?.label ?? "special (staff)"}
          </span>
        </div>
        <StateBadge state={t.overdue ? "no_show" : t.state} label={t.overdue ? "overdue" : t.state} />
      </div>
      {t.state === "waiting" && (
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex-1">
            <span className="sr-only">Table for ticket {t.display_number}</span>
            <NativeSelect value={chosen} onChange={(e) => setTableId(e.target.value)} disabled={disabled || options.length === 0} className="w-full">
              {options.length === 0 && <option value="">No fitting table free</option>}
              {options.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label} ({o.capacity})
                </option>
              ))}
            </NativeSelect>
          </label>
          <Button type="button" size="sm" disabled={disabled || !chosen} onClick={() => onCall(chosen)}>
            <BellRing aria-hidden /> Call
          </Button>
        </div>
      )}
      {t.state === "called" && (
        <div className="grid gap-2">
          <p className="text-sm">
            called to <strong>{t.called_table_label}</strong> until {time(t.called_until)}
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" size="sm" disabled={disabled} onClick={onSeat}>
              Seat
            </Button>
            <label className="min-w-0 flex-1">
              <span className="sr-only">No-show reason for ticket {t.display_number}</span>
              <Input placeholder="No-show reason" value={reason} onChange={(e) => setReason(e.target.value)} className="h-7" />
            </label>
            <Button type="button" size="sm" variant="destructive" disabled={disabled || !reason.trim()} onClick={() => onNoShow(reason)}>
              No-show
            </Button>
          </div>
        </div>
      )}
      <Button type="button" variant="ghost" size="sm" disabled={disabled} onClick={onCancel} className="justify-self-start text-muted-foreground">
        <X aria-hidden /> Cancel ticket
      </Button>
    </article>
  );
}

function TableCard({ table: t, tables, disabled, onSeatWalkIn, onReady, onMove, onDepart, onClose, onRotate }: {
  table: Table; tables: Table[]; disabled: boolean;
  onSeatWalkIn: (party: number) => void; onReady: () => void; onMove: (dest: string) => void;
  onDepart: () => void; onClose: (reason: string) => void; onRotate: (reason: string) => void;
}) {
  const [party, setParty] = useState(Math.min(2, t.capacity));
  const [dest, setDest] = useState("");
  const free = tables.filter((x) => x.id !== t.id && x.state === "available" && x.active);
  const moveTo = free.some((f) => f.id === dest) ? dest : free[0]?.id ?? "";
  return (
    <Card aria-label={`Table ${t.label}`} role="article" className={t.active ? "h-full" : "h-full opacity-60"}>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <h3 className="text-base">{t.label}</h3>
          <NeedIcons needs={t.needs} />
        </CardTitle>
        <CardDescription>
          {t.capacity} seats{t.needs.length ? ` · ${t.needs.map((n) => NEED_LABELS[n]).join(", ")}` : ""}
        </CardDescription>
        <CardAction>
          <StateBadge state={t.state} />
        </CardAction>
      </CardHeader>
      <CardContent className="grid gap-3 text-sm">
        {(!t.active || t.claim?.kind === "hold") && (
          <p className="text-muted-foreground">
            {!t.active && "inactive"}
            {t.claim?.kind === "hold" && `held for #${t.claim.display_number}`}
          </p>
        )}
        {t.state === "available" && t.active && (
          <div className="flex items-end gap-2">
            <div className="grid gap-1.5">
              <Label htmlFor={`walkin-${t.id}`}>Walk-in party</Label>
              <Input id={`walkin-${t.id}`} type="number" min={1} max={t.capacity} value={party} onChange={(e) => setParty(Number(e.target.value))} className="w-20" />
            </div>
            <Button type="button" disabled={disabled} onClick={() => onSeatWalkIn(party)}>
              Seat walk-in
            </Button>
          </div>
        )}
        {t.state === "occupied" && t.claim?.visit_id && (
          <>
            <Link href={`/visits/${t.claim.visit_id}`} className="font-medium text-primary underline-offset-4 hover:underline">
              Orders for {t.label}
            </Link>
            <div className="flex gap-2">
              <label className="min-w-0 flex-1">
                <span className="sr-only">Move table {t.label} to</span>
                <NativeSelect value={moveTo} onChange={(e) => setDest(e.target.value)} disabled={disabled || free.length === 0} className="w-full">
                  {free.length === 0 && <option value="">No free table</option>}
                  {free.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.label} ({f.capacity})
                    </option>
                  ))}
                </NativeSelect>
              </label>
              <Button type="button" variant="outline" size="sm" disabled={disabled || !moveTo} onClick={() => onMove(moveTo)}>
                Move
              </Button>
            </div>
          </>
        )}
      </CardContent>
      {t.state === "occupied" && t.claim?.visit_id && (
        <CardFooter className="flex flex-wrap gap-2">
          <Button type="button" size="sm" disabled={disabled} onClick={onDepart}>
            Depart (paid)
          </Button>
          <ReasonDialog trigger="Close empty" title={`Close table ${t.label} as empty`}
            description="Only for parties that leave without ordering anything chargeable. The reason is audited."
            reasonLabel={`Reason for table ${t.label}`} confirmLabel="Close visit" destructive disabled={disabled} onConfirm={onClose} />
          <ReasonDialog trigger="New QR" title={`New dining QR for table ${t.label}`}
            description="The old QR and every phone using it stop working. The reason is audited."
            reasonLabel={`Reason for new QR at ${t.label}`} confirmLabel="Issue new QR" disabled={disabled} onConfirm={onRotate} />
        </CardFooter>
      )}
      {t.state === "cleaning" && (
        <CardFooter>
          <Button type="button" variant="outline" disabled={disabled} onClick={onReady}>
            <Check aria-hidden /> Mark ready
          </Button>
        </CardFooter>
      )}
    </Card>
  );
}

function OverrideDialog({ override, onCancel }: { override: { description: string; retry: (reason: string) => void } | null; onCancel: () => void }) {
  const [reason, setReason] = useState("");
  return (
    <Dialog open={!!override} onOpenChange={(o) => { if (!o) { onCancel(); setReason(""); } }}>
      <DialogContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            override?.retry(reason.trim());
            setReason("");
          }}>
          <DialogHeader>
            <DialogTitle>Skip the queue order?</DialogTitle>
            <DialogDescription>{override?.description}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="override-reason">Manager override reason</Label>
            <Input id="override-reason" value={reason} onChange={(e) => setReason(e.target.value)} maxLength={500} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => { onCancel(); setReason(""); }}>
              Keep queue order
            </Button>
            <Button type="submit" disabled={!reason.trim()}>
              Override and continue
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

const TOPICS: Record<Assistance["topic"], string> = { help: "Help", allergy: "Allergy", checkout: "Bill please" };

function AssistanceBoard({ branchId, disabled }: { branchId: string; disabled: boolean }) {
  const run = useIdempotent();
  const [notice, setNotice] = useState("");
  const board = usePolling(
    useCallback((signal: AbortSignal) => api<{ items: Assistance[] }>(`/branches/${branchId}/assistance`, { signal }), [branchId]),
    POLL_MS,
  );
  async function move(a: Assistance, to: "acknowledged" | "resolved") {
    const res = await run(`assist:${a.id}:${a.version}:${to}`, (key) =>
      api(`/assistance/${a.id}/transition`, { method: "POST", key, body: { expected_version: a.version, to_state: to } }));
    setNotice(res.ok ? "" : res.error.message);
    board.refresh();
  }
  const items = board.data?.items ?? [];
  return (
    <section aria-labelledby="assist-board-title" className="mb-6 grid gap-3">
      <h2 id="assist-board-title" className="flex items-center gap-2 text-lg font-semibold">
        Requests ({items.length})
      </h2>
      <Notice notice={notice ? { role: "alert", text: notice } : null} />
      {items.length === 0 ? (
        <p className="text-sm text-muted-foreground">No open requests.</p>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((a) => {
            const urgent = a.topic === "allergy" && a.state === "open";
            return (
              <li key={`${a.id}:${a.version}`}
                className={`flex flex-wrap items-center gap-2 rounded-lg border p-3 text-sm ${urgent ? "border-destructive/40 bg-destructive-soft" : "bg-card"}`}>
                <strong>{TOPICS[a.topic]}</strong> <span>· table {a.table_label}</span> <StateBadge state={a.state} />
                {a.note && <span className="w-full text-muted-foreground">“{a.note}”</span>}
                <span className="ml-auto flex gap-2">
                  {a.state === "open" && (
                    <Button type="button" size="sm" variant="outline" disabled={disabled} onClick={() => void move(a, "acknowledged")}>Acknowledge</Button>
                  )}
                  <Button type="button" size="sm" disabled={disabled} onClick={() => void move(a, "resolved")}>Resolve</Button>
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
