"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api, type ApiResult } from "@/lib/api/client";
import {
  ROLE_LABELS,
  ROLES,
  type Activation,
  type ApiError,
  type Role,
  type Staff,
  type StaffPage,
} from "@/lib/api/types";

type Props = { branchId: string; selfId: string };

function activationLink(token: string) {
  return `${window.location.origin}/activate#${token}`;
}

export function StaffManager({ branchId, selfId }: Props) {
  const router = useRouter();
  const [items, setItems] = useState<Staff[]>([]);
  const [next, setNext] = useState<string | null>(null);
  const [loadedAt, setLoadedAt] = useState<string>("");
  const [error, setError] = useState<ApiError | null>(null);
  const [link, setLink] = useState<{ email: string; url: string; expires: string } | null>(null);

  // Any 401 means the session ended (logout, expiry or role change).
  const handle = useCallback(
    <T,>(res: ApiResult<T>): res is Extract<ApiResult<T>, { ok: true }> => {
      if (res.ok) return true;
      if (res.status === 401) {
        router.replace("/login");
        router.refresh();
      }
      setError(res.error);
      return false;
    },
    [router],
  );

  const load = useCallback(
    async (cursor: string | null) => {
      const q = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
      const res = await api<StaffPage>(`/branches/${branchId}/staff${q}`);
      if (!handle(res)) return;
      setError(null);
      setItems((prev) => (cursor ? [...prev, ...res.data.items] : res.data.items));
      setNext(res.data.next_cursor);
      setLoadedAt(res.data.server_time);
    },
    [branchId, handle],
  );

  useEffect(() => {
    // Initial fetch on mount; state updates happen after the request resolves.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(null);
  }, [load]);

  async function invite(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const formEl = e.currentTarget;
    const form = new FormData(formEl);
    const roles = ROLES.filter((r) => form.get(`role-${r}`) === "on");
    const res = await api<{ staff: Staff; activation: Activation }>(`/branches/${branchId}/staff`, {
      method: "POST",
      body: { email: form.get("email"), display_name: form.get("display_name"), roles },
    });
    if (!handle(res)) return;
    setLink({ email: res.data.staff.email, url: activationLink(res.data.activation.token), expires: res.data.activation.expires_at });
    formEl.reset();
    await load(null);
  }

  async function reissue(s: Staff) {
    const res = await api<Activation>(`/staff/${s.id}/activation`, { method: "POST" });
    if (!handle(res)) return;
    setLink({ email: s.email, url: activationLink(res.data.token), expires: res.data.expires_at });
  }

  async function saveRoles(s: Staff, roles: Role[]) {
    const res = await api<Staff>(`/staff/${s.id}/roles`, { method: "PATCH", body: { expected_version: s.version, roles } });
    if (!handle(res)) return;
    setError(null);
    if (s.id === selfId) {
      // Changing your own roles revokes your session.
      router.replace("/login");
      router.refresh();
      return;
    }
    await load(null);
  }

  async function deactivate(s: Staff, reason: string) {
    const res = await api<Staff>(`/staff/${s.id}/deactivate`, { method: "POST", body: { expected_version: s.version, reason } });
    if (!handle(res)) return;
    setError(null);
    await load(null);
  }

  return (
    <>
      <h1>Staff</h1>
      {error && (
        <p role="alert">
          {error.message}
          {Object.entries(error.fields ?? {}).map(([k, v]) => ` ${k}: ${v}.`)}
        </p>
      )}

      <section aria-labelledby="invite-title">
        <h2 id="invite-title">Invite staff</h2>
        <form onSubmit={invite}>
          <p>
            <label htmlFor="invite-email">Email</label>
            <input id="invite-email" name="email" type="email" required />
          </p>
          <p>
            <label htmlFor="invite-name">Display name</label>
            <input id="invite-name" name="display_name" required maxLength={100} />
          </p>
          <fieldset>
            <legend>Roles</legend>
            {ROLES.map((r) => (
              <label key={r}>
                <input type="checkbox" name={`role-${r}`} /> {ROLE_LABELS[r]}
              </label>
            ))}
          </fieldset>
          <button type="submit">Create invitation</button>
        </form>
        {link && (
          <div role="status" aria-live="polite">
            <p>
              One-time activation link for {link.email} (expires {new Date(link.expires).toLocaleString()}). Share it
              privately; it will not be shown again.
            </p>
            <input aria-label="Activation link" readOnly value={link.url} onFocus={(e) => e.currentTarget.select()} />
          </div>
        )}
      </section>

      <section aria-labelledby="list-title">
        <h2 id="list-title">Accounts</h2>
        <p>
          <small>Last refreshed {loadedAt ? new Date(loadedAt).toLocaleTimeString() : "—"}</small>{" "}
          <button type="button" onClick={() => void load(null)}>
            Refresh
          </button>
        </p>
        <table>
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Email</th>
              <th scope="col">Status</th>
              <th scope="col">Roles</th>
              <th scope="col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((s) => (
              <StaffRow key={`${s.id}:${s.version}`} staff={s} onRoles={saveRoles} onDeactivate={deactivate} onReissue={reissue} />
            ))}
          </tbody>
        </table>
        {next && (
          <button type="button" onClick={() => void load(next)}>
            Load more
          </button>
        )}
      </section>
    </>
  );
}

type RowProps = {
  staff: Staff;
  onRoles: (s: Staff, roles: Role[]) => Promise<void>;
  onDeactivate: (s: Staff, reason: string) => Promise<void>;
  onReissue: (s: Staff) => Promise<void>;
};

function StaffRow({ staff, onRoles, onDeactivate, onReissue }: RowProps) {
  const [roles, setRoles] = useState<Role[]>(staff.roles);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const disabled = staff.status === "disabled";
  const changed = roles.slice().sort().join() !== staff.roles.slice().sort().join();

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    await fn();
    setBusy(false);
  }

  return (
    <tr>
      <td>{staff.display_name}</td>
      <td>{staff.email}</td>
      <td>{staff.status}</td>
      <td>
        <fieldset disabled={disabled || busy}>
          <legend className="sr-only">Roles for {staff.display_name}</legend>
          {ROLES.map((r) => (
            <label key={r}>
              <input
                type="checkbox"
                checked={roles.includes(r)}
                onChange={(e) => setRoles(e.target.checked ? [...roles, r] : roles.filter((x) => x !== r))}
              />{" "}
              {ROLE_LABELS[r]}
            </label>
          ))}
        </fieldset>
      </td>
      <td>
        {!disabled && (
          <>
            <button type="button" disabled={!changed || roles.length === 0 || busy} onClick={() => run(() => onRoles(staff, roles))}>
              Save roles
            </button>
            {staff.status === "invited" && (
              <button type="button" disabled={busy} onClick={() => run(() => onReissue(staff))}>
                New activation link
              </button>
            )}
            <label>
              <span className="sr-only">Reason to deactivate {staff.display_name}</span>
              <input placeholder="Reason" value={reason} maxLength={500} onChange={(e) => setReason(e.target.value)} />
            </label>
            <button type="button" disabled={!reason.trim() || busy} onClick={() => run(() => onDeactivate(staff, reason))}>
              Deactivate
            </button>
          </>
        )}
      </td>
    </tr>
  );
}
