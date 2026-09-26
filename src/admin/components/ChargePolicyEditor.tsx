"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { ChargePolicy } from "@/lib/api/types";
import { bpToPercent, percentToBP } from "@/lib/money";

// Manager editor for the versioned charge/tax policy (BIL-004). Nothing
// here is a statutory default; the operator must confirm the rates.
export function ChargePolicyEditor({ branchId }: { branchId: string }) {
  const [current, setCurrent] = useState<ChargePolicy | null>(null);
  const [mode, setMode] = useState<"exclusive" | "inclusive">("exclusive");
  const [tax, setTax] = useState("0");
  const [service, setService] = useState("0");
  const [notice, setNotice] = useState<{ role: "alert" | "status"; text: string } | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  const show = useCallback((p: ChargePolicy) => {
    setCurrent(p);
    setMode(p.tax_mode);
    setTax(bpToPercent(p.tax_bp));
    setService(bpToPercent(p.service_bp));
  }, []);

  const load = useCallback(async () => {
    const res = await api<ChargePolicy>(`/branches/${branchId}/charge-policy`);
    if (res.ok) show(res.data);
    else setNotice({ role: "alert", text: res.error.message });
  }, [branchId, show]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (!current) return;
    const taxBP = percentToBP(tax);
    const serviceBP = percentToBP(service);
    const bad: Record<string, string> = {};
    if (taxBP === null) bad.tax = "Enter a percentage from 0 to 100 with up to two decimals.";
    if (serviceBP === null) bad.service = "Enter a percentage from 0 to 100 with up to two decimals.";
    setFields(bad);
    if (taxBP === null || serviceBP === null) return;
    const res = await api<ChargePolicy>(`/branches/${branchId}/charge-policy`, {
      method: "PUT",
      body: { expected_version: current.version, tax_mode: mode, tax_bp: taxBP, service_bp: serviceBP },
    });
    if (res.ok) {
      show(res.data);
      setNotice({ role: "status", text: `Saved as version ${res.data.version}. Open bills use it now; paid receipts are unchanged.` });
    } else if (res.error.code === "VERSION_CONFLICT") {
      setNotice({ role: "alert", text: "Another manager changed the policy. The latest version is shown; review it and save again." });
      void load();
    } else {
      setNotice({ role: "alert", text: res.error.message });
    }
  }

  return (
    <>
      <h1>Charges &amp; tax</h1>
      <p>
        Confirm every rate and the tax mode with the restaurant operator and their accountant before live use. TableFlow does not supply
        statutory rates.
      </p>
      {notice && <p role={notice.role}>{notice.text}</p>}
      {current && (
        <p>{current.configured ? `Current version ${current.version}.` : "Not configured yet: bills carry no service charge or tax."}</p>
      )}
      <form onSubmit={(e) => void save(e)} noValidate>
        <fieldset>
          <legend>Menu prices are</legend>
          <label>
            <input type="radio" name="mode" checked={mode === "exclusive"} onChange={() => setMode("exclusive")} /> Before tax (tax added)
          </label>
          <label>
            <input type="radio" name="mode" checked={mode === "inclusive"} onChange={() => setMode("inclusive")} /> Tax included
          </label>
        </fieldset>
        <p>
          <label>
            Service charge (%){" "}
            <input inputMode="decimal" value={service} aria-invalid={!!fields.service} aria-describedby="service-error"
              onChange={(e) => setService(e.target.value)} />
          </label>{" "}
          {fields.service && <span id="service-error">{fields.service}</span>}
        </p>
        <p>
          <label>
            Tax (%){" "}
            <input inputMode="decimal" value={tax} aria-invalid={!!fields.tax} aria-describedby="tax-error" onChange={(e) => setTax(e.target.value)} />
          </label>{" "}
          {fields.tax && <span id="tax-error">{fields.tax}</span>}
        </p>
        <button type="submit" disabled={!current}>
          Save new version
        </button>
      </form>
    </>
  );
}
