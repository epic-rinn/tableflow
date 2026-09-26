"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import type { Member } from "@/lib/api/types";

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
    router.push("/account");
  }

  return (
    <main>
      <h1>Member sign in</h1>
      <form onSubmit={onSubmit} noValidate>
        <Field id="email" label="Email" type="email" autoComplete="username" required />
        <Field id="password" label="Password" type="password" autoComplete="current-password" required />
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={busy}>
          Sign in
        </button>
      </form>
      <p>
        <Link href="/account/reset">Forgot your password?</Link> · <Link href="/account/signup">Create an account</Link>
      </p>
    </main>
  );
}
