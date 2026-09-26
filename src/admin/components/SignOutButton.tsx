"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api/client";

export function SignOutButton() {
  const router = useRouter();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function signOut() {
    setBusy(true);
    const res = await api("/sessions/current", { method: "DELETE" });
    setBusy(false);
    if (res.ok || res.status === 401) {
      router.replace("/login");
      router.refresh();
      return;
    }
    setError(res.error.message);
  }

  return (
    <span>
      <button type="button" onClick={signOut} disabled={busy}>
        Sign out
      </button>
      {error && <span role="alert"> {error}</span>}
    </span>
  );
}
