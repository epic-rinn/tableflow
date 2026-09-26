import { StateBadge } from "@/components/common/StateBadge";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import type { BillLine, Policy, Totals } from "@/lib/api/types";
import { bpToPercent, formatTHB } from "@/lib/money";

const METHOD_LABELS = { cash: "Cash", bank_transfer: "Bank transfer", card: "Card", other: "Other" } as const;
export const methodLabel = (m: keyof typeof METHOD_LABELS) => METHOD_LABELS[m];

// Itemised lines and aggregate charges exactly as Go calculated them; the
// admin never recomputes totals (BIL-001/003).
export function BillView({ lines, policy, totals }: { lines: BillLine[]; policy: Policy; totals: Totals }) {
  const inclusive = policy.tax_mode === "inclusive";
  return (
    <div className="grid gap-4">
      <Table>
        <caption className="sr-only">Bill items</caption>
        <TableHeader>
          <TableRow>
            <TableHead scope="col">Item</TableHead>
            <TableHead scope="col" className="text-right">Qty</TableHead>
            <TableHead scope="col" className="text-right">Unit</TableHead>
            <TableHead scope="col" className="text-right">Amount</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {lines.map((l) => (
            <TableRow key={l.id}>
              <TableCell className="whitespace-normal">
                <span className="font-medium">{l.name_en}</span> <span lang="th" className="text-muted-foreground">{l.name_th}</span>
                {l.options.length > 0 && <span className="block text-xs text-muted-foreground">{l.options.map((o) => o.name_en).join(", ")}</span>}
                {l.state !== "served" && <StateBadge state={l.state} className="mt-1" />}
              </TableCell>
              <TableCell className="text-right tabular-nums">{l.quantity}</TableCell>
              <TableCell className="text-right tabular-nums">{formatTHB(l.unit_price_satang)}</TableCell>
              <TableCell className="text-right tabular-nums">{formatTHB(l.line_total_satang)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <dl className="ml-auto grid w-full max-w-xs grid-cols-[1fr_auto] gap-x-6 gap-y-1.5 text-sm">
        <dt className="text-muted-foreground">Subtotal</dt>
        <dd className="text-right tabular-nums">{formatTHB(totals.gross_satang)}</dd>
        {totals.discount_satang > 0 && (
          <>
            <dt className="text-muted-foreground">Discount ({bpToPercent(totals.discount_bp)}%)</dt>
            <dd className="text-right tabular-nums">−{formatTHB(totals.discount_satang)}</dd>
          </>
        )}
        <dt className="text-muted-foreground">Service charge ({bpToPercent(policy.service_bp)}%)</dt>
        <dd className="text-right tabular-nums">{formatTHB(totals.service_satang)}</dd>
        <dt className="text-muted-foreground">{inclusive ? `Tax included (${bpToPercent(policy.tax_bp)}%)` : `Tax (${bpToPercent(policy.tax_bp)}%)`}</dt>
        <dd className="text-right tabular-nums">{formatTHB(totals.tax_satang)}</dd>
        <Separator className="col-span-2 my-1" />
        <dt className="text-base font-semibold">Total due</dt>
        <dd className="text-right text-base font-semibold tabular-nums">{formatTHB(totals.total_satang)}</dd>
      </dl>
      <p className="text-xs text-muted-foreground">
        Charge policy {policy.configured ? `version ${policy.version}` : "not configured (no service charge or tax)"}; amounts rounded half up per charge.
      </p>
    </div>
  );
}
