"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";

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
      <main>
        <h1>Check your email</h1>
        <p role="status">We sent a link to finish setting up your account. You can sign in meanwhile.</p>
        <Link href="/account/login">Sign in</Link>
      </main>
    );
  }
  return (
    <main>
      <h1>Create an account</h1>
      <form onSubmit={onSubmit} noValidate>
        <Field id="email" label="Email" type="email" autoComplete="email" required error={fields.email} />
        <Field id="password" label="Password (at least 12 characters)" type="password" autoComplete="new-password" required error={fields.password} />
        <p>
          <label htmlFor="locale">Language</label>
          <select id="locale" name="locale" defaultValue="th">
            <option value="th">ไทย</option>
            <option value="en">English</option>
          </select>
        </p>
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={busy}>
          Create account
        </button>
      </form>
      <p>
        Already a member? <Link href="/account/login">Sign in</Link>
      </p>
    </main>
  );
}
