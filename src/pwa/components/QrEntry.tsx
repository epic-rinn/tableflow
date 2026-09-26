"use client";

import { ScanLine } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { MobileShell } from "@/components/common/MobileShell";
import { Skeleton } from "@/components/ui/skeleton";
import { type MessageKey, useI18n } from "@/lib/i18n";
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

const COPY: Record<CapabilityKind, { title: MessageKey; connected: MessageKey }> = {
  queue: { title: "qr.queueTitle", connected: "qr.queueConnected" },
  visit: { title: "qr.visitTitle", connected: "qr.visitConnected" },
};

// QR links carry the capability token in the URL fragment, which browsers
// never send to servers or in Referer. It is removed from the address bar and
// history immediately, then exchanged once through a POST body for an
// HttpOnly session cookie. The token is never stored in browser storage.
export function QrEntry({ kind }: { kind: CapabilityKind }) {
  const { t, errorText } = useI18n();
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
      else next = { step: "failed", message: res.ok ? "qr.wrongKind" : `!${res.error.code}\u0000${res.error.message}` };
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
    <MobileShell title={t(copy.title)}
      hero={state.step === "connected" ? <p role="status" aria-live="polite" className="mt-1 text-sm">{t(copy.connected)}</p> : null}>
      {state.step === "reading" || state.step === "exchanging" ? (
        <div role="status" aria-live="polite" className="grid gap-3 py-6">
          <span className="sr-only">{t("qr.connecting")}</span>
          <Skeleton className="h-24 rounded-2xl" />
          <Skeleton className="h-16 rounded-2xl" />
          <Skeleton className="h-16 rounded-2xl" />
        </div>
      ) : null}
      {state.step === "connected" &&
        (kind === "queue" ? <QueueTracking ticketId={state.session.resource_id} /> : <Dining visitId={state.session.resource_id} />)}
      {state.step === "missing" && <Problem text={t("qr.missing")} />}
      {state.step === "failed" && <Problem text={failure(state.message, t, errorText)} />}
    </MobileShell>
  );
}

// Failures are stored as a dictionary key or "!CODE\0server message" so the
// text follows the current language.
function failure(m: string, t: ReturnType<typeof useI18n>["t"], errorText: ReturnType<typeof useI18n>["errorText"]) {
  if (!m.startsWith("!")) return t(m as MessageKey);
  const [code, message] = m.slice(1).split("\u0000");
  return errorText({ code, message, request_id: "", fields: {} });
}

function Problem({ text }: { text: string }) {
  return (
    <div className="grid justify-items-center gap-3 py-10 text-center">
      <span className="flex size-14 items-center justify-center rounded-full bg-destructive-soft text-destructive">
        <ScanLine className="size-7" aria-hidden />
      </span>
      <p role="alert" className="max-w-xs text-sm">
        {text}
      </p>
    </div>
  );
}
