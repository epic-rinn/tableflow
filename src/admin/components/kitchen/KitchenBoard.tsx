"use client";

import { Clock, StickyNote } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { ReasonDialog } from "@/components/common/ReasonDialog";
import { StateBadge } from "@/components/common/StateBadge";
import { Freshness } from "@/components/Freshness";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
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

  const columns: { state: LineState; title: string }[] = [
    { state: "submitted", title: "New" },
    { state: "accepted", title: "Accepted" },
    { state: "preparing", title: "Preparing" },
    { state: "ready", title: "Ready to serve" },
  ];
  return (
    <>
      <PageHeader title="Kitchen" description="Oldest first. Advance each line as it moves through the kitchen."
        actions={<Freshness updatedAt={board.updatedAt} error={board.error} intervalMs={POLL_MS} />} />
      <Notice notice={notice ? { role: "alert", text: notice } : null} className="mb-4" />
      {lines.length === 0 && <p className="mb-6 rounded-xl border border-dashed p-8 text-center text-muted-foreground">No open orders.</p>}
      <div className={lines.length === 0 ? "hidden" : "mb-8 grid gap-4 md:grid-cols-2 xl:grid-cols-4"}>
        {columns.map((c) => {
          const col = lines.filter((l) => l.state === c.state);
          return (
            <section key={c.state} aria-label={`${c.title} (${col.length})`} className="grid content-start gap-3 rounded-xl bg-muted/50 p-3">
              <h2 className="flex items-center justify-between px-1 text-sm font-semibold">
                {c.title} <StateBadge state={c.state} label={String(col.length)} />
              </h2>
              <ul className="grid gap-3">
                {col.map((l) => (
                  <li key={`${l.id}:${l.version}`}>
                    <KitchenCard line={l} disabled={offline} isManager={isManager} onMove={(to, reason) => void move(l, to, reason)} />
                  </li>
                ))}
              </ul>
            </section>
          );
        })}
      </div>
      <SoldOutToggles branchId={branchId} />
    </>
  );
}

// Minutes since the line was submitted, refreshed every 30 s.
function Elapsed({ since }: { since: string }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(t);
  }, []);
  const minutes = Math.max(0, Math.floor((now - new Date(since).getTime()) / 60_000));
  return (
    <span className={`inline-flex items-center gap-1 text-xs tabular-nums ${minutes >= 15 ? "font-semibold text-destructive" : "text-muted-foreground"}`}>
      <Clock className="size-3.5" aria-hidden /> {minutes} min
    </span>
  );
}

function KitchenCard({ line: l, disabled, isManager, onMove }: { line: KitchenLine; disabled: boolean; isManager: boolean; onMove: (to: LineState, reason?: string) => void }) {
  const next = NEXT[l.state];
  const canCancel = l.state === "submitted" || l.state === "accepted" || isManager;
  const reasonLabel = `Reason for ${l.name_en} at table ${l.table_label}`;
  return (
    <Card role="article" aria-label={`${l.name_en} for table ${l.table_label}`} size="sm">
      <CardHeader>
        <CardTitle className="flex items-start justify-between gap-2">
          <h3 className="text-base leading-snug">
            <span className="tabular-nums">{l.quantity}×</span> <span lang="th">{l.name_th}</span>
            <span className="block text-sm font-normal text-muted-foreground">{l.name_en}</span>
          </h3>
          <span className="rounded-md bg-secondary px-2 py-1 text-xs font-semibold">T {l.table_label}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="grid gap-2 text-sm">
        {l.options.length > 0 && <p>{l.options.map((o) => o.name_en).join(", ")}</p>}
        {l.note && (
          <p className="flex gap-1.5 rounded-md bg-warning-soft px-2 py-1 text-warning">
            <StickyNote className="mt-0.5 size-3.5 shrink-0" aria-hidden /> Note: {l.note}
          </p>
        )}
        <div className="flex items-center justify-between">
          <StateBadge state={l.state} />
          <Elapsed since={l.created_at} />
        </div>
      </CardContent>
      <CardFooter className="flex flex-wrap gap-2">
        {next && (
          <Button type="button" disabled={disabled} onClick={() => onMove(next.to)} className="flex-1">
            {next.label}
          </Button>
        )}
        {l.state === "submitted" && (
          <ReasonDialog trigger="Reject" title={`Reject ${l.name_en}?`} description="The guest sees this reason. The line is not charged."
            reasonLabel={reasonLabel} confirmLabel="Reject line" destructive disabled={disabled} onConfirm={(r) => onMove("rejected", r)} />
        )}
        {canCancel && (
          <ReasonDialog trigger="Cancel line" title={`Cancel ${l.name_en}?`}
            description={l.state === "submitted" || l.state === "accepted" ? "The line is not charged." : "Late cancellation is audited."}
            reasonLabel={reasonLabel} confirmLabel="Cancel this line" destructive disabled={disabled} onConfirm={(r) => onMove("cancelled", r)} />
        )}
      </CardFooter>
    </Card>
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
    <Card role="region" aria-labelledby="soldout-title">
      <CardHeader>
        <CardTitle>
          <h2 id="soldout-title">Availability</h2>
        </CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3">
        <Notice notice={notice ? { role: "alert", text: notice } : null} />
        <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((i) => (
            <li key={`${i.id}:${i.version}`} className="flex flex-wrap items-center gap-2 rounded-lg border p-2 text-sm">
              <span className="min-w-0 flex-1 basis-full sm:basis-auto">
                <span lang="th">{i.name_th}</span> <span className="text-muted-foreground">({i.name_en})</span>
              </span>
              <StateBadge state={i.sold_out ? "rejected" : "available"} label={i.sold_out ? "sold out" : "available"} />
              {/* The state shown is the server's; the button states the action. */}
              <Button type="button" size="xs" variant="outline" onClick={() => void toggle(i.id, i.version, !i.sold_out)}>
                {i.sold_out ? `Mark ${i.name_en} available` : `Mark ${i.name_en} sold out`}
              </Button>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
