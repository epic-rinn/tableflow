"use client";

import { Award, QrCode, ReceiptText, Wallet } from "lucide-react";
import Link from "next/link";
import { useCallback, useState } from "react";
import { BillView } from "@/components/cashier/BillView";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { ReasonDialog } from "@/components/common/ReasonDialog";
import { StateBadge } from "@/components/common/StateBadge";
import { Freshness } from "@/components/Freshness";
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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Textarea } from "@/components/ui/textarea";
import { api, type ApiResult } from "@/lib/api/client";
import { type Bill, type BillLine, type PaymentMethod, type Settlement, type SettlementRef, type Table, TIER_LABELS } from "@/lib/api/types";
import { bahtToSatang, formatTHB } from "@/lib/money";
import { useIdempotent } from "@/lib/useIdempotent";
import { usePolling } from "@/lib/usePolling";

const POLL_MS = 3000;
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
  const [notice, setNotice] = useState<NoticeValue>(null);
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
        case "IDEMPOTENCY_CONFLICT":
          setNotice({ role: "alert", text: "An earlier attempt with different details may already have been recorded. Check the bill before collecting again." });
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

  async function detach(target: Bill, reason: string) {
    setBusy(true);
    const res = await run(`detach:${target.visit_id}:${target.bill_version}`, (key) =>
      api<Bill>(`/visits/${target.visit_id}/member-detach`, { method: "POST", key, body: { expected_version: target.bill_version, reason } }));
    setBusy(false);
    setNotice(res.ok ? { role: "status", text: "Member detached from this visit." } : { role: "alert", text: res.error.message });
    bill.refresh();
  }

  // ADM-006: payment actions are disabled while the bill cannot be refreshed.
  const offline = bill.error?.code === "NETWORK" || floor.error?.code === "NETWORK";
  const blocked = busy || offline;
  const b = bill.data;
  return (
    <>
      <PageHeader title="Cashier" description="Settle bills after verifying payment with the restaurant's own records." />
      {offline && (
        <Notice className="mb-4" notice={{ role: "alert", text: "Offline — payment actions are disabled until the connection returns. Do not collect money for a bill you cannot see." }} />
      )}
      <Notice notice={notice} className="mb-4" />
      <div className="grid gap-6 lg:grid-cols-[300px_1fr]">
        <Card role="region" aria-labelledby="find-title" className="self-start">
          <CardHeader>
            <CardTitle>
              <h2 id="find-title">Find a bill</h2>
            </CardTitle>
            <CardDescription>Pick a seated table or paste the guest&apos;s dining QR link.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <form onSubmit={(e) => void resolve(e)} className="grid gap-2">
              <Label htmlFor="dining-token">Dining QR link or code</Label>
              <div className="flex gap-2">
                <Input id="dining-token" value={token} onChange={(e) => setToken(e.target.value)} autoComplete="off" required />
                <Button type="submit" variant="outline">
                  <QrCode aria-hidden /> Open bill
                </Button>
              </div>
            </form>
            {occupied.length === 0 ? (
              <p className="text-sm text-muted-foreground">No seated tables.</p>
            ) : (
              <ul className="flex flex-wrap gap-2">
                {occupied.map((t) => {
                  const active = t.claim?.visit_id === visitId;
                  return (
                    <li key={t.id}>
                      <Button type="button" variant={active ? "default" : "outline"} className="h-11 px-4" aria-pressed={active}
                        onClick={() => open(t.claim!.visit_id!)}>
                        Table {t.label}
                      </Button>
                    </li>
                  );
                })}
              </ul>
            )}
          </CardContent>
        </Card>

        {visitId ? (
          <Card role="region" aria-labelledby="bill-title">
            <CardHeader>
              <CardTitle className="flex flex-wrap items-center gap-2">
                <h2 id="bill-title">{b ? `Table ${b.table_label} — ${stateLabel(b.visit_state)}` : "Bill"}</h2>
                {b && <StateBadge state={b.visit_state} />}
              </CardTitle>
              <CardDescription>
                <Freshness updatedAt={bill.updatedAt} error={bill.error} intervalMs={POLL_MS} />
              </CardDescription>
            </CardHeader>
            {b && (
              <CardContent className="grid gap-6">
                {b.member_claim?.claimed && (
                  <div className="flex flex-wrap items-center gap-3 rounded-lg border border-primary/30 bg-accent/60 p-3 text-sm">
                    <Award className="size-4 text-primary" aria-hidden />
                    <span className="flex-1">
                      Member {b.member_claim.masked_email ?? ""} · {b.member_claim.tier ? TIER_LABELS[b.member_claim.tier] : ""} tier
                      {b.frozen ? " (benefit fixed for this bill)" : " (discount previewed; fixed when settlement begins)"}
                    </span>
                    {b.visit_state === "open" && (
                      <ReasonDialog trigger="Detach member" title="Detach the member from this visit?"
                        description="Use when the wrong member claimed. The member at the table can then claim again from their phone."
                        reasonLabel="Reason for detaching" confirmLabel="Detach member" destructive disabled={blocked}
                        onConfirm={(reason) => void detach(b, reason)} />
                    )}
                  </div>
                )}
                <BillView lines={b.lines} policy={b.policy} totals={b} />
                {unresolved.length > 0 && (
                  <ul aria-label="Unresolved items" className="grid gap-1 rounded-lg border border-warning/40 bg-warning-soft p-3 text-sm">
                    {unresolved.map((l) => (
                      <li key={l.id}>
                        {l.quantity}× {l.name_en} — {l.state}
                      </li>
                    ))}
                  </ul>
                )}
                {b.visit_state === "open" && (
                  <div className="flex flex-wrap items-center justify-end gap-3 border-t pt-4">
                    {b.unresolved_lines > 0 && <p className="text-sm text-warning">{b.unresolved_lines} item(s) still with the kitchen.</p>}
                    <Button type="button" size="lg" disabled={blocked || b.lines.length === 0}
                      onClick={() => void command<Bill>(`begin:${visitId}:${b.bill_version}`, "begin", { expected_version: b.bill_version },
                        (s) => `Ordering is frozen. Collect ${formatTHB(s.total_satang)}.`)}>
                      <Wallet aria-hidden /> Begin settlement
                    </Button>
                  </div>
                )}
                {b.visit_state === "settling" && (
                  <SettlingActions bill={b} busy={blocked}
                    onConfirm={(body) => void command<Settlement>(`confirm:${visitId}:${b.bill_version}`, "confirm", body,
                      (s) => `Payment recorded. Receipt ${s.receipt_reference}.${s.points_earned !== null ? ` Member earned ${s.points_earned} point(s).` : ""} The table stays occupied until the party departs.`)}
                    onReopen={(reason) => void command<Bill>(`reopen:${visitId}:${b.bill_version}`, "reopen", { expected_version: b.bill_version, reason },
                      () => "Bill reopened; guests can order again.")} />
                )}
                {b.settlement && (
                  <p className="flex items-center gap-2 rounded-lg bg-success-soft p-3 text-sm text-success">
                    <ReceiptText className="size-4" aria-hidden /> Paid — receipt{" "}
                    <Link href={`/receipts/${b.settlement.id}`} className="font-semibold underline underline-offset-4">
                      {b.settlement.receipt_reference}
                    </Link>
                  </p>
                )}
              </CardContent>
            )}
          </Card>
        ) : (
          <div className="grid place-items-center rounded-xl border border-dashed p-10 text-center text-sm text-muted-foreground">
            Select a table to review its bill.
          </div>
        )}
      </div>
    </>
  );
}

