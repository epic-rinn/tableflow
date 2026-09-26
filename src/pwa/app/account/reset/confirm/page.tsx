"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function ResetConfirmPage() {
  const { t, errorText } = useI18n();
  const token = useFragmentToken();
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const password = String(form.get("new_password") ?? "");
    if (password !== String(form.get("confirm") ?? "")) {
      setFields({ confirm: t("resetConfirm.mismatch") });
      return;
    }
    setFields({});
    setError("");
    const res = await api("/members/password-reset/confirm", { method: "POST", body: { token, new_password: password } });
    if (res.ok) {
      setDone(true);
      return;
    }
    setFields(res.error.fields ?? {});
    setError(errorText(res.error));
  }

  if (done) {
    return (
      <MobileShell eyebrow={t("account.eyebrow")} title={t("resetConfirm.doneTitle")}>
        <p role="status">
          {t("resetConfirm.doneBody")} <Link href="/account/login">{t("resetConfirm.signIn")}</Link>
        </p>
      </MobileShell>
    );
  }
  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("resetConfirm.title")}>
      {token === "" ? (
        <p role="alert">{t("verify.missing")}</p>
      ) : (
        <form onSubmit={onSubmit} noValidate className="grid gap-4">
          <Field id="new_password" label={t("resetConfirm.new")} type="password" autoComplete="new-password" required error={fields.new_password} />
          <Field id="confirm" label={t("resetConfirm.confirm")} type="password" autoComplete="new-password" required error={fields.confirm} />
          <Notice notice={error ? { role: "alert", text: error } : null} />
          <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={!token}>
            {t("resetConfirm.submit")}
          </Button>
        </form>
      )}
    </MobileShell>
  );
}
