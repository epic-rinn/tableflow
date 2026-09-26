"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { AuthShell, FieldError } from "@/components/common/AuthShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
      <AuthShell title="Account activated">
        <p role="status" className="text-sm">
          Your password is set.{" "}
          <Link href="/login" className="font-medium text-primary underline-offset-4 hover:underline">
            Sign in
          </Link>
        </p>
      </AuthShell>
    );
  }
  if (token === null) return <main aria-busy="true" />;
  if (token === "") {
    return (
      <AuthShell title="Activation link missing">
        <Notice notice={{ role: "alert", text: "Open the full activation link your manager sent you." }} />
      </AuthShell>
    );
  }
  return (
    <AuthShell title="Activate your staff account" description="Choose a password to finish setting up your account.">
      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <div className="grid gap-2">
          <Label htmlFor="display_name">Display name (optional)</Label>
          <Input id="display_name" name="display_name" maxLength={100} autoComplete="name" />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="password">New password (at least 12 characters)</Label>
          <Input id="password" name="password" type="password" minLength={12} autoComplete="new-password" required
            aria-invalid={!!fields.password} aria-describedby={fields.password ? "password-error" : undefined} />
          <FieldError id="password-error" message={fields.password} />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="confirm">Confirm password</Label>
          <Input id="confirm" name="confirm" type="password" autoComplete="new-password" required
            aria-invalid={!!fields.confirm} aria-describedby={fields.confirm ? "confirm-error" : undefined} />
          <FieldError id="confirm-error" message={fields.confirm} />
        </div>
        <Notice notice={error ? { role: "alert", text: error } : null} />
        <Button type="submit" size="lg" disabled={busy}>
          {busy ? "Activating…" : "Activate account"}
        </Button>
      </form>
    </AuthShell>
  );
}
