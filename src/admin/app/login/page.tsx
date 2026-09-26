"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api/client";
import type { StaffIdentity } from "@/lib/api/types";

export default function LoginPage() {
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    const res = await api<StaffIdentity>("/sessions/staff", {
      method: "POST",
      body: { email: String(form.get("email") ?? ""), password: String(form.get("password") ?? "") },
    });
    setBusy(false);
    if (!res.ok) {
      setError(res.error.message);
      return;
    }
    router.replace("/");
    router.refresh();
  }

  return (
    <main>
      <h1>Staff sign in</h1>
      <form onSubmit={onSubmit} noValidate>
        <p>
          <label htmlFor="email">Email</label>
          <input id="email" name="email" type="email" autoComplete="username" required />
        </p>
        <p>
          <label htmlFor="password">Password</label>
          <input id="password" name="password" type="password" autoComplete="current-password" required />
        </p>
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={busy}>
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
