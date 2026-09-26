"use client";

import { Award, Sparkles } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Pill } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api/client";
import type { ClaimStatus } from "@/lib/api/types";
import { useIdempotent } from "@/lib/useIdempotent";
import { useI18n } from "@/lib/i18n";

const percent = (bp: number) => (bp / 100).toFixed(2).replace(/\.?0+$/, "");

// Optional loyalty at the table (LOY-001): a signed-in member adds this
// visit to their points. Other diners only ever learn that it is claimed.
export function MemberPoints({ visitId, open }: { visitId: string; open: boolean }) {
  const run = useIdempotent();
  const { t, errorText } = useI18n();
  const [status, setStatus] = useState<ClaimStatus | "signed-out" | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const res = await api<ClaimStatus>(`/visits/${visitId}/member-claim`);
    if (res.ok) setStatus(res.data);
    else if (res.status === 401) setStatus("signed-out");
    else setError(errorText(res.error));
  }, [visitId, errorText]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function claim(s: ClaimStatus) {
    setBusy(true);
    const res = await run(`claim:${visitId}:${s.bill_version}`, (key) =>
      api<ClaimStatus>(`/visits/${visitId}/member-claim`, { method: "POST", key, body: { expected_version: s.bill_version } }));
    setBusy(false);
    if (res.ok) {
      setStatus(res.data);
      setError("");
      return;
    }
    setError(res.error.code === "VERSION_CONFLICT" ? t("points.changed") : errorText(res.error));
    void load();
  }

  if (status === null && !error) return null;
  return (
    <section aria-labelledby="points-title" className="grid gap-3 rounded-2xl border border-primary/30 bg-accent/60 p-4">
      <h3 id="points-title" className="flex items-center gap-2 text-lg font-bold">
        <Award className="size-5 text-primary" aria-hidden /> {t("points.title")}
      </h3>
      <Notice notice={error ? { role: "alert", text: error } : null} />
      {status === "signed-out" && (
        <>
          <p className="text-sm">{t("points.pitch")}</p>
          <div className="flex flex-wrap gap-2">
            <Button asChild size="lg" className="h-11 rounded-xl">
              <Link href="/account/login?next=/t">{t("points.signIn")}</Link>
            </Button>
            <Button asChild size="lg" variant="outline" className="h-11 rounded-xl">
              <Link href="/account/signup">{t("points.create")}</Link>
            </Button>
          </div>
        </>
      )}
      {status && status !== "signed-out" && status.mine && (
        <p role="status" className="text-sm">
          <strong className="block text-base">{t("points.mine")}</strong>
          <Pill tone="primary">{t("points.tier", { tier: t(`tier.${status.tier ?? "base"}`) })}</Pill>{" "}
          {status.discount_bp ? t("points.discount", { p: percent(status.discount_bp) }) : t("points.noDiscount")}
        </p>
      )}
      {status && status !== "signed-out" && !status.mine && status.claimed && (
        <p className="text-sm">{t("points.other")}</p>
      )}
      {status && status !== "signed-out" && !status.claimed &&
        (open ? (
          <Button type="button" size="lg" className="h-12 rounded-xl" disabled={busy} onClick={() => void claim(status)}>
            <Sparkles aria-hidden /> {t("points.claim")}
          </Button>
        ) : (
          <p className="text-sm">{t("points.tooLate")}</p>
        ))}
    </section>
  );
}
