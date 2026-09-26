"use client";

import { useCallback, useState } from "react";
import { Freshness } from "@/components/Freshness";
import { api } from "@/lib/api/client";
import type { KitchenLine, KitchenPage, LineState, Menu, OrderLine } from "@/lib/api/types";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 3000;
const NEXT: Partial<Record<LineState, { to: LineState; label: string }>> = {
  submitted: { to: "accepted", label: "Accept" },
  accepted: { to: "preparing", label: "Start preparing" },
  preparing: { to: "ready", label: "Ready" },
  ready: { to: "served", label: "Served" },
};

export function KitchenBoard({ branchId, isManager }: { branchId: string; isManager: boolean }) {
  const run = useIdempotent();
  const [notice, setNotice] = useState("");
  const board = usePolling(
    useCallback((signal: AbortSignal) => api<KitchenPage>(`/branches/${branchId}/kitchen-lines?limit=100`, { signal }), [branchId]),
    POLL_MS,
  );
  const lines = board.data?.items ?? [];
  const offline = board.error?.code === "NETWORK";

  async function move(l: KitchenLine, to: LineState, reason?: string) {
    setNotice("");
    const res = await run(`line:${l.id}:${l.version}:${to}`, (key) =>
      api<OrderLine>(`/order-lines/${l.id}/transition`, { method: "POST", key, body: { expected_version: l.version, to_state: to, ...(reason ? { reason } : {}) } }),
    );
    if (!res.ok) setNotice(res.error.message);
    board.refresh();
  }

  return (
    <>
      <h1>Kitchen</h1>
      <Freshness updatedAt={board.updatedAt} error={board.error} intervalMs={POLL_MS} />
      {notice && <p role="alert">{notice}</p>}
      {lines.length === 0 && <p>No open orders.</p>}
      <ul className="kitchen">
        {lines.map((l) => (
          <li key={`${l.id}:${l.version}`}>
            <KitchenCard line={l} disabled={offline} isManager={isManager} onMove={(to, reason) => void move(l, to, reason)} />
          </li>
        ))}
      </ul>
      <SoldOutToggles branchId={branchId} />
    </>
  );
}

function KitchenCard({ line: l, disabled, isManager, onMove }: { line: KitchenLine; disabled: boolean; isManager: boolean; onMove: (to: LineState, reason?: string) => void }) {
  const [reason, setReason] = useState("");
  const next = NEXT[l.state];
  const canCancel = l.state === "submitted" || l.state === "accepted" || isManager;
  return (
    <article aria-label={`${l.name_en} for table ${l.table_label}`} className={`kitchen-line ${l.state}`}>
      <h3>
        {l.quantity}× {l.name_th} <small>({l.name_en})</small> · table {l.table_label}
      </h3>
      {l.options.length > 0 && <p>{l.options.map((o) => o.name_en).join(", ")}</p>}
      {l.note && <p className="note">Note: {l.note}</p>}
      <p>
        {l.state} · {new Date(l.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
      </p>
      {next && (
        <button type="button" disabled={disabled} onClick={() => onMove(next.to)}>
          {next.label}
        </button>
      )}{" "}
      <label>
        <span className="sr-only">Reason for {l.name_en} at table {l.table_label}</span>
        <input placeholder="Reason" value={reason} maxLength={500} onChange={(e) => setReason(e.target.value)} />
      </label>{" "}
      {l.state === "submitted" && (
        <button type="button" disabled={disabled || !reason.trim()} onClick={() => onMove("rejected", reason)}>
          Reject
        </button>
      )}{" "}
      {canCancel && (
        <button type="button" disabled={disabled || !reason.trim()} onClick={() => onMove("cancelled", reason)}>
          Cancel line
        </button>
      )}
    </article>
  );
}

function SoldOutToggles({ branchId }: { branchId: string }) {
  const menu = usePolling(useCallback((signal: AbortSignal) => api<Menu>(`/branches/${branchId}/menu`, { signal }), [branchId]), 15_000);
  const [notice, setNotice] = useState("");
  const items = (menu.data?.categories ?? []).flatMap((c) => c.items);
  async function toggle(id: string, version: number, soldOut: boolean) {
    const res = await api(`/menu-items/${id}/availability`, { method: "PATCH", body: { expected_version: version, sold_out: soldOut } });
    setNotice(res.ok ? "" : res.error.message);
    menu.refresh();
  }
  return (
    <section aria-labelledby="soldout-title">
      <h2 id="soldout-title">Availability</h2>
      {notice && <p role="alert">{notice}</p>}
      <ul>
        {items.map((i) => (
          <li key={`${i.id}:${i.version}`}>
            <label>
              <input type="checkbox" checked={i.sold_out} onChange={(e) => void toggle(i.id, i.version, e.target.checked)} /> Sold out: {i.name_th} ({i.name_en})
            </label>
          </li>
        ))}
      </ul>
    </section>
  );
}
