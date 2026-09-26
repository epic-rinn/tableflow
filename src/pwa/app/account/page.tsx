"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { type ApiError, api } from "@/lib/api/client";
import type { Member } from "@/lib/api/types";
import { MobileShell } from "@/components/common/MobileShell";
import { LoyaltySummary } from "@/components/LoyaltySummary";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

type State = { step: "loading" } | { step: "signed-out" } | { step: "member"; member: Member } | { step: "error"; error: ApiError };

export default function AccountPage() {
  const { t, errorText } = useI18n();
  const [state, setState] = useState<State>({ step: "loading" });
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let cancelled = false;
    void api<Member>("/members/me").then((res) => {
      if (cancelled) return;
      if (res.ok) setState({ step: "member", member: res.data });
      else if (res.status === 401) setState({ step: "signed-out" });
      else setState({ step: "error", error: res.error });
    });
    return () => {
      cancelled = true;
    };
  }, []);

  async function signOut() {
    const res = await api("/sessions/member", { method: "DELETE" });
    if (res.ok || res.status === 401) setState({ step: "signed-out" });
    else setNotice(errorText(res.error));
  }

  async function resend(email: string) {
    const res = await api("/members/verification", { method: "POST", body: { email } });
    setNotice(res.ok ? t("account.resent") : errorText(res.error));
  }

  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("account.title")}>
      {state.step === "loading" && <p role="status">{t("common.loading")}</p>}
      {state.step === "error" && <Notice notice={{ role: "alert", text: errorText(state.error) }} />}
      {state.step === "signed-out" && (
        <p>
          <Link href="/account/login">{t("account.signIn")}</Link>
          {t("account.or")}
          <Link href="/account/signup">{t("account.create")}</Link>
          {t("account.optional")}
        </p>
      )}
      {state.step === "member" && (
        <>
          <p>{t("account.signedInAs", { email: state.member.email })}</p>
          <LoyaltySummary />
          {state.member.email_verified ? (
            <p>{t("account.confirmed")}</p>
          ) : (
            <p>
              {t("account.unconfirmed")}{" "}
              <Button type="button" variant="outline" size="lg" className="h-12 rounded-2xl" onClick={() => resend(state.member.email)}>
                {t("account.resend")}
              </Button>
            </p>
          )}
          <Button type="button" variant="outline" size="lg" className="h-12 rounded-2xl" onClick={signOut}>
            {t("account.signOut")}
          </Button>
        </>
      )}
      <Notice notice={notice ? { role: "status", text: notice } : null} />
    </MobileShell>
  );
}
