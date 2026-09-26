import type { BillLine, Policy, Totals } from "@/lib/api/types";
import { bpToPercent, formatTHB } from "@/lib/money";

const METHOD_LABELS = { cash: "Cash", bank_transfer: "Bank transfer", card: "Card", other: "Other" } as const;
export const methodLabel = (m: keyof typeof METHOD_LABELS) => METHOD_LABELS[m];

// Itemised lines and aggregate charges exactly as Go calculated them; the
// admin never recomputes totals (BIL-001/003).
export function BillView({ lines, policy, totals }: { lines: BillLine[]; policy: Policy; totals: Totals }) {
  const inclusive = policy.tax_mode === "inclusive";
  return (
    <>
      <table>
        <caption className="sr-only">Bill items</caption>
        <thead>
          <tr>
            <th scope="col">Item</th>
            <th scope="col">Qty</th>
            <th scope="col">Unit</th>
            <th scope="col">Amount</th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l) => (
            <tr key={l.id}>
              <td>
                {l.name_en} <span lang="th">{l.name_th}</span>
                {l.options.length > 0 && <small> ({l.options.map((o) => o.name_en).join(", ")})</small>}
                {l.state !== "served" && <strong> — {l.state}</strong>}
              </td>
              <td>{l.quantity}</td>
              <td>{formatTHB(l.unit_price_satang)}</td>
              <td>{formatTHB(l.line_total_satang)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <dl className="totals">
        <dt>Subtotal</dt>
        <dd>{formatTHB(totals.gross_satang)}</dd>
        {totals.discount_satang > 0 && (
          <>
            <dt>Discount ({bpToPercent(totals.discount_bp)}%)</dt>
            <dd>−{formatTHB(totals.discount_satang)}</dd>
          </>
        )}
        <dt>Service charge ({bpToPercent(policy.service_bp)}%)</dt>
        <dd>{formatTHB(totals.service_satang)}</dd>
        <dt>{inclusive ? `Tax included (${bpToPercent(policy.tax_bp)}%)` : `Tax (${bpToPercent(policy.tax_bp)}%)`}</dt>
        <dd>{formatTHB(totals.tax_satang)}</dd>
        <dt>Total due</dt>
        <dd>
          <strong>{formatTHB(totals.total_satang)}</strong>
        </dd>
      </dl>
      <p>
        <small>
          Charge policy {policy.configured ? `version ${policy.version}` : "not configured (no service charge or tax)"}; amounts rounded half
          up per charge.
        </small>
      </p>
    </>
  );
}
