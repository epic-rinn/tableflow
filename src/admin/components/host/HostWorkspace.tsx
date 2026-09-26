"use client";

import { useCallback, useState } from "react";
import { Freshness } from "@/components/Freshness";
import { api, type ApiResult } from "@/lib/api/client";
import {
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
type Link = { label: string; url: string } | null;
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
  const [link, setLink] = useState<Link>(null);
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
      <h1>Queue &amp; tables</h1>
      <Freshness updatedAt={queue.updatedAt} error={queue.error ?? floor.error} intervalMs={POLL_MS} />
      {offline && <p role="alert">Offline — actions are disabled until the connection returns.</p>}
      {notice && <p role={notice.kind === "error" ? "alert" : "status"}>{notice.text}</p>}
      {override && <OverrideForm description={override.description} onSubmit={(r) => override.retry(r)} onCancel={() => setOverride(null)} />}
      {link && (
        <div role="status" className="link-panel">
          <p>{link.label} — show, print or read out this link; it is not shown again:</p>
          <input aria-label={link.label} readOnly value={link.url} onFocus={(e) => e.currentTarget.select()} />
          <button type="button" onClick={() => setLink(null)}>
            Done
          </button>
        </div>
      )}

      <AssistedJoin branchId={branchId} disabled={offline} onJoined={(r) => setLink({ label: `Ticket ${r.ticket.display_number} tracking link`, url: trackingLink(r.tracking.token) })} act={act} />

      <section aria-labelledby="queue-title">
        <h2 id="queue-title">Queue ({tickets.length})</h2>
        {tickets.length === 0 && <p>No one is waiting.</p>}
        <ol className="queue">
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
      </section>

      <section aria-labelledby="tables-title">
        <h2 id="tables-title">Tables</h2>
        <ul className="tables">
          {tables.map((t) => (
            <li key={`${t.id}:${t.version}`} className={`table ${t.state}`}>
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
    </>
  );
}

type Act = <T>(action: string, send: (key: string) => Promise<ApiResult<T>>, onOk?: (d: T) => void) => Promise<void>;

function AssistedJoin({ branchId, disabled, onJoined, act }: { branchId: string; disabled: boolean; onJoined: (r: JoinResult) => void; act: Act }) {
  const [party, setParty] = useState(2);
  const [needs, setNeeds] = useState<Need[]>([]);
  return (
    <section aria-labelledby="join-title">
      <h2 id="join-title">Add a party to the queue</h2>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void act(`join:${party}:${needs.join(",")}`,
            (key) => api<JoinResult>(`/branches/${branchId}/queue-tickets`, { method: "POST", key, body: { party_size: party, needs } }), onJoined);
        }}>
        <label>
          Party size <input type="number" min={1} max={50} value={party} onChange={(e) => setParty(Number(e.target.value))} />
        </label>
        {NEEDS.map((n) => (
          <label key={n}>
            <input type="checkbox" checked={needs.includes(n)} onChange={(e) => setNeeds(e.target.checked ? [...needs, n] : needs.filter((x) => x !== n))} />{" "}
            {NEED_LABELS[n]}
          </label>
        ))}
        <button type="submit" disabled={disabled}>
          Add to queue
        </button>
      </form>
    </section>
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
    <article aria-label={`Ticket ${t.display_number}`}>
      <strong>#{t.display_number}</strong> · party of {t.party_size}
      {t.needs.length > 0 && ` · ${t.needs.map((n) => NEED_LABELS[n]).join(", ")}`} · {t.seating_group?.label ?? "special (staff)"}
      {t.state === "waiting" && (
        <span>
          {" "}
          <label>
            <span className="sr-only">Table for ticket {t.display_number}</span>
            <select value={chosen} onChange={(e) => setTableId(e.target.value)} disabled={disabled || options.length === 0}>
              {options.length === 0 && <option value="">No fitting table free</option>}
              {options.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label} ({o.capacity})
                </option>
              ))}
            </select>
          </label>{" "}
          <button type="button" disabled={disabled || !chosen} onClick={() => onCall(chosen)}>
            Call
          </button>
        </span>
      )}
      {t.state === "called" && (
        <span>
          {" "}
          · called to {t.called_table_label} until {time(t.called_until)} {t.overdue && <strong className="overdue">overdue</strong>}{" "}
          <button type="button" disabled={disabled} onClick={onSeat}>
            Seat
          </button>{" "}
          <label>
            <span className="sr-only">No-show reason for ticket {t.display_number}</span>
            <input placeholder="No-show reason" value={reason} onChange={(e) => setReason(e.target.value)} />
          </label>
          <button type="button" disabled={disabled || !reason.trim()} onClick={() => onNoShow(reason)}>
            No-show
          </button>
        </span>
      )}{" "}
      <button type="button" disabled={disabled} onClick={onCancel}>
        Cancel ticket
      </button>
    </article>
  );
}

