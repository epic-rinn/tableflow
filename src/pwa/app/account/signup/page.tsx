"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { useI18n } from "@/lib/i18n";

export default function SignupPage() {
  const { t, errorText } = useI18n();
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    setFields({});
    const res = await api("/members", {
      method: "POST",
      body: { email: form.get("email"), password: form.get("password"), locale: form.get("locale") },
    });
    setBusy(false);
    if (res.ok) {
      setSent(true);
      return;
    }
    setFields(res.error.fields ?? {});
    setError(errorText(res.error));
  }

  if (sent) {
    return (
      <MobileShell eyebrow={t("account.eyebrow")} title={t("signup.checkTitle")}>
        <p role="status">{t("signup.checkBody")}</p>
        <Link href="/account/login">{t("login.submit")}</Link>
      </MobileShell>
    );
  }
  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("signup.title")}>
      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <Field id="email" label={t("login.email")} type="email" autoComplete="email" required error={fields.email} />
        <Field id="password" label={t("signup.password")} type="password" autoComplete="new-password" required error={fields.password} />
        <div className="grid gap-2">
          <Label htmlFor="locale">{t("signup.language")}</Label>
          <select id="locale" name="locale" defaultValue="th" className="h-11 rounded-lg border border-input bg-background px-3 text-base">
            <option value="th">ไทย</option>
            <option value="en">English</option>
          </select>
        </div>
        <Notice notice={error ? { role: "alert", text: error } : null} />
        <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={busy}>
          {t("signup.submit")}
        </Button>
      </form>
      <p>
        {t("signup.have")} <Link href="/account/login">{t("login.submit")}</Link>
      </p>
    </MobileShell>
  );
}
