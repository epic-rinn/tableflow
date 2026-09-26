"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { type ApiError, api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";
import { MobileShell } from "@/components/common/MobileShell";
import { useI18n } from "@/lib/i18n";

export default function VerifyPage() {
  const { t, errorText } = useI18n();
  const token = useFragmentToken();
  const [result, setResult] = useState<{ ok: boolean; error?: ApiError } | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    void api("/members/verify", { method: "POST", body: { token } }).then((res) => {
      if (!cancelled) setResult(res.ok ? { ok: true } : { ok: false, error: res.error });
    });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("verify.title")}>
      {token === "" && <p role="alert">{t("verify.missing")}</p>}
      {token && !result && <p role="status">{t("verify.confirming")}</p>}
      {result?.ok && (
        <p role="status">
          {t("verify.done")} <Link href="/account">{t("verify.goAccount")}</Link>
        </p>
      )}
      {result && !result.ok && result.error && <p role="alert">{errorText(result.error)}</p>}
    </MobileShell>
  );
}
