"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";

// The activation token arrives in the URL fragment (never sent to servers or
// in Referer). It is read once and removed from the visible URL/history.
export default function ActivatePage() {
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Reads the fragment on mount and whenever another link is opened in
    // this tab (a fragment-only navigation does not remount the page).
    function readFragment() {
      const fragment = window.location.hash.slice(1);
      if (fragment) {
        window.history.replaceState(null, "", window.location.pathname);
      }
      setToken(fragment);
      setDone(false);
      setError("");
      setFields({});
    }
    readFragment();
    window.addEventListener("hashchange", readFragment);
    return () => window.removeEventListener("hashchange", readFragment);
  }, []);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const password = String(form.get("password") ?? "");
    if (password !== String(form.get("confirm") ?? "")) {
      setFields({ confirm: "Passwords do not match" });
      return;
    }
    setBusy(true);
    setError("");
    setFields({});
    const res = await api("/staff/activate", {
      method: "POST",
      body: { token, password, display_name: String(form.get("display_name") ?? "") },
    });
    setBusy(false);
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
        <h1>Account activated</h1>
        <p role="status">
          Your password is set. <Link href="/login">Sign in</Link>
        </p>
      </main>
    );
  }
  if (token === null) return <main aria-busy="true" />;
  if (token === "") {
    return (
      <main>
        <h1>Activation link missing</h1>
        <p role="alert">Open the full activation link your manager sent you.</p>
      </main>
    );
  }
  return (
    <main>
      <h1>Activate your staff account</h1>
      <form onSubmit={onSubmit} noValidate>
        <p>
          <label htmlFor="display_name">Display name (optional)</label>
          <input id="display_name" name="display_name" maxLength={100} autoComplete="name" />
        </p>
        <p>
          <label htmlFor="password">New password (at least 12 characters)</label>
          <input id="password" name="password" type="password" minLength={12} autoComplete="new-password" required
            aria-invalid={!!fields.password} aria-describedby={fields.password ? "password-error" : undefined} />
          {fields.password && <span id="password-error"> {fields.password}</span>}
        </p>
        <p>
          <label htmlFor="confirm">Confirm password</label>
          <input id="confirm" name="confirm" type="password" autoComplete="new-password" required
            aria-invalid={!!fields.confirm} aria-describedby={fields.confirm ? "confirm-error" : undefined} />
          {fields.confirm && <span id="confirm-error"> {fields.confirm}</span>}
        </p>
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={busy}>
          {busy ? "Activating…" : "Activate account"}
        </button>
      </form>
    </main>
  );
}
