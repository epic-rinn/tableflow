"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { methodLabel } from "@/components/cashier/BillView";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { DateRange } from "@/components/reports/DateRange";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import type { DailyReport, PaymentMethod } from "@/lib/api/types";
import { businessDate } from "@/lib/dates";
import { formatTHB } from "@/lib/money";

const METHODS: PaymentMethod[] = ["cash", "bank_transfer", "card", "other"];

// Manager daily summary (OPS-002): figures come straight from the API, which
// reconciles them with settlements, refunds and the loyalty ledger.
export function DailyReports({ branchId }: { branchId: string }) {
  const [range, setRange] = useState({ from: businessDate(-6), to: businessDate() });
  const [report, setReport] = useState<DailyReport | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async (from: string, to: string) => {
    const res = await api<DailyReport>(`/branches/${branchId}/reports/daily?from=${from}&to=${to}`);
    if (res.ok) {
      setReport(res.data);
      setError("");
    } else {
      setError(res.error.message + (res.error.fields?.from ? ` ${res.error.fields.from}` : ""));
    }
  }, [branchId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(range.from, range.to);
  }, [load, range]);

  const t = report?.totals;
  const kpis: [string, string, string?][] = t
    ? [
        ["Net sales", formatTHB(t.net_satang), `${t.sales.count} paid bills`],
        ["Refunds", formatTHB(t.refunds.satang), `${t.refunds.count} refunds`],
        ["Visits", String(t.visits_opened), `${t.tickets_joined} queue tickets · ${t.no_shows} no-shows`],
        ["Points earned", String(t.points_earned), `${t.points_reversed} reversed · ${t.member_settlements} member bills`],
      ]
    : [];
  return (
    <>
      <PageHeader title="Reports" description={`Business dates in ${report?.timezone ?? "the branch timezone"}. Up to 31 days.`} />
      <div className="mb-6">
        <DateRange from={range.from} to={range.to} onApply={(from, to) => setRange({ from, to })} />
      </div>
      <Notice notice={error ? { role: "alert", text: error } : null} className="mb-4" />
      {report && t && (
        <div className="grid gap-6">
          <ul className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="Totals for the range">
            {kpis.map(([label, value, hint]) => (
              <li key={label}>
                <Card className="h-full">
                  <CardHeader>
                    <CardDescription>{label}</CardDescription>
                    <CardTitle className="text-2xl tabular-nums">{value}</CardTitle>
                    {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
                  </CardHeader>
                </Card>
              </li>
            ))}
          </ul>

          <Card role="region" aria-labelledby="methods-title">
            <CardHeader>
              <CardTitle>
                <h2 id="methods-title">By payment method</h2>
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead scope="col">Method</TableHead>
                    <TableHead scope="col" className="text-right">Sales</TableHead>
                    <TableHead scope="col" className="text-right">Refunds</TableHead>
                    <TableHead scope="col" className="text-right">Net</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {METHODS.map((m) => {
                    const s = t.sales_by_method[m] ?? { count: 0, satang: 0 };
                    const r = t.refunds_by_method[m] ?? { count: 0, satang: 0 };
                    return (
                      <TableRow key={m}>
                        <TableCell>{methodLabel(m)}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatTHB(s.satang)} <span className="text-muted-foreground">({s.count})</span></TableCell>
                        <TableCell className="text-right tabular-nums">{formatTHB(r.satang)} <span className="text-muted-foreground">({r.count})</span></TableCell>
                        <TableCell className="text-right font-medium tabular-nums">{formatTHB(s.satang - r.satang)}</TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card role="region" aria-labelledby="days-title">
            <CardHeader>
              <CardTitle>
                <h2 id="days-title">By day</h2>
              </CardTitle>
              <CardDescription>
                <Link href={`/receipts?from=${report.from}&to=${report.to}`} className="inline-flex items-center gap-1 text-primary underline-offset-4 hover:underline">
                  View the receipts in this range <ArrowRight className="size-3.5" aria-hidden />
                </Link>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead scope="col">Date</TableHead>
                    <TableHead scope="col" className="text-right">Tickets</TableHead>
                    <TableHead scope="col" className="text-right">No-shows</TableHead>
                    <TableHead scope="col" className="text-right">Visits</TableHead>
                    <TableHead scope="col" className="text-right">Sales</TableHead>
                    <TableHead scope="col" className="text-right">Refunds</TableHead>
                    <TableHead scope="col" className="text-right">Net</TableHead>
                    <TableHead scope="col" className="text-right">Points</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {report.days.map((d) => (
                    <TableRow key={d.date}>
                      <TableCell>
                        <Link href={`/receipts?from=${d.date}&to=${d.date}`} className="underline-offset-4 hover:underline">{d.date}</Link>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">{d.tickets_joined}</TableCell>
                      <TableCell className="text-right tabular-nums">{d.no_shows}</TableCell>
                      <TableCell className="text-right tabular-nums">{d.visits_opened}</TableCell>
                      <TableCell className="text-right tabular-nums">{formatTHB(d.sales.satang)}</TableCell>
                      <TableCell className="text-right tabular-nums">{formatTHB(d.refunds.satang)}</TableCell>
                      <TableCell className="text-right font-medium tabular-nums">{formatTHB(d.net_satang)}</TableCell>
                      <TableCell className="text-right tabular-nums">{d.points_earned - d.points_reversed}</TableCell>
                    </TableRow>
                  ))}
                  <TableRow className="font-semibold">
                    <TableCell>Total</TableCell>
                    <TableCell className="text-right tabular-nums">{t.tickets_joined}</TableCell>
                    <TableCell className="text-right tabular-nums">{t.no_shows}</TableCell>
                    <TableCell className="text-right tabular-nums">{t.visits_opened}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatTHB(t.sales.satang)}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatTHB(t.refunds.satang)}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatTHB(t.net_satang)}</TableCell>
                    <TableCell className="text-right tabular-nums">{t.points_earned - t.points_reversed}</TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      )}
    </>
  );
}
