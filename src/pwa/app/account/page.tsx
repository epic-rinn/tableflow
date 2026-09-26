"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { Member } from "@/lib/api/types";

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
    <main>
      <h1>Your account</h1>
      {state.step === "loading" && <p role="status">Loading…</p>}
      {state.step === "error" && <p role="alert">{state.message}</p>}
      {state.step === "signed-out" && (
        <p>
          <Link href="/account/login">Sign in</Link> or <Link href="/account/signup">create an account</Link>. Membership is
          optional.
        </p>
      )}
      {state.step === "member" && (
        <>
          <p>Signed in as {state.member.email}</p>
          {state.member.email_verified ? (
            <p>Email confirmed.</p>
          ) : (
            <p>
              Email not confirmed yet.{" "}
              <button type="button" onClick={() => resend(state.member.email)}>
                Send a new confirmation link
              </button>
            </p>
          )}
          <button type="button" onClick={signOut}>
            Sign out
          </button>
        </>
      )}
      {notice && <p role="status">{notice}</p>}
    </main>
  );
}
