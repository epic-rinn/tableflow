"use client";

import { useCallback, useEffect, useState } from "react";
import { Freshness } from "@/components/Freshness";
import { api } from "@/lib/api/client";
import type { Assistance, Menu, MenuItem, OrdersPage, Visit } from "@/lib/api/types";
import { formatTHB } from "@/lib/money";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 10_000;
const STATE_LABELS: Record<string, string> = {
  submitted: "Sent", accepted: "Accepted", preparing: "Preparing", ready: "Ready", served: "Served",
  rejected: "Not available — not charged", cancelled: "Cancelled — not charged",
};

type CartLine = { key: string; itemId: string; name_th: string; name_en: string; quantity: number; optionIds: string[]; unit: number; label: string };

// This phone's draft cart: kept per visit in sessionStorage so a reload or
// app update does not lose it, never shared with other phones.
function useCart(visitId: string) {
  const storageKey = `tableflow.cart.${visitId}`;
  const [cart, setCart] = useState<CartLine[]>([]);
  useEffect(() => {
    try {
      const raw = sessionStorage.getItem(storageKey);
      // eslint-disable-next-line react-hooks/set-state-in-effect
      if (raw) setCart(JSON.parse(raw) as CartLine[]);
    } catch {
      // storage unavailable: keep an in-memory cart
    }
  }, [storageKey]);
  const update = (next: CartLine[]) => {
    setCart(next);
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(next));
    } catch {
      // ignore storage failures
    }
  };
  return [cart, update] as const;
}

export function Dining({ visitId }: { visitId: string }) {
  const visit = usePolling(useCallback((s: AbortSignal) => api<Visit>(`/visits/${visitId}`, { signal: s }), [visitId]), POLL_MS);
  if (!visit.data) return visit.error ? <p role="alert">{visit.error.message}</p> : <p role="status">Loading…</p>;
  return <DiningRoom visit={visit.data} />;
}

function DiningRoom({ visit }: { visit: Visit }) {
  const run = useIdempotent();
  const [cart, setCart] = useCart(visit.id);
  const [notice, setNotice] = useState<{ role: "alert" | "status"; text: string } | null>(null);
  const [changed, setChanged] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const menu = usePolling(useCallback((s: AbortSignal) => api<Menu>(`/branches/${visit.branch_id}/menu`, { signal: s }), [visit.branch_id]), 60_000);
  const orders = usePolling(useCallback((s: AbortSignal) => api<OrdersPage>(`/visits/${visit.id}/orders?limit=100`, { signal: s }), [visit.id]), POLL_MS);
  const open = visit.state === "open";
  const offline = orders.error?.code === "NETWORK";

  async function submit() {
    if (!menu.data || cart.length === 0) return;
    setBusy(true);
    setNotice(null);
    const body = { menu_revision: menu.data.revision, lines: cart.map((c) => ({ item_id: c.itemId, quantity: c.quantity, option_ids: c.optionIds })) };
    // Same cart content → same key, so a retry after a lost response returns
    // the original order instead of ordering twice.
    const res = await run(`order:${JSON.stringify(body)}`, (key) => api(`/visits/${visit.id}/orders`, { method: "POST", key, body }));
    setBusy(false);
    if (res.ok) {
      setCart([]);
      setChanged([]);
      setNotice({ role: "status", text: "Your order was sent to the kitchen." });
      orders.refresh();
      return;
    }
    if (res.error.code === "MENU_CHANGED") {
      setChanged(Object.keys(res.error.fields ?? {}));
      menu.refresh();
      setNotice({ role: "alert", text: "Some items changed or sold out. Nothing was ordered — please review the highlighted items." });
      return;
    }
    const unknown = res.status === 0 || res.status >= 500;
    setNotice({ role: "alert", text: unknown ? `${res.error.message} Tap “Send order” again — it will not be ordered twice.` : res.error.message });
  }

  const total = cart.reduce((sum, c) => sum + c.unit * c.quantity, 0);
  return (
    <>
      <h2>Table {visit.table.label}</h2>
      {!open && <p role="status">Ordering is closed for this table.</p>}
      {notice && <p role={notice.role}>{notice.text}</p>}

      {open && (
        <section aria-labelledby="menu-title">
          <h3 id="menu-title">Menu</h3>
          {(menu.data?.categories ?? []).map((c) => (
            <div key={c.id}>
              <h4>
                {c.name_th} <small>{c.name_en}</small>
              </h4>
              {c.items.map((i) => (
                <ItemCard key={i.id} item={i} disabled={offline} onAdd={(line) => setCart([...cart, line])} />
              ))}
            </div>
          ))}
        </section>
      )}

      {open && (
        <section aria-labelledby="cart-title">
          <h3 id="cart-title">Your cart (this phone)</h3>
          {cart.length === 0 ? (
            <p>Nothing added yet.</p>
          ) : (
            <>
              <ul>
                {cart.map((c) => (
                  <li key={c.key} className={changed.includes(c.itemId) ? "changed" : ""}>
                    {c.quantity}× {c.name_th} <small>{c.label}</small> · {formatTHB(c.unit * c.quantity)}
                    {changed.includes(c.itemId) && <strong> — changed or sold out</strong>}{" "}
                    <button type="button" onClick={() => setCart(cart.filter((x) => x.key !== c.key))}>
                      Remove {c.name_en}
                    </button>
                  </li>
                ))}
              </ul>
              <p>Cart total {formatTHB(total)}</p>
              <button type="button" disabled={busy || offline} onClick={() => void submit()}>
                {busy ? "Sending…" : "Send order"}
              </button>
            </>
          )}
        </section>
      )}

      <section aria-labelledby="orders-title">
        <h3 id="orders-title">Table orders</h3>
        <Freshness updatedAt={orders.updatedAt} error={orders.error} intervalMs={POLL_MS} />
        <ul>
          {(orders.data?.items ?? []).flatMap((o) =>
            o.lines.map((l) => (
              <li key={l.id} className={l.chargeable ? "" : "not-chargeable"}>
                {l.quantity}× {l.name_th} <small>{l.name_en}</small> · {formatTHB(l.line_total_satang)} · {STATE_LABELS[l.state] ?? l.state}
              </li>
            )),
          )}
        </ul>
        <p>
          Table total so far: <strong>{formatTHB(orders.data?.chargeable_total_satang ?? 0)}</strong>
        </p>
      </section>

      <AssistancePanel visitId={visit.id} disabled={offline} />
    </>
  );
}

