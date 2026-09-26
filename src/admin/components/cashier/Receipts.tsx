"use client";

import { Search } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { BillView, methodLabel } from "@/components/cashier/BillView";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { StateBadge } from "@/components/common/StateBadge";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import type { Receipt, ReceiptPage, ReceiptSummary } from "@/lib/api/types";
import { formatTHB } from "@/lib/money";
import { useIdempotent } from "@/lib/useIdempotent";

const dateTime = new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short" });

// Historical receipts, newest first, with an exact reference search.
export function ReceiptList({ branchId }: { branchId: string }) {
  const [items, setItems] = useState<ReceiptSummary[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (after: string | null, reference: string) => {
      const params = new URLSearchParams({ limit: "25" });
      if (after) params.set("cursor", after);
      if (reference) params.set("receipt_reference", reference);
      const res = await api<ReceiptPage>(`/branches/${branchId}/settlements?${params}`);
      if (!res.ok) {
        setError(res.error.message);
        return;
      }
      setError(null);
      setItems((prev) => (after ? [...prev, ...res.data.items] : res.data.items));
      setCursor(res.data.next_cursor);
    },
    [branchId],
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(null, "");
  }, [load]);

  return (
    <>
      <PageHeader title="Receipts" description="Paid bills, newest first. Receipts never change after payment." />
      <Card>
        <CardHeader>
          <form
            role="search"
            className="flex flex-wrap items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              void load(null, query.trim().toUpperCase());
            }}
          >
            <div className="grid gap-2">
              <Label htmlFor="receipt-search">Receipt reference</Label>
              <Input id="receipt-search" value={query} placeholder="R-XXXXXXXXXX" onChange={(e) => setQuery(e.target.value)} className="w-56 font-mono" />
            </div>
            <Button type="submit" variant="outline">
              <Search aria-hidden /> Search
            </Button>
          </form>
        </CardHeader>
        <CardContent className="grid gap-4">
          <Notice notice={error ? { role: "alert", text: error } : null} />
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">Receipt</TableHead>
                <TableHead scope="col">Paid</TableHead>
                <TableHead scope="col">Table</TableHead>
                <TableHead scope="col">Method</TableHead>
                <TableHead scope="col" className="text-right">Amount</TableHead>
                <TableHead scope="col">Refund</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>
                    <Link href={`/receipts/${r.id}`} className="font-mono font-medium text-primary underline-offset-4 hover:underline">
                      {r.receipt_reference}
                    </Link>
                  </TableCell>
                  <TableCell>{dateTime.format(new Date(r.paid_at))}</TableCell>
                  <TableCell>{r.table_label}</TableCell>
                  <TableCell>{methodLabel(r.method)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatTHB(r.amount_satang)}</TableCell>
                  <TableCell>{r.refunded ? <StateBadge state="refunded" label="Refunded" /> : ""}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {items.length === 0 && !error && <p className="py-6 text-center text-sm text-muted-foreground">No receipts found.</p>}
          {cursor && (
            <Button type="button" variant="outline" className="justify-self-center" onClick={() => void load(cursor, query.trim().toUpperCase())}>
              Load older receipts
            </Button>
          )}
        </CardContent>
      </Card>
    </>
  );
}

