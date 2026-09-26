"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";
import { MobileShell } from "@/components/common/MobileShell";

export default function VerifyPage() {
  const token = useFragmentToken();
  const [result, setResult] = useState<{ ok: boolean; message: string } | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    void api("/members/verify", { method: "POST", body: { token } }).then((res) => {
      if (!cancelled) setResult(res.ok ? { ok: true, message: "Your email is confirmed." } : { ok: false, message: res.error.message });
    });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <MobileShell eyebrow="Member account" title="Confirm your email">
      {token === "" && <p role="alert">Open the full link from your email.</p>}
      {token && !result && <p role="status">Confirming…</p>}
      {result?.ok && (
        <p role="status">
          {result.message} <Link href="/account">Go to your account</Link>
        </p>
      )}
      {result && !result.ok && <p role="alert">{result.message}</p>}
    </MobileShell>
  );
}
