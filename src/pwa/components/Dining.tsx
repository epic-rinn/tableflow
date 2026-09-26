"use client";

import { BellRing, ChevronRight, CircleHelp, Minus, Plus, ReceiptText, ShoppingBag, Trash2, TriangleAlert, Utensils } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Pill } from "@/components/common/MobileShell";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { Freshness } from "@/components/Freshness";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api/client";
import type { Assistance, Bill, Menu, MenuItem, OrdersPage, Visit } from "@/lib/api/types";
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
  const [notice, setNotice] = useState<NoticeValue>(null);
  const [cartOpen, setCartOpen] = useState(false);
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
    setCartOpen(false);
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
  const count = cart.reduce((n, c) => n + c.quantity, 0);
  const categories = menu.data?.categories ?? [];
  const setQuantity = (key: string, q: number) => setCart(cart.map((c) => (c.key === key ? { ...c, quantity: Math.min(20, Math.max(1, q)) } : c)));
  return (
    <div className={cn("grid gap-5", open && cart.length > 0 && "pb-24")}>
      <section aria-labelledby="table-title" className="flex items-center gap-3 rounded-2xl border bg-card p-4 shadow-sm">
        <span className="flex size-12 items-center justify-center rounded-2xl bg-accent text-accent-foreground">
          <Utensils className="size-6" aria-hidden />
        </span>
        <div className="grid flex-1 gap-0.5">
          <h2 id="table-title" className="text-lg font-bold">Table {visit.table.label}</h2>
          <Freshness updatedAt={orders.updatedAt} error={orders.error} intervalMs={POLL_MS} />
        </div>
        <Pill tone={open ? "success" : visit.state === "settling" ? "warning" : "muted"}>{open ? "Ordering open" : visit.state === "settling" ? "Settling" : "Closed"}</Pill>
      </section>
      {visit.state === "settling" && <p role="status" className="rounded-2xl bg-warning-soft p-3 text-sm text-warning">Your bill is being settled at the counter, so ordering is paused.</p>}
      {!open && visit.state !== "settling" && <p role="status" className="rounded-2xl bg-muted p-3 text-sm">Ordering is closed for this table.</p>}
      <Notice notice={notice} />

      {open && (
        <section aria-labelledby="menu-title" className="grid gap-3">
          <h3 id="menu-title" className="text-lg font-bold">Menu</h3>
          <nav aria-label="Menu categories" className="no-scrollbar sticky top-0 z-10 -mx-4 flex gap-2 overflow-x-auto bg-background/95 px-4 py-2 backdrop-blur">
            {categories.map((c) => (
              <Button key={c.id} type="button" variant="outline" size="sm" className="h-9 shrink-0 rounded-full"
                onClick={() => document.getElementById(`cat-${c.id}`)?.scrollIntoView({ behavior: "smooth", block: "start" })}>
                <span lang="th">{c.name_th}</span>
              </Button>
            ))}
          </nav>
          {categories.map((c) => (
            <div key={c.id} id={`cat-${c.id}`} className="grid scroll-mt-14 gap-1">
              <h4 className="pt-2 text-base font-semibold">
                <span lang="th">{c.name_th}</span> <small className="font-normal text-muted-foreground">{c.name_en}</small>
              </h4>
              <ul className="divide-y">
                {c.items.map((i) => (
                  <li key={i.id}>
                    <ItemRow item={i} disabled={offline} changed={changed.includes(i.id)} onAdd={(line) => setCart([...cart, line])} />
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
      )}

      <section aria-labelledby="orders-title" className="grid gap-3">
        <h3 id="orders-title" className="text-lg font-bold">Table orders</h3>
        {(orders.data?.items ?? []).length === 0 ? (
          <p className="rounded-2xl border border-dashed p-4 text-center text-sm text-muted-foreground">No orders yet.</p>
        ) : (
          <ol className="grid gap-3">
            {(orders.data?.items ?? []).map((o) => (
              <li key={o.id} className="rounded-2xl border bg-card p-3">
                <p className="mb-2 text-xs text-muted-foreground">Ordered {new Date(o.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</p>
                <ul className="grid gap-2">
                  {o.lines.map((l) => (
                    <li key={l.id} className={cn("flex items-start gap-2 text-sm", !l.chargeable && "text-muted-foreground")}>
                      <span className="w-6 shrink-0 font-semibold tabular-nums">{l.quantity}×</span>
                      <span className="min-w-0 flex-1">
                        <span lang="th" className={cn(!l.chargeable && "line-through")}>{l.name_th}</span> <small className="text-muted-foreground">{l.name_en}</small>
                        <span className="block">
                          <Pill tone={LINE_TONES[l.state] ?? "muted"}>{STATE_LABELS[l.state] ?? l.state}</Pill>
                        </span>
                      </span>
                      <span className="tabular-nums">{formatTHB(l.line_total_satang)}</span>
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ol>
        )}
        <p className="flex items-center justify-between rounded-2xl bg-muted p-3 text-sm">
          Table total so far: <strong className="text-base tabular-nums">{formatTHB(orders.data?.chargeable_total_satang ?? 0)}</strong>
        </p>
      </section>

      <BillPanel visitId={visit.id} />
      <AssistancePanel visitId={visit.id} disabled={offline} />

      {open && cart.length > 0 && (
        <div className="fixed inset-x-0 bottom-0 z-20 mx-auto max-w-md px-4 pb-safe">
          <Button type="button" size="lg" onClick={() => setCartOpen(true)}
            className={cn("h-14 w-full justify-between rounded-2xl px-4 text-base shadow-lg", changed.length > 0 && "bg-warning hover:bg-warning/90")}>
            <span className="flex items-center gap-2">
              {changed.length > 0 ? <TriangleAlert aria-hidden /> : <ShoppingBag aria-hidden />}
              View cart · {count} {count === 1 ? "item" : "items"}
            </span>
            <span className="tabular-nums">{formatTHB(total)}</span>
          </Button>
        </div>
      )}
      <Sheet open={cartOpen} onOpenChange={setCartOpen}>
        <SheetContent side="bottom" className="mx-auto max-h-[85dvh] max-w-md rounded-t-3xl">
          <SheetHeader>
            <SheetTitle>Your cart</SheetTitle>
            <SheetDescription>Only on this phone until you send it. Everyone at the table sees sent orders.</SheetDescription>
          </SheetHeader>
          <section aria-labelledby="cart-title" className="grid gap-3 overflow-y-auto px-4">
            <h3 id="cart-title" className="sr-only">Your cart (this phone)</h3>
            {cart.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">Nothing added yet.</p>
            ) : (
              <ul className="grid gap-3">
                {cart.map((c) => (
                  <li key={c.key} className={cn("grid gap-2 rounded-2xl border p-3", changed.includes(c.itemId) && "border-warning bg-warning-soft")}>
                    <div className="flex items-start gap-2">
                      <span className="min-w-0 flex-1 text-sm">
                        <span lang="th" className="font-medium">{c.name_th}</span> <small className="text-muted-foreground">{c.label}</small>
                        {changed.includes(c.itemId) && <strong className="block text-warning">Changed or sold out — remove or re-add it</strong>}
                      </span>
                      <span className="text-sm font-semibold tabular-nums">{formatTHB(c.unit * c.quantity)}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Stepper value={c.quantity} label={c.name_en} onChange={(q) => setQuantity(c.key, q)} />
                      <Button type="button" variant="ghost" size="sm" className="ml-auto text-destructive" onClick={() => setCart(cart.filter((x) => x.key !== c.key))}>
                        <Trash2 aria-hidden /> Remove {c.name_en}
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>
          <SheetFooter>
            <Button type="button" size="lg" className="h-14 w-full justify-between rounded-2xl px-4 text-base" disabled={busy || offline || cart.length === 0} onClick={() => void submit()}>
              <span>{busy ? "Sending…" : "Send order"}</span>
              <span className="tabular-nums">{formatTHB(total)}</span>
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  );
}

const LINE_TONES: Record<string, "success" | "warning" | "info" | "destructive" | "muted" | "primary"> = {
  submitted: "info", accepted: "primary", preparing: "warning", ready: "success", served: "muted", rejected: "destructive", cancelled: "destructive",
};

function Stepper({ value, label, onChange }: { value: number; label: string; onChange: (v: number) => void }) {
  return (
    <div className="flex items-center gap-1" role="group" aria-label={`Quantity of ${label}`}>
      <Button type="button" variant="outline" size="icon" className="size-11 rounded-full" aria-label="Decrease quantity" disabled={value <= 1} onClick={() => onChange(value - 1)}>
        <Minus aria-hidden />
      </Button>
      <span className="w-8 text-center font-semibold tabular-nums" aria-live="polite">{value}</span>
      <Button type="button" variant="outline" size="icon" className="size-11 rounded-full" aria-label="Increase quantity" disabled={value >= 20} onClick={() => onChange(value + 1)}>
        <Plus aria-hidden />
      </Button>
    </div>
  );
}

const percent = (bp: number) => (bp / 100).toFixed(2).replace(/\.?0+$/, "");

// The itemised bill as Go calculated it (BIL-001/003); read-only. Paying
// happens with staff at the counter — nothing here can settle the bill.
function BillPanel({ visitId }: { visitId: string }) {
  const [shown, setShown] = useState(false);
  return (
    <section aria-labelledby="bill-title" className="grid gap-3 rounded-2xl border bg-card p-4">
      <div className="flex items-center justify-between">
        <h3 id="bill-title" className="flex items-center gap-2 text-lg font-bold">
          <ReceiptText className="size-5 text-muted-foreground" aria-hidden /> Bill
        </h3>
        <Button type="button" variant="ghost" size="sm" aria-expanded={shown} onClick={() => setShown(!shown)}>
          {shown ? "Hide bill" : "View bill"} <ChevronRight className={cn("transition-transform", shown && "rotate-90")} aria-hidden />
        </Button>
      </div>
      {shown && <BillDetails visitId={visitId} />}
    </section>
  );
}

function BillDetails({ visitId }: { visitId: string }) {
  const bill = usePolling(useCallback((s: AbortSignal) => api<Bill>(`/visits/${visitId}/bill`, { signal: s }), [visitId]), POLL_MS);
  const b = bill.data;
  if (!b) return bill.error ? <Notice notice={{ role: "alert", text: bill.error.message }} /> : <p role="status" className="text-sm text-muted-foreground">Loading bill…</p>;
  return (
    <div className="grid gap-3">
      <Freshness updatedAt={bill.updatedAt} error={bill.error} intervalMs={POLL_MS} />
      <ul className="grid gap-1.5 text-sm">
        {b.lines.map((l) => (
          <li key={l.id} className="flex gap-2">
            <span className="w-6 shrink-0 tabular-nums">{l.quantity}×</span>
            <span className="min-w-0 flex-1">
              <span lang="th">{l.name_th}</span> <small className="text-muted-foreground">{l.name_en}</small>
            </span>
            <span className="tabular-nums">{formatTHB(l.line_total_satang)}</span>
          </li>
        ))}
      </ul>
      <Separator className="border-dashed" />
      <dl className="grid grid-cols-[1fr_auto] gap-y-1 text-sm">
        <dt className="text-muted-foreground">Subtotal</dt>
        <dd className="text-right tabular-nums">{formatTHB(b.gross_satang)}</dd>
        {b.discount_satang > 0 && (
          <>
            <dt className="text-muted-foreground">Discount</dt>
            <dd className="text-right tabular-nums">−{formatTHB(b.discount_satang)}</dd>
          </>
        )}
        <dt className="text-muted-foreground">Service charge ({percent(b.policy.service_bp)}%)</dt>
        <dd className="text-right tabular-nums">{formatTHB(b.service_satang)}</dd>
        <dt className="text-muted-foreground">{b.policy.tax_mode === "inclusive" ? `Tax included (${percent(b.policy.tax_bp)}%)` : `Tax (${percent(b.policy.tax_bp)}%)`}</dt>
        <dd className="text-right tabular-nums">{formatTHB(b.tax_satang)}</dd>
        <dt className="pt-2 text-base font-bold">Total</dt>
        <dd className="pt-2 text-right text-base font-bold tabular-nums">{formatTHB(b.total_satang)}</dd>
      </dl>
      {!b.frozen && b.unresolved_lines > 0 && <p className="text-xs text-muted-foreground">{b.unresolved_lines} item(s) are still being prepared; the total may change.</p>}
      <p className="rounded-xl bg-accent p-3 text-sm text-accent-foreground">Please pay a staff member at the counter. Showing a transfer slip here does not complete payment.</p>
    </div>
  );
}

// One menu row: tap "+" to add plain items at once or open the options sheet.
function ItemRow({ item, disabled, changed, onAdd }: { item: MenuItem; disabled: boolean; changed: boolean; onAdd: (l: CartLine) => void }) {
  const [open, setOpen] = useState(false);
  const add = (optionIds: string[], qty: number) => {
    const opts = item.option_groups.flatMap((g) => g.options.filter((o) => optionIds.includes(o.id)));
    const unit = item.price_satang + opts.reduce((s, o) => s + o.price_delta_satang, 0);
    onAdd({ key: crypto.randomUUID(), itemId: item.id, name_th: item.name_th, name_en: item.name_en, quantity: qty,
      optionIds, unit, label: opts.map((o) => o.name_en).join(", ") });
  };
  return (
    <article aria-label={item.name_en} className={cn("flex items-center gap-3 py-3", item.sold_out && "opacity-60", changed && "rounded-xl bg-warning-soft px-2")}>
      <span className="flex size-16 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground" aria-hidden>
        <Utensils className="size-6" />
      </span>
      <div className="grid min-w-0 flex-1 gap-0.5">
        <p className="font-semibold" lang="th">{item.name_th}</p>
        <p className="text-sm text-muted-foreground">{item.name_en}</p>
        <p className="text-sm font-semibold tabular-nums">
          {formatTHB(item.price_satang)}
          {item.sold_out && <Pill tone="destructive"> sold out</Pill>}
        </p>
      </div>
      {!item.sold_out && (
        <Button type="button" size="icon" className="size-11 shrink-0 rounded-full" aria-label={`Add ${item.name_en}`} disabled={disabled}
          onClick={() => (item.option_groups.length === 0 ? add([], 1) : setOpen(true))}>
          <Plus className="size-5" aria-hidden />
        </Button>
      )}
      {item.option_groups.length > 0 && (
        <OptionSheet item={item} open={open} onOpenChange={setOpen} onAdd={(ids, qty) => { add(ids, qty); setOpen(false); }} />
      )}
    </article>
  );
}

function OptionSheet({ item, open, onOpenChange, onAdd }: {
  item: MenuItem; open: boolean; onOpenChange: (o: boolean) => void; onAdd: (optionIds: string[], qty: number) => void;
}) {
  const [chosen, setChosen] = useState<Record<string, string[]>>({});
  const [qty, setQty] = useState(1);
  const valid = item.option_groups.every((g) => {
    const n = chosen[g.id]?.length ?? 0;
    return n >= g.min_choices && n <= g.max_choices;
  });
  const opts = item.option_groups.flatMap((g) => g.options.filter((o) => chosen[g.id]?.includes(o.id)));
  const unit = item.price_satang + opts.reduce((s, o) => s + o.price_delta_satang, 0);
  return (
    <Sheet open={open} onOpenChange={(o) => { onOpenChange(o); if (!o) { setChosen({}); setQty(1); } }}>
      <SheetContent side="bottom" className="mx-auto max-h-[85dvh] max-w-md rounded-t-3xl">
        <SheetHeader>
          <SheetTitle lang="th">{item.name_th}</SheetTitle>
          <SheetDescription>{item.name_en} · {formatTHB(item.price_satang)}</SheetDescription>
        </SheetHeader>
        <div className="grid gap-4 overflow-y-auto px-4">
          {item.option_groups.map((g) => (
            <fieldset key={g.id} className="grid gap-2">
              <legend className="mb-2 flex w-full items-center justify-between text-sm font-semibold">
                <span><span lang="th">{g.name_th}</span> / {g.name_en}</span>
                <Pill tone={g.min_choices > 0 ? "warning" : "muted"}>{g.min_choices > 0 ? "(required)" : "(optional)"}{g.max_choices > 1 ? ` · up to ${g.max_choices}` : ""}</Pill>
              </legend>
              {g.options.map((o) => (
                <label key={o.id} className="flex min-h-12 cursor-pointer items-center gap-3 rounded-xl border px-3 has-checked:border-primary has-checked:bg-accent">
                  <input
                    className="size-5 accent-primary"
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
                  />
                  <span className="flex-1 text-sm"><span lang="th">{o.name_th}</span> {o.name_en}</span>
                  {o.price_delta_satang > 0 && <span className="text-sm text-muted-foreground tabular-nums">+{formatTHB(o.price_delta_satang)}</span>}
                </label>
              ))}
            </fieldset>
          ))}
          <div className="flex items-center justify-between">
            <span className="text-sm font-semibold">Quantity for {item.name_en}</span>
            <Stepper value={qty} label={item.name_en} onChange={(v) => setQty(Math.min(20, Math.max(1, v)))} />
          </div>
        </div>
        <SheetFooter>
          <Button type="button" size="lg" className="h-14 w-full justify-between rounded-2xl px-4 text-base" disabled={!valid} onClick={() => onAdd(opts.map((o) => o.id), qty)}>
            <span>Add to cart</span>
            <span className="tabular-nums">{formatTHB(unit * qty)}</span>
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

const TOPICS: { topic: Assistance["topic"]; label: string; icon: typeof BellRing }[] = [
  { topic: "help", label: "Call staff", icon: BellRing },
  { topic: "allergy", label: "Allergy question", icon: CircleHelp },
  { topic: "checkout", label: "Ask for the bill", icon: ReceiptText },
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
    <section aria-labelledby="help-title" className="grid gap-3 rounded-2xl border bg-card p-4">
      <h3 id="help-title" className="text-lg font-bold">Need something?</h3>
      <div className="grid grid-cols-3 gap-2">
        {TOPICS.map((t) => {
          const Icon = t.icon;
          return (
            <Button key={t.topic} type="button" variant="outline" disabled={disabled} onClick={() => void raise(t.topic)}
              className="h-auto flex-col gap-1.5 rounded-2xl py-3 text-xs whitespace-normal">
              <Icon className="size-5 text-primary" aria-hidden />
              {t.label}
            </Button>
          );
        })}
      </div>
      <div className="grid gap-2">
        <Label htmlFor="allergy-note" className="text-sm">Allergy details (optional)</Label>
        <Input id="allergy-note" value={note} maxLength={500} onChange={(e) => setNote(e.target.value)} className="h-11" />
      </div>
      <p className="text-xs text-muted-foreground">Staff will talk to you about allergies; the app cannot confirm that a dish is safe.</p>
      <Notice notice={error ? { role: "alert", text: error } : null} />
      <ul aria-live="polite" className="grid gap-2">
        {outstanding.map((a) => (
          <li key={a.id} className="flex items-center justify-between rounded-xl bg-muted px-3 py-2 text-sm">
            <span>
              {TOPICS.find((t) => t.topic === a.topic)?.label}: {ASSIST_STATE[a.state]}
            </span>
            <Pill tone={a.state === "open" ? "warning" : "primary"}>{a.state === "open" ? "Sent" : "On the way"}</Pill>
          </li>
        ))}
      </ul>
    </section>
  );
}
