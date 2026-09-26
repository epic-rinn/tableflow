"use client";

import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";

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
    <main>
      <h1>Reset your password</h1>
      {sent ? (
        <p role="status">If an account uses that email, a reset link is on its way. It expires in 1 hour.</p>
      ) : (
        <form onSubmit={onSubmit} noValidate>
          <Field id="email" label="Email" type="email" autoComplete="email" required />
          {error && <p role="alert">{error}</p>}
          <button type="submit">Send reset link</button>
        </form>
      )}
    </main>
  );
}
