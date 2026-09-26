"use client";

import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/client";
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
  const [state, setState] = useState<State>({ step: "reading" });
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return; // StrictMode double-invokes effects in dev
    started.current = true;
    const token = window.location.hash.slice(1);
    if (token) window.history.replaceState(null, "", window.location.pathname);
    async function run() {
      if (!token) {
        setState({ step: "missing" });
        return;
      }
      setState({ step: "exchanging" });
      const res = await api<GuestSession>("/sessions/capability", { method: "POST", body: { token, kind } });
      setState(res.ok ? { step: "connected", session: res.data } : { step: "failed", message: res.error.message });
    }
    void run();
  }, [kind]);

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
        <p role="status" aria-live="polite">
          {copy.connected}
        </p>
      )}
      {state.step === "missing" && <p role="alert">Scan the QR code again, or ask staff for help.</p>}
      {state.step === "failed" && <p role="alert">{state.message}</p>}
    </main>
  );
}
