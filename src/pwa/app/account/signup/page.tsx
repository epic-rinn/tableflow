"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

export default function SignupPage() {
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
    setError(res.error.message);
  }

  if (sent) {
    return (
      <MobileShell eyebrow="Member account" title="Check your email">
        <p role="status">We sent a link to finish setting up your account. You can sign in meanwhile.</p>
        <Link href="/account/login">Sign in</Link>
      </MobileShell>
    );
  }
  return (
    <MobileShell eyebrow="Member account" title="Create an account">
      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <Field id="email" label="Email" type="email" autoComplete="email" required error={fields.email} />
        <Field id="password" label="Password (at least 12 characters)" type="password" autoComplete="new-password" required error={fields.password} />
        <div className="grid gap-2">
          <Label htmlFor="locale">Language</Label>
          <select id="locale" name="locale" defaultValue="th" className="h-11 rounded-lg border border-input bg-background px-3 text-base">
            <option value="th">ไทย</option>
            <option value="en">English</option>
          </select>
        </div>
        <Notice notice={error ? { role: "alert", text: error } : null} />
        <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={busy}>
          Create account
        </Button>
      </form>
      <p>
        Already a member? <Link href="/account/login">Sign in</Link>
      </p>
    </MobileShell>
  );
}
