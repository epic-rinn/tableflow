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

export default function MemberLoginPage() {
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
      setError(res.error.message);
      return;
    }
    // Only same-site paths (e.g. back to the table page); never another origin.
    const next = new URLSearchParams(window.location.search).get("next") ?? "";
    router.push(/^\/(?![/\\])/.test(next) ? next : "/account");
  }

  return (
    <MobileShell eyebrow="Member account" title="Member sign in">
      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <Field id="email" label="Email" type="email" autoComplete="username" required />
        <Field id="password" label="Password" type="password" autoComplete="current-password" required />
        <Notice notice={error ? { role: "alert", text: error } : null} />
        <Button type="submit" size="lg" className="h-12 w-full rounded-2xl text-base" disabled={busy}>
          Sign in
        </Button>
      </form>
      <p>
        <Link href="/account/reset">Forgot your password?</Link> · <Link href="/account/signup">Create an account</Link>
      </p>
    </MobileShell>
  );
}
