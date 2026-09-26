"use client";

import { useParams, useRouter } from "next/navigation";
import { Accessibility, Baby, Minus, Plus } from "lucide-react";
import { useEffect, useState } from "react";
import { MobileShell } from "@/components/common/MobileShell";
import { Notice } from "@/components/common/Notice";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { type ApiError, api } from "@/lib/api/client";
import type { Need, Ticket } from "@/lib/api/types";
import { useIdempotent } from "@/lib/useIdempotent";
import { type MessageKey, useI18n } from "@/lib/i18n";

const NEEDS: { id: Need; label: MessageKey }[] = [
  { id: "accessible", label: "join.accessible" },
  { id: "high_chair", label: "join.highChair" },
];

// Entrance QR → join the queue. The anonymous session scopes the join's
// Idempotency-Key, so a retried submit returns the same ticket.
export default function JoinPage() {
  const { branchId } = useParams<{ branchId: string }>();
  const router = useRouter();
  const run = useIdempotent();
  const { t, errorText } = useI18n();
  const [ready, setReady] = useState(false);
  const [party, setParty] = useState(2);
  const [needs, setNeeds] = useState<Need[]>([]);
  const [error, setError] = useState("");
  const [anonError, setAnonError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void api("/sessions/anonymous", { method: "POST" }).then((res) => {
      if (cancelled) return;
      if (res.ok) setReady(true);
      else setAnonError(res.error);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const res = await run(`join:${party}:${needs.join(",")}`, (key) =>
      api<{ ticket: Ticket; tracking: { token: string } }>(`/branches/${branchId}/queue-tickets`, { method: "POST", key, body: { party_size: party, needs } }),
    );
    setBusy(false);
    if (res.ok) {
      router.replace(`/q#${res.data.tracking.token}`);
      return;
    }
    const unknown = res.status === 0 || res.status >= 500;
    setError(unknown ? t("join.retry", { message: errorText(res.error) }) : errorText(res.error));
  }

  return (
    <MobileShell title={t("join.title")} hero={<p className="mt-1 text-sm">{t("join.tagline")}</p>}>
      <form onSubmit={onSubmit} className="flex flex-1 flex-col gap-5">
        <div className="grid gap-3 rounded-2xl border bg-card p-4">
          <Label htmlFor="party" className="text-base">{t("join.people")}</Label>
          <div className="flex items-center gap-3">
            <Button type="button" variant="outline" size="icon-lg" className="size-12 rounded-full" aria-label={t("join.fewer")}
              disabled={party <= 1} onClick={() => setParty(Math.max(1, party - 1))}>
              <Minus aria-hidden />
            </Button>
            <Input id="party" type="number" min={1} max={50} value={party} onChange={(e) => setParty(Number(e.target.value))}
              className="h-12 flex-1 text-center text-2xl font-bold tabular-nums" />
            <Button type="button" variant="outline" size="icon-lg" className="size-12 rounded-full" aria-label={t("join.more")}
              disabled={party >= 50} onClick={() => setParty(Math.min(50, party + 1))}>
              <Plus aria-hidden />
            </Button>
          </div>
        </div>
        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-medium">{t("join.needs")}</legend>
          {NEEDS.map((n) => {
            const Icon = n.id === "accessible" ? Accessibility : Baby;
            return (
              <label key={n.id} className="flex min-h-14 cursor-pointer items-center gap-3 rounded-2xl border bg-card px-4 has-checked:border-primary has-checked:bg-accent">
                <Icon className="size-5 text-muted-foreground" aria-hidden />
                <span className="flex-1 text-sm font-medium">{t(n.label)}</span>
                <input type="checkbox" className="size-5 accent-primary" checked={needs.includes(n.id)}
                  onChange={(e) => setNeeds(e.target.checked ? [...needs, n.id] : needs.filter((x) => x !== n.id))} />
              </label>
            );
          })}
        </fieldset>
        <Notice notice={error || anonError ? { role: "alert", text: error || errorText(anonError!) } : null} />
        <div className="sticky bottom-0 mt-auto bg-background pt-2 pb-safe">
          <Button type="submit" size="lg" className="h-14 w-full rounded-2xl text-base" disabled={!ready || busy}>
            {busy ? t("join.joining") : t("join.submit")}
          </Button>
        </div>
      </form>
    </MobileShell>
  );
}
