"use client";

import { useCallback, useState } from "react";
import { BillView } from "@/components/cashier/BillView";
import { Freshness } from "@/components/Freshness";
import { api, type ApiResult } from "@/lib/api/client";
import type { Bill, BillLine, PaymentMethod, Settlement, SettlementRef, Table } from "@/lib/api/types";
import { bahtToSatang, formatTHB } from "@/lib/money";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 3000;
type Notice = { role: "alert" | "status"; text: string };
type Conflict = { bill?: Bill; lines?: BillLine[]; settlement?: SettlementRef };

// A pasted dining link carries the token in its fragment; accept either.
function tokenFrom(input: string): string {
  const t = input.trim();
  const hash = t.indexOf("#");
  return hash >= 0 ? t.slice(hash + 1) : t;
}

const uncertain = (r: { status: number }) => r.status === 0 || r.status >= 500;

// Cashier workspace: find a bill by table or dining QR, begin settlement,
// confirm externally verified payment of the exact total, or reopen.
// Guests can never reach these actions; Go re-checks every step.
export function CashierWorkspace({ branchId }: { branchId: string }) {
  const run = useIdempotent();
  const [visitId, setVisitId] = useState<string | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [unresolved, setUnresolved] = useState<BillLine[]>([]);
  const [busy, setBusy] = useState(false);
  const floor = usePolling(
    useCallback((s: AbortSignal) => api<{ items: Table[] }>(`/branches/${branchId}/tables`, { signal: s }), [branchId]),
    POLL_MS * 2,
  );
  const bill = usePolling(
    useCallback(
      (s: AbortSignal): Promise<ApiResult<Bill | null>> =>
        visitId ? api<Bill>(`/visits/${visitId}/bill`, { signal: s }) : Promise.resolve({ ok: true, status: 200, data: null }),
      [visitId],
    ),
    POLL_MS,
  );
  const [token, setToken] = useState("");
  const occupied = (floor.data?.items ?? []).filter((t) => t.claim?.kind === "visit" && t.claim.visit_id);

  function open(id: string) {
    setVisitId(id);
    setNotice(null);
    setUnresolved([]);
  }

  async function resolve(e: React.FormEvent) {
    e.preventDefault();
    const res = await api<Bill>("/bills/resolve", { method: "POST", body: { dining_token: tokenFrom(token) } });
    if (res.ok) {
      setToken("");
      open(res.data.visit_id);
    } else {
      setNotice({ role: "alert", text: res.status === 404 ? "No open bill matches that QR code." : res.error.message });
    }
  }

  // Shared handling for settlement commands.
  async function command<T>(action: string, path: string, body: object, ok: (data: T) => string) {
    if (!visitId) return;
    setBusy(true);
    const res = await run(action, (key) => api<T>(`/visits/${visitId}/settlement/${path}`, { method: "POST", key, body }));
    setBusy(false);
    setUnresolved([]);
    if (res.ok) {
      setNotice({ role: "status", text: ok(res.data) });
    } else {
      const c = (res.body ?? {}) as Conflict;
      switch (res.error.code) {
        case "BILL_VERSION_CONFLICT":
          setNotice({ role: "alert", text: "The bill changed since you opened it. Review the refreshed bill and try again." });
          break;
        case "UNRESOLVED_LINES":
          setUnresolved(c.lines ?? []);
          setNotice({ role: "alert", text: "Some items are not served, rejected or cancelled yet. Resolve them with the kitchen first." });
          break;
        case "ALREADY_PAID":
          setNotice({ role: "alert", text: `This bill is already paid (receipt ${c.settlement?.receipt_reference ?? res.error.fields.receipt_reference}). Do not collect again.` });
          break;
        default:
          setNotice({
            role: "alert",
            text: uncertain(res)
              ? `${res.error.message} The outcome is unknown: check the bill before collecting again, then retry — a retry never records payment twice.`
              : res.error.message,
          });
      }
    }
    bill.refresh();
  }

  const b = bill.data;
  return (
    <>
      <h1>Cashier</h1>
      {notice && <p role={notice.role}>{notice.text}</p>}
      <section aria-labelledby="find-title">
        <h2 id="find-title">Find a bill</h2>
        <form onSubmit={(e) => void resolve(e)}>
          <label>
            Dining QR link or code <input value={token} onChange={(e) => setToken(e.target.value)} autoComplete="off" required />
          </label>{" "}
          <button type="submit">Open bill</button>
        </form>
        {occupied.length === 0 ? (
          <p>No seated tables.</p>
        ) : (
          <ul className="inline-list">
            {occupied.map((t) => (
              <li key={t.id}>
                <button type="button" aria-pressed={t.claim?.visit_id === visitId} onClick={() => open(t.claim!.visit_id!)}>
                  Table {t.label}
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      {visitId && (
        <section aria-labelledby="bill-title">
          <h2 id="bill-title">
            {b ? `Table ${b.table_label} — ${stateLabel(b.visit_state)}` : "Bill"}
          </h2>
          <Freshness updatedAt={bill.updatedAt} error={bill.error} intervalMs={POLL_MS} />
          {b && (
            <>
              <BillView lines={b.lines} policy={b.policy} totals={b} />
              {unresolved.length > 0 && (
                <ul aria-label="Unresolved items">
                  {unresolved.map((l) => (
                    <li key={l.id}>
                      {l.quantity}× {l.name_en} — {l.state}
                    </li>
                  ))}
                </ul>
              )}
              {b.visit_state === "open" && (
                <>
                  {b.unresolved_lines > 0 && <p>{b.unresolved_lines} item(s) still with the kitchen.</p>}
                  <button type="button" disabled={busy || b.lines.length === 0}
                    onClick={() => void command<Bill>(`begin:${visitId}:${b.bill_version}`, "begin", { expected_version: b.bill_version },
                      (s) => `Ordering is frozen. Collect ${formatTHB(s.total_satang)}.`)}>
                    Begin settlement
                  </button>
                </>
              )}
              {b.visit_state === "settling" && (
                <SettlingActions bill={b} busy={busy}
                  onConfirm={(body) => void command<Settlement>(`confirm:${visitId}:${b.bill_version}`, "confirm", body,
                    (s) => `Payment recorded. Receipt ${s.receipt_reference}. The table stays occupied until the party departs.`)}
                  onReopen={(reason) => void command<Bill>(`reopen:${visitId}:${b.bill_version}`, "reopen", { expected_version: b.bill_version, reason },
                    () => "Bill reopened; guests can order again.")} />
              )}
              {b.settlement && (
                <p>
                  Paid — receipt <a href={`/receipts/${b.settlement.id}`}>{b.settlement.receipt_reference}</a>
                </p>
              )}
            </>
          )}
        </section>
      )}
    </>
  );
}

function stateLabel(s: Bill["visit_state"]) {
  return { open: "open", settling: "settling (ordering frozen)", paid: "paid", departed: "departed", closed: "closed" }[s];
}

type ConfirmBody = { expected_version: number; amount_satang: number; method: PaymentMethod; verification_note: string; external_reference: string | null };

function SettlingActions({ bill, busy, onConfirm, onReopen }: {
  bill: Bill; busy: boolean; onConfirm: (b: ConfirmBody) => void; onReopen: (reason: string) => void;
}) {
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [tendered, setTendered] = useState("");
  const [note, setNote] = useState("");
  const [reference, setReference] = useState("");
  const [reason, setReason] = useState("");
  const tender = bahtToSatang(tendered);
  const change = tender !== null && tender >= bill.total_satang ? tender - bill.total_satang : null;

  return (
    <>
      <form
        aria-labelledby="confirm-title"
        onSubmit={(e) => {
          e.preventDefault();
          onConfirm({ expected_version: bill.bill_version, amount_satang: bill.total_satang, method,
            verification_note: note.trim(), external_reference: reference.trim() || null });
        }}
      >
        <h3 id="confirm-title">Confirm payment</h3>
        <p>
          Amount to record: <strong>{formatTHB(bill.total_satang)}</strong> (always the exact bill total)
        </p>
        <fieldset>
          <legend>Method</legend>
          {(["cash", "bank_transfer", "card", "other"] as const).map((m) => (
            <label key={m}>
              <input type="radio" name="method" value={m} checked={method === m} onChange={() => setMethod(m)} />{" "}
              {{ cash: "Cash", bank_transfer: "Bank transfer", card: "Card", other: "Other" }[m]}
            </label>
          ))}
        </fieldset>
        {method === "cash" && (
          <p>
            <label>
              Cash received (฿, optional) <input inputMode="decimal" value={tendered} onChange={(e) => setTendered(e.target.value)} />
            </label>{" "}
            {change !== null && <output>Change: {formatTHB(change)}</output>}
          </p>
        )}
        {method !== "cash" && (
          <p>
            <label>
              External reference (optional) <input value={reference} maxLength={200} onChange={(e) => setReference(e.target.value)} />
            </label>
          </p>
        )}
        <p>
          <label>
            Verification note{" "}
            <input value={note} maxLength={500} required onChange={(e) => setNote(e.target.value)}
              placeholder="e.g. counted at till / seen in bank app" />
          </label>
        </p>
        <p>
          <small>Verify transfers in the restaurant&apos;s own bank app. A customer&apos;s slip or screenshot is not proof of payment.</small>
        </p>
        <button type="submit" disabled={busy}>
          Confirm {formatTHB(bill.total_satang)} received
        </button>
      </form>
      <form
        aria-labelledby="reopen-title"
        onSubmit={(e) => {
          e.preventDefault();
          onReopen(reason.trim());
        }}
      >
        <h3 id="reopen-title">Reopen bill</h3>
        <label>
          Reason <input value={reason} maxLength={500} required onChange={(e) => setReason(e.target.value)} />
        </label>{" "}
        <button type="submit" disabled={busy}>Reopen</button>
      </form>
    </>
  );
}
