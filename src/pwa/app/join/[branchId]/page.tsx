"use client";

import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { Need, Ticket } from "@/lib/api/types";
import { useIdempotent } from "@/lib/useIdempotent";

const NEEDS: { id: Need; label: string }[] = [
  { id: "accessible", label: "Wheelchair-accessible table" },
  { id: "high_chair", label: "High chair" },
];

// Entrance QR → join the queue. The anonymous session scopes the join's
// Idempotency-Key, so a retried submit returns the same ticket.
export default function JoinPage() {
  const { branchId } = useParams<{ branchId: string }>();
  const router = useRouter();
  const run = useIdempotent();
  const [ready, setReady] = useState(false);
  const [party, setParty] = useState(2);
  const [needs, setNeeds] = useState<Need[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void api("/sessions/anonymous", { method: "POST" }).then((res) => {
      if (cancelled) return;
      if (res.ok) setReady(true);
      else setError(res.error.message);
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
    setError(unknown ? `${res.error.message} Tap “Join the queue” again — you will not get a second ticket.` : res.error.message);
  }

  return (
    <main>
      <h1>Join the queue</h1>
      <form onSubmit={onSubmit}>
        <p>
          <label htmlFor="party">Number of people</label>
          <input id="party" type="number" min={1} max={50} value={party} onChange={(e) => setParty(Number(e.target.value))} />
        </p>
        <fieldset>
          <legend>Seating needs (optional)</legend>
          {NEEDS.map((n) => (
            <label key={n.id}>
              <input type="checkbox" checked={needs.includes(n.id)} onChange={(e) => setNeeds(e.target.checked ? [...needs, n.id] : needs.filter((x) => x !== n.id))} />{" "}
              {n.label}
            </label>
          ))}
        </fieldset>
        {error && <p role="alert">{error}</p>}
        <button type="submit" disabled={!ready || busy}>
          {busy ? "Joining…" : "Join the queue"}
        </button>
      </form>
    </main>
  );
}