function ItemCard({ item, disabled, onAdd }: { item: MenuItem; disabled: boolean; onAdd: (l: CartLine) => void }) {
  const [chosen, setChosen] = useState<Record<string, string[]>>({});
  const [qty, setQty] = useState(1);
  const valid = item.option_groups.every((g) => {
    const n = chosen[g.id]?.length ?? 0;
    return n >= g.min_choices && n <= g.max_choices;
  });
  const opts = item.option_groups.flatMap((g) => g.options.filter((o) => chosen[g.id]?.includes(o.id)));
  const unit = item.price_satang + opts.reduce((s, o) => s + o.price_delta_satang, 0);
  return (
    <article aria-label={item.name_en} className={item.sold_out ? "sold-out" : ""}>
      <p>
        <strong>{item.name_th}</strong> <small>{item.name_en}</small> · {formatTHB(item.price_satang)}
        {item.sold_out && <strong> — sold out</strong>}
      </p>
      {!item.sold_out && (
        <>
          {item.option_groups.map((g) => (
            <fieldset key={g.id}>
              <legend>
                {g.name_th} / {g.name_en} {g.min_choices > 0 ? "(required)" : "(optional)"}
              </legend>
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
                        return { ...prev, [g.id]: next.slice(0, g.max_choices) };
                      })
                    }
                  />{" "}
                  {o.name_th} {o.name_en}
                  {o.price_delta_satang > 0 && ` +${formatTHB(o.price_delta_satang)}`}{" "}
                </label>
              ))}
            </fieldset>
          ))}
          <label>
            Quantity for {item.name_en} <input type="number" min={1} max={20} value={qty} onChange={(e) => setQty(Math.min(20, Math.max(1, Number(e.target.value))))} />
          </label>{" "}
          <button
            type="button"
            disabled={!valid || disabled}
            onClick={() => {
              onAdd({ key: crypto.randomUUID(), itemId: item.id, name_th: item.name_th, name_en: item.name_en, quantity: qty,
                optionIds: opts.map((o) => o.id), unit, label: opts.map((o) => o.name_en).join(", ") });
              setChosen({});
              setQty(1);
            }}>
            Add {item.name_en}
          </button>
        </>
      )}
    </article>
  );
}

const TOPICS: { topic: Assistance["topic"]; label: string }[] = [
  { topic: "help", label: "Call staff" },
  { topic: "allergy", label: "Allergy question" },
  { topic: "checkout", label: "Ask for the bill" },
];
const ASSIST_STATE: Record<Assistance["state"], string> = {
  open: "waiting for staff", acknowledged: "staff are on the way", resolved: "done",
};

function AssistancePanel({ visitId, disabled }: { visitId: string; disabled: boolean }) {
  const run = useIdempotent();
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const requests = usePolling(useCallback((s: AbortSignal) => api<{ items: Assistance[] }>(`/visits/${visitId}/assistance`, { signal: s }), [visitId]), POLL_MS);
  async function raise(topic: Assistance["topic"]) {
    const body = { topic, note: topic === "allergy" && note.trim() ? note.trim() : null };
    const res = await run(`assist:${topic}:${body.note ?? ""}`, (key) => api(`/visits/${visitId}/assistance`, { method: "POST", key, body }));
    setError(res.ok ? "" : res.error.message);
    requests.refresh();
  }
  const outstanding = (requests.data?.items ?? []).filter((a) => a.state !== "resolved");
  return (
    <section aria-labelledby="help-title">
      <h3 id="help-title">Need something?</h3>
      <label>
        Allergy details (optional) <input value={note} maxLength={500} onChange={(e) => setNote(e.target.value)} />
      </label>
      <p>
        {TOPICS.map((t) => (
          <button key={t.topic} type="button" disabled={disabled} onClick={() => void raise(t.topic)}>
            {t.label}
          </button>
        ))}
      </p>
      <p className="small">Staff will talk to you about allergies; the app cannot confirm that a dish is safe.</p>
      {error && <p role="alert">{error}</p>}
      <ul aria-live="polite">
        {outstanding.map((a) => (
          <li key={a.id}>
            {TOPICS.find((t) => t.topic === a.topic)?.label}: {ASSIST_STATE[a.state]}
          </li>
        ))}
      </ul>
    </section>
  );
}
