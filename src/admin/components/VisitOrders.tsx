"use client";

import { Minus, Plus, Send, Trash2 } from "lucide-react";
import { useCallback, useState } from "react";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { ReasonDialog } from "@/components/common/ReasonDialog";
import { StateBadge } from "@/components/common/StateBadge";
import { Freshness } from "@/components/Freshness";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
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
  const [notice, setNotice] = useState<NoticeValue>(null);
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
  const lines = (orders.data?.items ?? []).flatMap((o) => o.lines.map((l) => ({ line: l, actor: o.actor })));
  const draftTotal = draft.reduce((sum, d) => {
    const deltas = d.item.option_groups.flatMap((g) => g.options).filter((o) => d.optionIds.includes(o.id)).reduce((a, o) => a + o.price_delta_satang, 0);
    return sum + (d.item.price_satang + deltas) * d.quantity;
  }, 0);
  return (
    <>
      <PageHeader title={`Table ${visit.data?.table.label ?? "…"} orders`}
        description={visit.data ? <>Visit is <StateBadge state={visit.data.state} /></> : undefined}
        actions={<Freshness updatedAt={orders.updatedAt} error={orders.error} intervalMs={POLL_MS} />} />
      <Notice notice={notice} className="mb-4" />
      <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
        <Card role="region" aria-labelledby="orders-title">
          <CardHeader>
            <CardTitle>
              <h2 id="orders-title">Orders</h2>
            </CardTitle>
            <CardDescription>
              Total so far: <strong className="text-foreground">{formatTHB(orders.data?.chargeable_total_satang ?? 0)}</strong>
            </CardDescription>
          </CardHeader>
          <CardContent>
            {lines.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">No orders yet.</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Item</TableHead>
                    <TableHead>State</TableHead>
                    <TableHead>By</TableHead>
                    <TableHead className="text-right">Amount</TableHead>
                    <TableHead><span className="sr-only">Actions</span></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {lines.map(({ line: l, actor }) => (
                    <LineRow key={`${l.id}:${l.version}`} line={l} actor={actor} canCancelLate={isManager} onCancel={(r) => void cancel(l, r)} />
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card role="region" aria-labelledby="assist-title" className="self-start">
          <CardHeader>
            <CardTitle>
              <h2 id="assist-title">Assisted order</h2>
            </CardTitle>
            <CardDescription>Order for guests at the table; you are recorded as the actor.</CardDescription>
          </CardHeader>
          <CardContent className="grid max-h-[28rem] gap-3 overflow-y-auto">
            {items.map((i) => (
              <ItemPicker key={`${i.id}:${i.version}`} item={i} onAdd={(d) => setDraft((prev) => [...prev, d])} />
            ))}
          </CardContent>
          {draft.length > 0 && (
            <CardFooter className="grid gap-3">
              <ul className="grid gap-1 text-sm">
                {draft.map((d, i) => (
                  <li key={i} className="flex items-center justify-between gap-2">
                    <span>{d.quantity}× {d.item.name_en}</span>
                    <Button type="button" variant="ghost" size="icon-xs" aria-label={`Remove ${d.item.name_en}`}
                      onClick={() => setDraft((prev) => prev.filter((_, j) => j !== i))}>
                      <Trash2 aria-hidden />
                    </Button>
                  </li>
                ))}
              </ul>
              <Button type="button" onClick={() => void submit()} disabled={visit.data?.state !== "open"}>
                <Send aria-hidden /> Send order · {formatTHB(draftTotal)}
              </Button>
            </CardFooter>
          )}
        </Card>
      </div>
    </>
  );
}

function LineRow({ line: l, actor, canCancelLate, onCancel }: { line: OrderLine; actor: string; canCancelLate: boolean; onCancel: (reason: string) => void }) {
  const cancellable = l.state === "submitted" || l.state === "accepted" || (canCancelLate && ["preparing", "ready", "served"].includes(l.state));
  return (
    <TableRow className={l.chargeable ? "" : "text-muted-foreground"}>
      <TableCell className="whitespace-normal">
        <span className={l.chargeable ? "font-medium" : "line-through"}>{l.quantity}× {l.name_en}</span>
        {l.options.length > 0 && <span className="block text-xs text-muted-foreground">{l.options.map((o) => o.name_en).join(", ")}</span>}
        {!l.chargeable && <span className="block text-xs">not charged{l.reason ? `: ${l.reason}` : ""}</span>}
      </TableCell>
      <TableCell><StateBadge state={l.state} /></TableCell>
      <TableCell className="text-xs">{actor}</TableCell>
      <TableCell className="text-right tabular-nums">{formatTHB(l.line_total_satang)}</TableCell>
      <TableCell className="text-right">
        {cancellable && (
          <ReasonDialog trigger="Cancel" title={`Cancel ${l.name_en}?`} description="The line will not be charged. Late cancellations are audited."
            reasonLabel={`Cancel reason for ${l.name_en}`} confirmLabel="Cancel line" destructive onConfirm={onCancel} />
        )}
      </TableCell>
    </TableRow>
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
    <fieldset className="grid gap-2 rounded-lg border p-3">
      <legend className="px-1 text-sm font-medium">
        <span lang="th">{item.name_th}</span> ({item.name_en}) · {formatTHB(item.price_satang)}
      </legend>
      {item.option_groups.map((g) => (
        <div key={g.id} className="grid gap-1 text-sm">
          <span className="text-xs text-muted-foreground">{g.name_en} (choose {g.min_choices}–{g.max_choices})</span>
          <div className="flex flex-wrap gap-x-3 gap-y-1">
            {g.options.map((o) => (
              <label key={o.id} className="inline-flex items-center gap-1.5">
                <input
                  className="size-4 accent-primary"
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
                />
                {o.name_en}
                {o.price_delta_satang > 0 && <span className="text-muted-foreground">+{formatTHB(o.price_delta_satang)}</span>}
              </label>
            ))}
          </div>
        </div>
      ))}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1" role="group" aria-label={`Quantity of ${item.name_en}`}>
          <Button type="button" variant="outline" size="icon-xs" aria-label="Decrease quantity" disabled={qty <= 1} onClick={() => setQty(qty - 1)}>
            <Minus aria-hidden />
          </Button>
          <span className="w-6 text-center text-sm tabular-nums" aria-live="polite">{qty}</span>
          <Button type="button" variant="outline" size="icon-xs" aria-label="Increase quantity" disabled={qty >= 20} onClick={() => setQty(qty + 1)}>
            <Plus aria-hidden />
          </Button>
        </div>
        <Button type="button" size="sm" disabled={!valid} onClick={() => onAdd({ item, quantity: qty, optionIds: Object.values(chosen).flat() })}>
          Add {item.name_en}
        </Button>
      </div>
    </fieldset>
  );
}
