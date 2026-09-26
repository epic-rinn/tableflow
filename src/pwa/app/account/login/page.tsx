"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import type { Member } from "@/lib/api/types";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

export default function MemberLoginPage() {
  const { t, errorText } = useI18n();
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    const res = await api<Member>("/sessions/member", {
      method: "POST",
      body: { email: form.get("email"), password: form.get("password") },
    });
    setBusy(false);
    if (!res.ok) {
      setError(errorText(res.error));
      return;
    }
    // Only same-site paths (e.g. back to the table page); never another origin.
    const next = new URLSearchParams(window.location.search).get("next") ?? "";
    router.push(/^\/(?![/\\])/.test(next) ? next : "/account");
  }

  return (
    <MobileShell eyebrow={t("account.eyebrow")} title={t("login.title")}>
      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <Field id="email" label={t("login.email")} type="email" autoComplete="username" required />
        <Field id="password" label={t("login.password")} type="password" autoComplete="current-password" required />
        <Notice notice={error ? { role: "alert", text: error } : null} />
        <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={busy}>
          {t("login.submit")}
        </Button>
      </form>
      <p>
        <Link href="/account/reset">{t("login.forgot")}</Link> · <Link href="/account/signup">{t("login.create")}</Link>
      </p>
    </MobileShell>
  );
}
