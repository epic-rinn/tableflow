"use client";

import { useCallback, useEffect, useState } from "react";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { DateRange } from "@/components/reports/DateRange";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import type { AuditEvent, AuditPage } from "@/lib/api/types";
import { businessDate } from "@/lib/dates";

const time = new Intl.DateTimeFormat("en-GB", { dateStyle: "short", timeStyle: "medium" });

function details(d: Record<string, unknown>) {
  return Object.entries(d)
    .map(([k, v]) => `${k.replaceAll("_", " ")}: ${typeof v === "object" ? JSON.stringify(v) : String(v)}`)
    .join(" · ");
}

// Audit viewer (OPS-001): newest first, filterable by action family.
export function AuditLog({ branchId }: { branchId: string }) {
  const [filter, setFilter] = useState({ from: businessDate(-6), to: businessDate(), action: "" });
  const [items, setItems] = useState<AuditEvent[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [action, setAction] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async (f: typeof filter, after: string | null) => {
    const q = new URLSearchParams({ from: f.from, to: f.to, limit: "50" });
    if (f.action) q.set("action", f.action);
    if (after) q.set("cursor", after);
    const res = await api<AuditPage>(`/branches/${branchId}/audit-events?${q}`);
    if (!res.ok) {
      setError(res.error.message + Object.values(res.error.fields ?? {}).map((v) => ` ${v}`).join(""));
      return;
    }
    setError("");
    setItems((prev) => (after ? [...prev, ...res.data.items] : res.data.items));
    setCursor(res.data.next_cursor);
  }, [branchId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(filter, null);
  }, [load, filter]);

  return (
    <>
      <PageHeader title="Audit log" description="Staff and system actions with reasons. Secrets are never recorded or shown." />
      <div className="mb-6">
        <DateRange from={filter.from} to={filter.to} onApply={(from, to) => setFilter({ from, to, action: action.trim() })}>
          <div className="grid gap-1.5">
            <Label htmlFor="audit-action">Action (optional)</Label>
            <Input id="audit-action" value={action} placeholder="e.g. settlement" onChange={(e) => setAction(e.target.value)} className="w-48" />
          </div>
        </DateRange>
      </div>
      <Notice notice={error ? { role: "alert", text: error } : null} className="mb-4" />
      <Card>
        <CardContent className="grid gap-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">Time</TableHead>
                <TableHead scope="col">Actor</TableHead>
                <TableHead scope="col">Action</TableHead>
                <TableHead scope="col">Reason</TableHead>
                <TableHead scope="col">Details</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((e) => (
                <TableRow key={e.id}>
                  <TableCell className="whitespace-nowrap tabular-nums">{time.format(new Date(e.occurred_at))}</TableCell>
                  <TableCell>{e.actor}</TableCell>
                  <TableCell className="font-mono text-xs">{e.action}</TableCell>
                  <TableCell className="max-w-56 whitespace-normal">{e.reason ?? ""}</TableCell>
                  <TableCell className="max-w-96 whitespace-normal text-xs text-muted-foreground">{details(e.details)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {items.length === 0 && !error && <p className="py-6 text-center text-sm text-muted-foreground">No events in this range.</p>}
          {cursor && (
            <Button type="button" variant="outline" className="justify-self-center" onClick={() => void load(filter, cursor)}>
              Load older events
            </Button>
          )}
        </CardContent>
      </Card>
    </>
  );
}
