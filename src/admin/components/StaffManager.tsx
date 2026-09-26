"use client";

import { useRouter } from "next/navigation";
import { Copy, MailPlus, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { StateBadge } from "@/components/common/StateBadge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
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
      <PageHeader title="Staff" description="Invite staff, assign roles and remove access. Role changes sign the person out." />
      {error && (
        <Notice className="mb-4" notice={{ role: "alert", text: `${error.message}${Object.entries(error.fields ?? {}).map(([k, v]) => ` ${k}: ${v}.`).join("")}` }} />
      )}
      <div className="grid gap-6 xl:grid-cols-[360px_1fr]">
        <Card role="region" aria-labelledby="invite-title" className="self-start">
          <CardHeader>
            <CardTitle>
              <h2 id="invite-title">Invite staff</h2>
            </CardTitle>
            <CardDescription>They receive a one-time activation link to set a password.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <form onSubmit={invite} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="invite-email">Email</Label>
                <Input id="invite-email" name="email" type="email" required />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="invite-name">Display name</Label>
                <Input id="invite-name" name="display_name" required maxLength={100} />
              </div>
              <fieldset className="grid gap-2">
                <legend className="mb-1 text-sm font-medium">Roles</legend>
                <div className="grid grid-cols-2 gap-2">
                  {ROLES.map((r) => (
                    <label key={r} className="flex items-center gap-2 rounded-lg border p-2 text-sm has-checked:border-primary has-checked:bg-accent">
                      <input type="checkbox" name={`role-${r}`} className="size-4 accent-primary" /> {ROLE_LABELS[r]}
                    </label>
                  ))}
                </div>
              </fieldset>
              <Button type="submit">
                <MailPlus aria-hidden /> Create invitation
              </Button>
            </form>
            {link && (
              <div role="status" aria-live="polite" className="grid gap-2 rounded-lg border border-primary/30 bg-accent/60 p-3 text-sm">
                <p>
                  One-time activation link for {link.email} (expires {new Date(link.expires).toLocaleString()}). Share it privately; it will not be
                  shown again.
                </p>
                <div className="flex gap-2">
                  <Input aria-label="Activation link" readOnly value={link.url} onFocus={(e) => e.currentTarget.select()} className="font-mono text-xs" />
                  <Button type="button" variant="outline" size="icon" aria-label="Copy link" onClick={() => void navigator.clipboard?.writeText(link.url)}>
                    <Copy aria-hidden />
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        <Card role="region" aria-labelledby="list-title">
          <CardHeader>
            <CardTitle className="flex items-center justify-between gap-2">
              <h2 id="list-title">Accounts</h2>
              <Button type="button" variant="outline" size="sm" onClick={() => void load(null)}>
                <RefreshCw aria-hidden /> Refresh
              </Button>
            </CardTitle>
            <CardDescription>Last refreshed {loadedAt ? new Date(loadedAt).toLocaleTimeString() : "—"}</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead scope="col">Name</TableHead>
                  <TableHead scope="col">Email</TableHead>
                  <TableHead scope="col">Status</TableHead>
                  <TableHead scope="col">Roles</TableHead>
                  <TableHead scope="col">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((s) => (
                  <StaffRow key={`${s.id}:${s.version}`} staff={s} onRoles={saveRoles} onDeactivate={deactivate} onReissue={reissue} />
                ))}
              </TableBody>
            </Table>
            {next && (
              <Button type="button" variant="outline" className="justify-self-center" onClick={() => void load(next)}>
                Load more
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

type RowProps = {
  staff: Staff;
  onRoles: (s: Staff, roles: Role[]) => Promise<void>;
  onDeactivate: (s: Staff, reason: string) => Promise<void>;
  onReissue: (s: Staff) => Promise<void>;
};

const STATUS_TONE: Record<string, string> = { active: "active", invited: "invited", disabled: "deactivated" };

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
    <TableRow className={disabled ? "text-muted-foreground" : ""}>
      <TableCell className="font-medium">{staff.display_name}</TableCell>
      <TableCell>{staff.email}</TableCell>
      <TableCell>
        <StateBadge state={STATUS_TONE[staff.status] ?? staff.status} label={staff.status} />
      </TableCell>
      <TableCell className="whitespace-normal">
        <fieldset disabled={disabled || busy} className="flex flex-wrap gap-x-3 gap-y-1">
          <legend className="sr-only">Roles for {staff.display_name}</legend>
          {ROLES.map((r) => (
            <label key={r} className="inline-flex items-center gap-1.5 text-sm">
              <input
                type="checkbox"
                className="size-4 accent-primary"
                checked={roles.includes(r)}
                onChange={(e) => setRoles(e.target.checked ? [...roles, r] : roles.filter((x) => x !== r))}
              />
              {ROLE_LABELS[r]}
            </label>
          ))}
        </fieldset>
      </TableCell>
      <TableCell>
        {!disabled && (
          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" size="sm" variant="outline" disabled={!changed || roles.length === 0 || busy} onClick={() => run(() => onRoles(staff, roles))}>
              Save roles
            </Button>
            {staff.status === "invited" && (
              <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => run(() => onReissue(staff))}>
                New activation link
              </Button>
            )}
            <label>
              <span className="sr-only">Reason to deactivate {staff.display_name}</span>
              <Input placeholder="Reason" value={reason} maxLength={500} onChange={(e) => setReason(e.target.value)} className="h-7 w-32" />
            </label>
            <Button type="button" size="sm" variant="destructive" disabled={!reason.trim() || busy} onClick={() => run(() => onDeactivate(staff, reason))}>
              Deactivate
            </Button>
          </div>
        )}
      </TableCell>
    </TableRow>
  );
}
