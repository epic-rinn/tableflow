"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";

export default function ResetConfirmPage() {
  const token = useFragmentToken();
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const password = String(form.get("new_password") ?? "");
    if (password !== String(form.get("confirm") ?? "")) {
      setFields({ confirm: "Passwords do not match" });
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
    setError(res.error.message);
  }

  if (done) {
    return (
      <MobileShell eyebrow="Member account" title="Password changed">
        <p role="status">
          You were signed out everywhere. <Link href="/account/login">Sign in with your new password</Link>
        </p>
      </MobileShell>
    );
  }
  return (
    <MobileShell eyebrow="Member account" title="Choose a new password">
      {token === "" ? (
        <p role="alert">Open the full link from your email.</p>
      ) : (
        <form onSubmit={onSubmit} noValidate className="grid gap-4">
          <Field id="new_password" label="New password (at least 12 characters)" type="password" autoComplete="new-password" required error={fields.new_password} />
          <Field id="confirm" label="Confirm new password" type="password" autoComplete="new-password" required error={fields.confirm} />
          <Notice notice={error ? { role: "alert", text: error } : null} />
          <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={!token}>
            Change password
          </Button>
        </form>
      )}
    </MobileShell>
  );
}