// One immutable receipt. Managers may record the single full refund.
export function ReceiptDetail({ id, isManager }: { id: string; isManager: boolean }) {
  const run = useIdempotent();
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [notice, setNotice] = useState<NoticeValue>(null);
  const [reason, setReason] = useState("");
  const [reference, setReference] = useState("");
  const [confirming, setConfirming] = useState(false);

  const load = useCallback(async () => {
    const res = await api<Receipt>(`/settlements/${id}`);
    if (res.ok) setReceipt(res.data);
    else setNotice({ role: "alert", text: res.error.message });
  }, [id]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function refund() {
    const body = { reason: reason.trim(), external_reference: reference.trim() };
    const res = await run(`refund:${id}`, (key) => api<Receipt>(`/settlements/${id}/refund`, { method: "POST", key, body }));
    if (res.ok) {
      setReceipt(res.data);
      setNotice({ role: "status", text: "Full refund recorded. The original receipt is unchanged." });
    } else if (res.error.code === "ALREADY_REFUNDED") {
      setNotice({ role: "alert", text: "A refund was already recorded for this receipt." });
      void load();
    } else {
      setNotice({
        role: "alert",
        text: res.status === 0 || res.status >= 500 ? `${res.error.message} Retry to be sure — it will not record twice.` : res.error.message,
      });
    }
  }

  if (!receipt) return notice ? <Notice notice={notice} /> : <p role="status" className="text-sm text-muted-foreground">Loading receipt…</p>;
  const facts: [string, React.ReactNode][] = [
    ["Table", receipt.table_label],
    ["Paid", dateTime.format(new Date(receipt.paid_at))],
    ["Method", methodLabel(receipt.method)],
    ["Amount recorded", formatTHB(receipt.amount_satang)],
    ["Confirmed by", receipt.confirmed_by],
    ["Verification", receipt.verification_note],
    ...(receipt.external_reference ? ([["External reference", receipt.external_reference]] as [string, React.ReactNode][]) : []),
  ];
  return (
    <>
      <PageHeader title={`Receipt ${receipt.receipt_reference}`}
        description={<Link href="/receipts" className="underline-offset-4 hover:underline">← All receipts</Link>}
        actions={receipt.refund ? <StateBadge state="refunded" label="Refunded" /> : <StateBadge state="paid" />} />
      <Notice notice={notice} className="mb-4" />
      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <Card role="region" aria-labelledby="bill-at-payment">
          <CardHeader>
            <CardTitle>
              <h2 id="bill-at-payment">Bill at payment</h2>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <BillView lines={receipt.lines} policy={receipt.policy} totals={receipt} />
          </CardContent>
        </Card>
        <div className="grid content-start gap-6">
          <Card>
            <CardContent>
              <dl className="grid gap-3 text-sm">
                {facts.map(([k, v]) => (
                  <div key={k} className="grid gap-0.5">
                    <dt className="text-xs text-muted-foreground">{k}</dt>
                    <dd className="font-medium">{v}</dd>
                  </div>
                ))}
              </dl>
            </CardContent>
          </Card>
          <Card role="region" aria-labelledby="refund-title">
            <CardHeader>
              <CardTitle>
                <h2 id="refund-title">Refund</h2>
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm">
              {receipt.refund ? (
                <p>
                  Full refund of {formatTHB(receipt.refund.amount_satang)} recorded by {receipt.refund.recorded_by} on{" "}
                  {dateTime.format(new Date(receipt.refund.created_at))}. Reason: {receipt.refund.reason}. Reference: {receipt.refund.external_reference}.
                </p>
              ) : isManager ? (
                <form className="grid gap-3" onSubmit={(e) => { e.preventDefault(); setConfirming(true); }}>
                  <p className="text-muted-foreground">Record a refund only after the full amount has been returned outside TableFlow. Partial refunds are not supported.</p>
                  <div className="grid gap-2">
                    <Label htmlFor="refund-reason">Reason</Label>
                    <Input id="refund-reason" value={reason} maxLength={500} required onChange={(e) => setReason(e.target.value)} />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="refund-reference">Refund reference</Label>
                    <Input id="refund-reference" value={reference} maxLength={200} required onChange={(e) => setReference(e.target.value)} />
                  </div>
                  <Button type="submit" variant="destructive" disabled={!reason.trim() || !reference.trim()}>
                    Record full refund of {formatTHB(receipt.amount_satang)}
                  </Button>
                </form>
              ) : (
                <p className="text-muted-foreground">No refund. Only a manager can record one.</p>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Record a full refund of {formatTHB(receipt.amount_satang)}?</AlertDialogTitle>
            <AlertDialogDescription>Only one refund can ever be recorded for this receipt. The original receipt stays unchanged.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Back</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void refund()}>Record refund</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
