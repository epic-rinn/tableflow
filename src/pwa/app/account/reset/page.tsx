"use client";

import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function ResetRequestPage() {
  const { t, errorText } = useI18n();
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const res = await api("/members/password-reset/request", { method: "POST", body: { email: form.get("email") } });
    if (res.ok) setSent(true);
    else setError(errorText(res.error));
  }

  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("reset.title")}>
      {sent ? (
        <p role="status">{t("reset.sent")}</p>
      ) : (
        <form onSubmit={onSubmit} noValidate className="grid gap-4">
          <Field id="email" label={t("login.email")} type="email" autoComplete="email" required />
          <Notice notice={error ? { role: "alert", text: error } : null} />
          <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base">{t("reset.submit")}</Button>
        </form>
      )}
    </MobileShell>
  );
}
