"use client";

import { useCallback, useEffect, useState } from "react";
import { BillView, methodLabel } from "@/components/cashier/BillView";
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
      <h1>Receipts</h1>
      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault();
          void load(null, query.trim().toUpperCase());
        }}
      >
        <label>
          Receipt reference <input value={query} placeholder="R-XXXXXXXXXX" onChange={(e) => setQuery(e.target.value)} />
        </label>{" "}
        <button type="submit">Search</button>
      </form>
      {error && <p role="alert">{error}</p>}
      <table>
        <thead>
          <tr>
            <th scope="col">Receipt</th>
            <th scope="col">Paid</th>
            <th scope="col">Table</th>
            <th scope="col">Method</th>
            <th scope="col">Amount</th>
            <th scope="col">Refund</th>
          </tr>
        </thead>
        <tbody>
          {items.map((r) => (
            <tr key={r.id}>
              <td>
                <a href={`/receipts/${r.id}`}>{r.receipt_reference}</a>
              </td>
              <td>{dateTime.format(new Date(r.paid_at))}</td>
              <td>{r.table_label}</td>
              <td>{methodLabel(r.method)}</td>
              <td>{formatTHB(r.amount_satang)}</td>
              <td>{r.refunded ? "Refunded" : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {items.length === 0 && !error && <p>No receipts found.</p>}
      {cursor && (
        <button type="button" onClick={() => void load(cursor, query.trim().toUpperCase())}>
          Load older receipts
        </button>
      )}
    </>
  );
}

// One immutable receipt. Managers may record the single full refund.
export function ReceiptDetail({ id, isManager }: { id: string; isManager: boolean }) {
  const run = useIdempotent();
  const [receipt, setReceipt] = useState<Receipt | null>(null);
  const [notice, setNotice] = useState<{ role: "alert" | "status"; text: string } | null>(null);
  const [reason, setReason] = useState("");
  const [reference, setReference] = useState("");

  const load = useCallback(async () => {
    const res = await api<Receipt>(`/settlements/${id}`);
    if (res.ok) setReceipt(res.data);
    else setNotice({ role: "alert", text: res.error.message });
  }, [id]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function refund(e: React.FormEvent) {
    e.preventDefault();
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

  if (!receipt) return notice ? <p role={notice.role}>{notice.text}</p> : <p>Loading receipt…</p>;
  return (
    <>
      <h1>Receipt {receipt.receipt_reference}</h1>
      {notice && <p role={notice.role}>{notice.text}</p>}
      <dl className="totals">
        <dt>Table</dt>
        <dd>{receipt.table_label}</dd>
        <dt>Paid</dt>
        <dd>{dateTime.format(new Date(receipt.paid_at))}</dd>
        <dt>Method</dt>
        <dd>{methodLabel(receipt.method)}</dd>
        <dt>Amount recorded</dt>
        <dd>{formatTHB(receipt.amount_satang)}</dd>
        <dt>Confirmed by</dt>
        <dd>{receipt.confirmed_by}</dd>
        <dt>Verification</dt>
        <dd>{receipt.verification_note}</dd>
        {receipt.external_reference && (
          <>
            <dt>External reference</dt>
            <dd>{receipt.external_reference}</dd>
          </>
        )}
      </dl>
      <h2>Bill at payment</h2>
      <BillView lines={receipt.lines} policy={receipt.policy} totals={receipt} />
      <h2>Refund</h2>
      {receipt.refund ? (
        <p>
          Full refund of {formatTHB(receipt.refund.amount_satang)} recorded by {receipt.refund.recorded_by} on{" "}
          {dateTime.format(new Date(receipt.refund.created_at))}. Reason: {receipt.refund.reason}. Reference: {receipt.refund.external_reference}.
        </p>
      ) : isManager ? (
        <form onSubmit={(e) => void refund(e)}>
          <p>Record a refund only after the full amount has been returned outside TableFlow. Partial refunds are not supported.</p>
          <p>
            <label>
              Reason <input value={reason} maxLength={500} required onChange={(e) => setReason(e.target.value)} />
            </label>
          </p>
          <p>
            <label>
              Refund reference <input value={reference} maxLength={200} required onChange={(e) => setReference(e.target.value)} />
            </label>
          </p>
          <button type="submit">Record full refund of {formatTHB(receipt.amount_satang)}</button>
        </form>
      ) : (
        <p>No refund. Only a manager can record one.</p>
      )}
    </>
  );
}
