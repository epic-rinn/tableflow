"use client";

import Link from "next/link";
import { useState } from "react";
import { Field } from "@/components/Field";
import { api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";

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
      <main>
        <h1>Password changed</h1>
        <p role="status">
          You were signed out everywhere. <Link href="/account/login">Sign in with your new password</Link>
        </p>
      </main>
    );
  }
  return (
    <main>
      <h1>Choose a new password</h1>
      {token === "" ? (
        <p role="alert">Open the full link from your email.</p>
      ) : (
        <form onSubmit={onSubmit} noValidate>
          <Field id="new_password" label="New password (at least 12 characters)" type="password" autoComplete="new-password" required error={fields.new_password} />
          <Field id="confirm" label="Confirm new password" type="password" autoComplete="new-password" required error={fields.confirm} />
          {error && <p role="alert">{error}</p>}
          <button type="submit" disabled={!token}>
            Change password
          </button>
        </form>
      )}
    </main>
  );
}
