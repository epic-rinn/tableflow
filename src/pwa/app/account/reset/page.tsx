"use client";

import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";

export default function ResetRequestPage() {
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const res = await api("/members/password-reset/request", { method: "POST", body: { email: form.get("email") } });
    if (res.ok) setSent(true);
    else setError(res.error.message);
  }

  return (
    <MobileShell eyebrow="Member account" title="Reset your password">
      {sent ? (
        <p role="status">If an account uses that email, a reset link is on its way. It expires in 1 hour.</p>
      ) : (
        <form onSubmit={onSubmit} noValidate className="grid gap-4">
          <Field id="email" label="Email" type="email" autoComplete="email" required />
          <Notice notice={error ? { role: "alert", text: error } : null} />
          <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base">Send reset link</Button>
        </form>
      )}
    </MobileShell>
  );
}