function TableCard({ table: t, tables, disabled, onSeatWalkIn, onReady, onMove, onDepart, onClose, onRotate }: {
  table: Table; tables: Table[]; disabled: boolean;
  onSeatWalkIn: (party: number) => void; onReady: () => void; onMove: (dest: string) => void;
  onDepart: () => void; onClose: (reason: string) => void; onRotate: (reason: string) => void;
}) {
  const [party, setParty] = useState(Math.min(2, t.capacity));
  const [reason, setReason] = useState("");
  const [dest, setDest] = useState("");
  const free = tables.filter((x) => x.id !== t.id && x.state === "available" && x.active);
  const moveTo = free.some((f) => f.id === dest) ? dest : free[0]?.id ?? "";
  return (
    <article aria-label={`Table ${t.label}`}>
      <h3>
        {t.label} <small>({t.capacity}{t.needs.length ? `, ${t.needs.map((n) => NEED_LABELS[n]).join(", ")}` : ""})</small>
      </h3>
      <p>
        {t.state}
        {!t.active && " · inactive"}
        {t.claim?.kind === "hold" && ` · held for #${t.claim.display_number}`}
      </p>
      {t.state === "available" && t.active && (
        <p>
          <label>
            Walk-in party <input type="number" min={1} max={t.capacity} value={party} onChange={(e) => setParty(Number(e.target.value))} />
          </label>{" "}
          <button type="button" disabled={disabled} onClick={() => onSeatWalkIn(party)}>
            Seat walk-in
          </button>
        </p>
      )}
      {t.state === "occupied" && t.claim?.visit_id && (
        <div>
          <label>
            <span className="sr-only">Move table {t.label} to</span>
            <select value={moveTo} onChange={(e) => setDest(e.target.value)} disabled={disabled || free.length === 0}>
              {free.length === 0 && <option value="">No free table</option>}
              {free.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.label} ({f.capacity})
                </option>
              ))}
            </select>
          </label>{" "}
          <button type="button" disabled={disabled || !moveTo} onClick={() => onMove(moveTo)}>
            Move
          </button>{" "}
          <button type="button" disabled={disabled} onClick={onDepart}>
            Depart (paid)
          </button>
          <p>
            <label>
              <span className="sr-only">Reason for table {t.label}</span>
              <input placeholder="Reason" value={reason} onChange={(e) => setReason(e.target.value)} />
            </label>{" "}
            <button type="button" disabled={disabled || !reason.trim()} onClick={() => onClose(reason)}>
              Close empty
            </button>{" "}
            <button type="button" disabled={disabled || !reason.trim()} onClick={() => onRotate(reason)}>
              New QR
            </button>
          </p>
        </div>
      )}
      {t.state === "cleaning" && (
        <button type="button" disabled={disabled} onClick={onReady}>
          Mark ready
        </button>
      )}
    </article>
  );
}

function OverrideForm({ description, onSubmit, onCancel }: { description: string; onSubmit: (reason: string) => void; onCancel: () => void }) {
  const [reason, setReason] = useState("");
  return (
    <form
      className="override"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(reason.trim());
      }}>
      <p>{description}</p>
      <label>
        Manager override reason <input value={reason} onChange={(e) => setReason(e.target.value)} maxLength={500} />
      </label>{" "}
      <button type="submit" disabled={!reason.trim()}>
        Override and continue
      </button>{" "}
      <button type="button" onClick={onCancel}>
        Keep queue order
      </button>
    </form>
  );
}
