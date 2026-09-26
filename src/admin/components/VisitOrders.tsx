"use client";

import { useCallback, useState } from "react";
import { Freshness } from "@/components/Freshness";
import { api } from "@/lib/api/client";
import type { Menu, MenuItem, OrderLine, OrdersPage, Visit } from "@/lib/api/types";
import { formatTHB } from "@/lib/money";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 3000;
type Draft = { item: MenuItem; quantity: number; optionIds: string[] };

// Staff view of one visit: shared orders with line states and cancellation,
// and an assisted order composer that records the staff member as actor.
export function VisitOrders({ branchId, visitId, isManager }: { branchId: string; visitId: string; isManager: boolean }) {
  const run = useIdempotent();
  const [notice, setNotice] = useState<{ role: "alert" | "status"; text: string } | null>(null);
  const orders = usePolling(useCallback((s: AbortSignal) => api<OrdersPage>(`/visits/${visitId}/orders?limit=100`, { signal: s }), [visitId]), POLL_MS);
  const visit = usePolling(useCallback((s: AbortSignal) => api<Visit>(`/visits/${visitId}`, { signal: s }), [visitId]), POLL_MS);
  const menu = usePolling(useCallback((s: AbortSignal) => api<Menu>(`/branches/${branchId}/menu`, { signal: s }), [branchId]), 30_000);
  const [draft, setDraft] = useState<Draft[]>([]);

  async function cancel(l: OrderLine, reason: string) {
    const res = await run(`cancel:${l.id}:${l.version}`, (key) =>
      api(`/order-lines/${l.id}/transition`, { method: "POST", key, body: { expected_version: l.version, to_state: "cancelled", reason } }));
    setNotice(res.ok ? { role: "status", text: "Line cancelled." } : { role: "alert", text: res.error.message });
    orders.refresh();
  }

  async function submit() {
    if (!menu.data || draft.length === 0) return;
    const body = {
      menu_revision: menu.data.revision,
      lines: draft.map((d) => ({ item_id: d.item.id, quantity: d.quantity, option_ids: d.optionIds })),
    };
    const res = await run(`order:${JSON.stringify(body)}`, (key) => api(`/visits/${visitId}/orders`, { method: "POST", key, body }));
    if (res.ok) {
      setDraft([]);
      setNotice({ role: "status", text: "Order sent to the kitchen." });
    } else {
      setNotice({ role: "alert", text: res.status === 0 || res.status >= 500 ? `${res.error.message} Send again to be sure — it will not duplicate.` : res.error.message });
      if (res.error.code === "MENU_CHANGED") menu.refresh();
    }
    orders.refresh();
  }

  const items = (menu.data?.categories ?? []).flatMap((c) => c.items.filter((i) => !i.sold_out));
  return (
    <>
      <h1>Table {visit.data?.table.label ?? "…"} orders</h1>
      <Freshness updatedAt={orders.updatedAt} error={orders.error} intervalMs={POLL_MS} />
      {notice && <p role={notice.role}>{notice.text}</p>}
      <p>
        Total so far: <strong>{formatTHB(orders.data?.chargeable_total_satang ?? 0)}</strong>
      </p>
      <ol>
        {(orders.data?.items ?? []).flatMap((o) =>
          o.lines.map((l) => (
            <li key={`${l.id}:${l.version}`}>
              <LineRow line={l} actor={o.actor} canCancelLate={isManager} onCancel={(r) => void cancel(l, r)} />
            </li>
          )),
        )}
      </ol>

      <section aria-labelledby="assist-title">
        <h2 id="assist-title">Assisted order</h2>
        {items.map((i) => (
          <ItemPicker key={`${i.id}:${i.version}`} item={i} onAdd={(d) => setDraft((prev) => [...prev, d])} />
        ))}
        {draft.length > 0 && (
          <>
            <ul>
              {draft.map((d, i) => (
                <li key={i}>
                  {d.quantity}× {d.item.name_en}{" "}
                  <button type="button" onClick={() => setDraft((prev) => prev.filter((_, j) => j !== i))}>Remove</button>
                </li>
              ))}
            </ul>
            <button type="button" onClick={() => void submit()} disabled={visit.data?.state !== "open"}>
              Send order
            </button>
          </>
        )}
      </section>
    </>
  );
}

function LineRow({ line: l, actor, canCancelLate, onCancel }: { line: OrderLine; actor: string; canCancelLate: boolean; onCancel: (reason: string) => void }) {
  const [reason, setReason] = useState("");
  const cancellable = l.state === "submitted" || l.state === "accepted" || (canCancelLate && ["preparing", "ready", "served"].includes(l.state));
  return (
    <span className={l.chargeable ? "" : "not-chargeable"}>
      {l.quantity}× {l.name_en} {l.options.length > 0 && `(${l.options.map((o) => o.name_en).join(", ")})`} · {formatTHB(l.line_total_satang)} · {l.state}
      {!l.chargeable && ` — not charged${l.reason ? `: ${l.reason}` : ""}`} · by {actor}{" "}
      {cancellable && (
        <>
          <label>
            <span className="sr-only">Cancel reason for {l.name_en}</span>
            <input placeholder="Cancel reason" value={reason} onChange={(e) => setReason(e.target.value)} />
          </label>{" "}
          <button type="button" disabled={!reason.trim()} onClick={() => onCancel(reason)}>Cancel</button>
        </>
      )}
    </span>
  );
}

function ItemPicker({ item, onAdd }: { item: MenuItem; onAdd: (d: Draft) => void }) {
  const [qty, setQty] = useState(1);
  const [chosen, setChosen] = useState<Record<string, string[]>>({});
  const valid = item.option_groups.every((g) => {
    const n = chosen[g.id]?.length ?? 0;
    return n >= g.min_choices && n <= g.max_choices;
  });
  return (
    <fieldset>
      <legend>
        {item.name_th} ({item.name_en}) · {formatTHB(item.price_satang)}
      </legend>
      {item.option_groups.map((g) => (
        <p key={g.id}>
          {g.name_en} ({g.min_choices}–{g.max_choices}):{" "}
          {g.options.map((o) => (
            <label key={o.id}>
              <input
                type={g.max_choices === 1 ? "radio" : "checkbox"}
                name={`${item.id}-${g.id}`}
                checked={chosen[g.id]?.includes(o.id) ?? false}
                onChange={(e) =>
                  setChosen((prev) => {
                    const cur = prev[g.id] ?? [];
                    const next = g.max_choices === 1 ? [o.id] : e.target.checked ? [...cur, o.id] : cur.filter((x) => x !== o.id);
                    return { ...prev, [g.id]: next };
                  })
                }
              />{" "}
              {o.name_en}
              {o.price_delta_satang > 0 && ` +${formatTHB(o.price_delta_satang)}`}{" "}
            </label>
          ))}
        </p>
      ))}
      <label>
        Qty <input type="number" min={1} max={20} value={qty} onChange={(e) => setQty(Number(e.target.value))} />
      </label>{" "}
      <button type="button" disabled={!valid} onClick={() => onAdd({ item, quantity: qty, optionIds: Object.values(chosen).flat() })}>
        Add {item.name_en}
      </button>
    </fieldset>
  );
}
