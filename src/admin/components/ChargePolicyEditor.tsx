"use client";

import { Info, Save } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { FieldError } from "@/components/common/AuthShell";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { StateBadge } from "@/components/common/StateBadge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
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
      <PageHeader title="Charges & tax" description="Versioned service charge and tax rates. Paid receipts never change."
        actions={current && <StateBadge tone={current.configured ? "success" : "warning"} label={current.configured ? `Version ${current.version}` : "Not configured"} />} />
      <div className="mb-4 flex gap-3 rounded-lg border border-info/30 bg-info-soft p-3 text-sm text-info">
        <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
        <p>
          Confirm every rate and the tax mode with the restaurant operator and their accountant before live use. TableFlow does not supply
          statutory rates.
        </p>
      </div>
      <Notice notice={notice} className="mb-4" />
      <Card className="max-w-xl">
        <form onSubmit={(e) => void save(e)} noValidate className="grid gap-6">
          <CardHeader>
            <CardTitle>
              <h2>Charge policy</h2>
            </CardTitle>
            {current && (
              <CardDescription>
                {current.configured ? `Current version ${current.version}.` : "Not configured yet: bills carry no service charge or tax."}
              </CardDescription>
            )}
          </CardHeader>
          <CardContent className="grid gap-6">
            <fieldset className="grid gap-2">
              <legend className="mb-2 text-sm font-medium">Menu prices are</legend>
              <RadioGroup value={mode} onValueChange={(v) => setMode(v as "exclusive" | "inclusive")} className="grid gap-2 sm:grid-cols-2">
                <Label className="flex cursor-pointer items-center gap-2 rounded-lg border p-3 font-normal has-data-[state=checked]:border-primary has-data-[state=checked]:bg-accent">
                  <RadioGroupItem value="exclusive" /> Before tax (tax added)
                </Label>
                <Label className="flex cursor-pointer items-center gap-2 rounded-lg border p-3 font-normal has-data-[state=checked]:border-primary has-data-[state=checked]:bg-accent">
                  <RadioGroupItem value="inclusive" /> Tax included
                </Label>
              </RadioGroup>
            </fieldset>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor="service-rate">Service charge (%)</Label>
                <Input id="service-rate" inputMode="decimal" value={service} aria-invalid={!!fields.service}
                  aria-describedby={fields.service ? "service-error" : undefined} onChange={(e) => setService(e.target.value)} />
                <FieldError id="service-error" message={fields.service} />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="tax-rate">Tax (%)</Label>
                <Input id="tax-rate" inputMode="decimal" value={tax} aria-invalid={!!fields.tax}
                  aria-describedby={fields.tax ? "tax-error" : undefined} onChange={(e) => setTax(e.target.value)} />
                <FieldError id="tax-error" message={fields.tax} />
              </div>
            </div>
          </CardContent>
          <CardFooter className="justify-end">
            <Button type="submit" disabled={!current}>
              <Save aria-hidden /> Save new version
            </Button>
          </CardFooter>
        </form>
      </Card>
    </>
  );
}
