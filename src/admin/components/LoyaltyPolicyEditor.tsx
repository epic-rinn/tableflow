"use client";

import { Info, Save } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { FieldError } from "@/components/common/AuthShell";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { StateBadge } from "@/components/common/StateBadge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api/client";
import type { LoyaltyPolicy } from "@/lib/api/types";
import { bahtToSatang, bpToPercent, percentToBP, satangToBaht } from "@/lib/money";

type Form = { perPoint: string; silverAt: string; silverPct: string; goldAt: string; goldPct: string };

const toForm = (p: LoyaltyPolicy): Form => ({
  perPoint: satangToBaht(p.satang_per_point),
  silverAt: satangToBaht(p.silver_threshold_satang),
  silverPct: bpToPercent(p.silver_discount_bp),
  goldAt: satangToBaht(p.gold_threshold_satang),
  goldPct: bpToPercent(p.gold_discount_bp),
});

// Manager editor for the versioned loyalty policy (LOY-003/004). Version 0 is
// the documented pilot default; the operator must confirm the economics.
export function LoyaltyPolicyEditor({ branchId }: { branchId: string }) {
  const [current, setCurrent] = useState<LoyaltyPolicy | null>(null);
  const [form, setForm] = useState<Form>({ perPoint: "", silverAt: "", silverPct: "", goldAt: "", goldPct: "" });
  const [fields, setFields] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<NoticeValue>(null);

  const show = useCallback((p: LoyaltyPolicy) => {
    setCurrent(p);
    setForm(toForm(p));
  }, []);
  const load = useCallback(async () => {
    const res = await api<LoyaltyPolicy>(`/branches/${branchId}/loyalty-policy`);
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
    const body = {
      expected_version: current.version,
      satang_per_point: bahtToSatang(form.perPoint),
      silver_threshold_satang: bahtToSatang(form.silverAt),
      silver_discount_bp: percentToBP(form.silverPct),
      gold_threshold_satang: bahtToSatang(form.goldAt),
      gold_discount_bp: percentToBP(form.goldPct),
    };
    const bad: Record<string, string> = {};
    if (body.satang_per_point === null) bad.satang_per_point = "Enter an amount in baht.";
    if (body.silver_threshold_satang === null) bad.silver_threshold_satang = "Enter an amount in baht.";
    if (body.gold_threshold_satang === null) bad.gold_threshold_satang = "Enter an amount in baht.";
    if (body.silver_discount_bp === null) bad.silver_discount_bp = "Enter a percentage.";
    if (body.gold_discount_bp === null) bad.gold_discount_bp = "Enter a percentage.";
    setFields(bad);
    if (Object.keys(bad).length) return;
    const res = await api<LoyaltyPolicy>(`/branches/${branchId}/loyalty-policy`, { method: "PUT", body });
    if (res.ok) {
      show(res.data);
      setNotice({ role: "status", text: `Saved as version ${res.data.version}. Settled bills keep the rates they were paid with.` });
    } else if (res.error.code === "VERSION_CONFLICT") {
      setNotice({ role: "alert", text: "Another manager changed the policy. The latest version is shown; review it and save again." });
      void load();
    } else {
      setFields(res.error.fields ?? {});
      setNotice({ role: "alert", text: res.error.message });
    }
  }

  const field = (key: keyof Form, apiKey: string, label: string, hint?: string) => (
    <div className="grid gap-2">
      <Label htmlFor={`loyalty-${key}`}>{label}</Label>
      <Input id={`loyalty-${key}`} inputMode="decimal" value={form[key]} aria-invalid={!!fields[apiKey]}
        aria-describedby={fields[apiKey] ? `${key}-error` : undefined} onChange={(e) => setForm({ ...form, [key]: e.target.value })} />
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      <FieldError id={`${key}-error`} message={fields[apiKey]} />
    </div>
  );

  return (
    <>
      <PageHeader title="Loyalty" description="Points and tier discounts for optional member accounts. Paid bills never change."
        actions={current && <StateBadge tone={current.configured ? "success" : "warning"} label={current.configured ? `Version ${current.version}` : "Pilot defaults"} />} />
      <div className="mb-4 flex gap-3 rounded-lg border border-info/30 bg-info-soft p-3 text-sm text-info">
        <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
        <p>
          Points count post-discount food spend, excluding service charge and tax. A tier earned on a bill applies from the next visit. Confirm these
          economics with the operator before live use.
        </p>
      </div>
      <Notice notice={notice} className="mb-4" />
      <Card className="max-w-2xl">
        <form onSubmit={(e) => void save(e)} noValidate className="grid gap-6">
          <CardHeader>
            <CardTitle>
              <h2>Loyalty policy</h2>
            </CardTitle>
            {current && (
              <CardDescription>{current.configured ? `Current version ${current.version}.` : "Using the documented pilot defaults (not yet confirmed)."}</CardDescription>
            )}
          </CardHeader>
          <CardContent className="grid gap-6">
            {field("perPoint", "satang_per_point", "Spend per point (฿)", "e.g. 100 = one point for every ฿100 of eligible spend")}
            <div className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2">
              <p className="font-medium sm:col-span-2">Silver</p>
              {field("silverAt", "silver_threshold_satang", "Silver from cumulative spend (฿)")}
              {field("silverPct", "silver_discount_bp", "Silver discount (%)")}
            </div>
            <div className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2">
              <p className="font-medium sm:col-span-2">Gold</p>
              {field("goldAt", "gold_threshold_satang", "Gold from cumulative spend (฿)")}
              {field("goldPct", "gold_discount_bp", "Gold discount (%)")}
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
