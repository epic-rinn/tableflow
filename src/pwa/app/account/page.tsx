"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { Member } from "@/lib/api/types";
import { MobileShell } from "@/components/common/MobileShell";
import { LoyaltySummary } from "@/components/LoyaltySummary";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";

type State = { step: "loading" } | { step: "signed-out" } | { step: "member"; member: Member } | { step: "error"; message: string };

export default function AccountPage() {
  const [state, setState] = useState<State>({ step: "loading" });
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let cancelled = false;
    void api<Member>("/members/me").then((res) => {
      if (cancelled) return;
      if (res.ok) setState({ step: "member", member: res.data });
      else if (res.status === 401) setState({ step: "signed-out" });
      else setState({ step: "error", message: res.error.message });
    });
    return () => {
      cancelled = true;
    };
  }, []);

  async function signOut() {
    const res = await api("/sessions/member", { method: "DELETE" });
    if (res.ok || res.status === 401) setState({ step: "signed-out" });
    else setNotice(res.error.message);
  }

  async function resend(email: string) {
    const res = await api("/members/verification", { method: "POST", body: { email } });
    setNotice(res.ok ? "If your email still needs confirming, a new link is on its way." : res.error.message);
  }

  return (
    <MobileShell eyebrow="Member account" title="Your account">
      {state.step === "loading" && <p role="status">Loading…</p>}
      {state.step === "error" && <Notice notice={{ role: "alert", text: state.message }} />}
      {state.step === "signed-out" && (
        <p>
          <Link href="/account/login">Sign in</Link> or <Link href="/account/signup">create an account</Link>. Membership is
          optional.
        </p>
      )}
      {state.step === "member" && (
        <>
          <p>Signed in as {state.member.email}</p>
          <LoyaltySummary />
          {state.member.email_verified ? (
            <p>Email confirmed.</p>
          ) : (
            <p>
              Email not confirmed yet.{" "}
              <Button type="button" variant="outline" size="lg" className="h-12 rounded-2xl" onClick={() => resend(state.member.email)}>
                Send a new confirmation link
              </Button>
            </p>
          )}
          <Button type="button" variant="outline" size="lg" className="h-12 rounded-2xl" onClick={signOut}>
            Sign out
          </Button>
        </>
      )}
      <Notice notice={notice ? { role: "status", text: notice } : null} />
    </MobileShell>
  );
}
