"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/client";
import { useFragmentToken } from "@/lib/useFragmentToken";
import { QueueTracking } from "./QueueTracking";
import { Dining } from "./Dining";
import type { CapabilityKind, GuestSession } from "@/lib/api/types";

type State =
  | { step: "reading" }
  | { step: "missing" }
  | { step: "exchanging" }
  | { step: "connected"; session: GuestSession }
  | { step: "failed"; message: string };

const COPY: Record<CapabilityKind, { title: string; connected: string }> = {
  queue: { title: "Queue ticket", connected: "You are connected to your queue ticket." },
  visit: { title: "Your table", connected: "You are connected to your table." },
};

// QR links carry the capability token in the URL fragment, which browsers
// never send to servers or in Referer. It is removed from the address bar and
// history immediately, then exchanged once through a POST body for an
// HttpOnly session cookie. The token is never stored in browser storage.
export function QrEntry({ kind }: { kind: CapabilityKind }) {
  const token = useFragmentToken(); // re-read when another QR link opens in this tab
  const [result, setResult] = useState<{ token: string; state: State } | null>(null);
  const exchanged = useRef<string | null>(null);

  useEffect(() => {
    if (token === null || exchanged.current === token) return; // StrictMode re-runs effects in dev
    exchanged.current = token;
    let cancelled = false;
    // No fragment (reload, reopened app): resume this phone's existing guest
    // session for this kind of page, if any; otherwise exchange the token.
    const request = token
      ? api<GuestSession>("/sessions/capability", { method: "POST", body: { token, kind } })
      : api<GuestSession>("/sessions/guest");
    void request.then((res) => {
      if (cancelled) return;
      let next: State;
      if (res.ok && res.data.kind === kind) next = { step: "connected", session: res.data };
      else if (!token) next = { step: "missing" };
      else next = { step: "failed", message: res.ok ? "This QR code is for a different page." : res.error.message };
      setResult({ token, state: next });
    });
    return () => {
      cancelled = true;
    };
  }, [token, kind]);

  const state: State =
    token === null ? { step: "reading" } : result?.token === token ? result.state : { step: "exchanging" };
  const copy = COPY[kind];
  return (
    <main>
      <h1>{copy.title}</h1>
      {state.step === "reading" || state.step === "exchanging" ? (
        <p role="status" aria-live="polite">
          Connecting…
        </p>
      ) : null}
      {state.step === "connected" && (
        <>
          <p role="status" aria-live="polite">
            {copy.connected}
          </p>
          {kind === "queue" ? <QueueTracking ticketId={state.session.resource_id} /> : <Dining visitId={state.session.resource_id} />}
        </>
      )}
      {state.step === "missing" && <p role="alert">Scan the QR code again, or ask staff for help.</p>}
      {state.step === "failed" && <p role="alert">{state.message}</p>}
    </main>
  );
}