function stateLabel(s: Bill["visit_state"]) {
  return { open: "open", settling: "settling (ordering frozen)", paid: "paid", departed: "departed", closed: "closed" }[s];
}

type ConfirmBody = { expected_version: number; amount_satang: number; method: PaymentMethod; verification_note: string; external_reference: string | null };

const METHODS: { value: PaymentMethod; label: string }[] = [
  { value: "cash", label: "Cash" },
  { value: "bank_transfer", label: "Bank transfer" },
  { value: "card", label: "Card" },
  { value: "other", label: "Other" },
];

function SettlingActions({ bill, busy, onConfirm, onReopen }: {
  bill: Bill; busy: boolean; onConfirm: (b: ConfirmBody) => void; onReopen: (reason: string) => void;
}) {
  const [method, setMethod] = useState<PaymentMethod>("cash");
  const [tendered, setTendered] = useState("");
  const [note, setNote] = useState("");
  const [reference, setReference] = useState("");
  const [review, setReview] = useState(false);
  const tender = bahtToSatang(tendered);
  const change = tender !== null && tender >= bill.total_satang ? tender - bill.total_satang : null;
  const total = formatTHB(bill.total_satang);
  const body = (): ConfirmBody => ({ expected_version: bill.bill_version, amount_satang: bill.total_satang, method,
    verification_note: note.trim(), external_reference: reference.trim() || null });

  return (
    <div className="grid gap-6 border-t pt-4">
      <form
        aria-labelledby="confirm-title"
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          setReview(true);
        }}
      >
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h3 id="confirm-title" className="text-base font-semibold">Confirm payment</h3>
          <p className="text-sm">
            Amount to record: <strong className="text-lg">{total}</strong> <span className="text-muted-foreground">(always the exact bill total)</span>
          </p>
        </div>
        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-medium">Method</legend>
          <RadioGroup value={method} onValueChange={(v) => setMethod(v as PaymentMethod)} className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            {METHODS.map((m) => (
              <Label key={m.value} className="flex cursor-pointer items-center gap-2 rounded-lg border p-3 font-normal has-data-[state=checked]:border-primary has-data-[state=checked]:bg-accent">
                <RadioGroupItem value={m.value} /> {m.label}
              </Label>
            ))}
          </RadioGroup>
        </fieldset>
        {method === "cash" ? (
          <div className="grid gap-2 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="tendered">Cash received (฿, optional)</Label>
              <Input id="tendered" inputMode="decimal" value={tendered} onChange={(e) => setTendered(e.target.value)} />
            </div>
            {change !== null && (
              <output className="self-end rounded-lg bg-secondary px-3 py-2 text-sm">
                Change: <strong>{formatTHB(change)}</strong>
              </output>
            )}
          </div>
        ) : (
          <div className="grid gap-2">
            <Label htmlFor="external-reference">External reference (optional)</Label>
            <Input id="external-reference" value={reference} maxLength={200} onChange={(e) => setReference(e.target.value)} />
          </div>
        )}
        <div className="grid gap-2">
          <Label htmlFor="verification-note">Verification note</Label>
          <Textarea id="verification-note" value={note} maxLength={500} required onChange={(e) => setNote(e.target.value)}
            placeholder="e.g. counted at till / seen in bank app" />
          <p className="text-xs text-muted-foreground">
            Verify transfers in the restaurant&apos;s own bank app. A customer&apos;s slip or screenshot is not proof of payment.
          </p>
        </div>
        <Button type="submit" size="lg" disabled={busy || !note.trim()} className="justify-self-end">
          Confirm {total} received
        </Button>
      </form>

      <AlertDialog open={review} onOpenChange={setReview}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Record payment of {total}?</AlertDialogTitle>
            <AlertDialogDescription>
              {METHODS.find((m) => m.value === method)?.label} · table {bill.table_label}. This closes the bill and cannot be undone; a refund is a
              separate manager action.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Back</AlertDialogCancel>
            <AlertDialogAction onClick={() => onConfirm(body())}>Record payment</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <div className="flex items-center justify-between gap-2 rounded-lg bg-muted/60 p-3 text-sm">
        <span id="reopen-title">Guests want to order more? Reopen the bill.</span>
        <ReasonDialog trigger="Reopen" title="Reopen this bill?" description="Ordering resumes and the bill will be recalculated."
          reasonLabel="Reason" confirmLabel="Reopen bill" disabled={busy} onConfirm={onReopen} />
      </div>
    </div>
  );
}
