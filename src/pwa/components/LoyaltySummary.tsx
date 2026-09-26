"use client";

import { Award } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Pill } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api/client";
import { type LedgerEntry, type MemberLoyalty, TIER_LABELS } from "@/lib/api/types";
import { formatTHB } from "@/lib/money";

const date = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium" });

// The member's own balance, tier progress and history (LOY-007).
export function LoyaltySummary() {
  const [profiles, setProfiles] = useState<MemberLoyalty[] | null>(null);
  const [entries, setEntries] = useState<LedgerEntry[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState("");

  const loadEntries = useCallback(async (after: string | null) => {
    const res = await api<{ items: LedgerEntry[]; next_cursor: string | null }>(`/members/me/loyalty/entries?limit=10${after ? `&cursor=${encodeURIComponent(after)}` : ""}`);
    if (!res.ok) {
      setError(res.error.message);
      return;
    }
    setEntries((prev) => (after ? [...prev, ...res.data.items] : res.data.items));
    setCursor(res.data.next_cursor);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void api<{ items: MemberLoyalty[] }>("/members/me/loyalty").then((res) => {
      if (cancelled) return;
      if (res.ok) setProfiles(res.data.items);
      else setError(res.error.message);
    });
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadEntries(null);
    return () => {
      cancelled = true;
    };
  }, [loadEntries]);

  return (
    <section aria-labelledby="loyalty-title" className="grid gap-3">
      <h2 id="loyalty-title" className="flex items-center gap-2 text-lg font-bold">
        <Award className="size-5 text-primary" aria-hidden /> Points & tier
      </h2>
      <Notice notice={error ? { role: "alert", text: error } : null} />
      {profiles?.length === 0 && (
        <p className="rounded-2xl bg-muted p-4 text-sm">No points yet. Add a visit to your points from the table page before paying.</p>
      )}
      {profiles?.map((p) => {
        const progress = p.next_threshold_satang ? Math.min(100, Math.round((p.qualifying_spend_satang / p.next_threshold_satang) * 100)) : 100;
        return (
          <div key={p.branch_id} className="grid gap-3 rounded-2xl bg-primary p-4 text-primary-foreground">
            <div className="flex items-center justify-between">
              <span className="text-sm">{p.branch_name}</span>
              <span className="rounded-full bg-primary-foreground px-2.5 py-0.5 text-xs font-semibold text-primary">{TIER_LABELS[p.tier]}</span>
            </div>
            <p className="text-4xl font-extrabold tabular-nums">
              {p.points} <span className="text-base font-medium">points</span>
            </p>
            {p.next_tier && p.next_threshold_satang ? (
              <div className="grid gap-1.5">
                <div className="h-2 overflow-hidden rounded-full bg-primary-foreground/30" role="progressbar" aria-valuemin={0} aria-valuemax={100}
                  aria-valuenow={progress} aria-label={`Progress to ${TIER_LABELS[p.next_tier]}`}>
                  <div className="h-full rounded-full bg-primary-foreground" style={{ width: `${progress}%` }} />
                </div>
                <p className="text-xs">
                  {formatTHB(Math.max(0, p.next_threshold_satang - p.qualifying_spend_satang))} more to {TIER_LABELS[p.next_tier]}
                </p>
              </div>
            ) : (
              <p className="text-xs">Top tier reached.</p>
            )}
            {p.discount_bp > 0 && <p className="text-sm">Your member discount: {(p.discount_bp / 100).toString()}%</p>}
          </div>
        );
      })}
      {entries.length > 0 && (
        <>
          <h3 className="pt-2 font-semibold">History</h3>
          <ul className="divide-y rounded-2xl border bg-card">
            {entries.map((e) => (
              <li key={e.id} className="flex items-center gap-3 p-3 text-sm">
                <span className="grid flex-1 gap-0.5">
                  <span className="font-medium">{e.kind === "earn" ? "Points earned" : "Refund reversal"}</span>
                  <span className="text-xs text-muted-foreground">
                    {date.format(new Date(e.created_at))} · {e.receipt_reference} · {e.branch_name}
                  </span>
                </span>
                <Pill tone={e.kind === "earn" ? "success" : "destructive"}>
                  {e.points > 0 ? "+" : ""}
                  {e.points} pts
                </Pill>
              </li>
            ))}
          </ul>
          {cursor && (
            <Button type="button" variant="outline" className="h-11 rounded-xl" onClick={() => void loadEntries(cursor)}>
              Show older
            </Button>
          )}
        </>
      )}
    </section>
  );
}
