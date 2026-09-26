"use client";

import { Plus, Save, Trash2 } from "lucide-react";
import { useCallback, useEffect, useId, useState } from "react";
import { Notice } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { StateBadge } from "@/components/common/StateBadge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api/client";
import { NEED_LABELS, NEEDS, type Group, type Need, type Table } from "@/lib/api/types";

export function FloorConfig({ branchId }: { branchId: string }) {
  const [tables, setTables] = useState<Table[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [message, setMessage] = useState<{ kind: "alert" | "status"; text: string } | null>(null);

  const load = useCallback(async () => {
    const [t, g] = await Promise.all([
      api<{ items: Table[] }>(`/branches/${branchId}/tables`),
      api<{ groups: Group[] }>(`/branches/${branchId}/seating-groups`),
    ]);
    if (t.ok) setTables(t.data.items);
    if (g.ok) setGroups(g.data.groups);
    if (!t.ok || !g.ok) setMessage({ kind: "alert", text: (t.ok ? g : t).ok ? "" : "Could not load the floor configuration." });
  }, [branchId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function save<T>(promise: Promise<{ ok: boolean; error?: { message: string; fields?: Record<string, string> } } & T>, done: string) {
    const res = await promise;
    if (res.ok) {
      setMessage({ kind: "status", text: done });
      await load();
    } else {
      const fields = Object.entries(res.error?.fields ?? {}).map(([k, v]) => ` ${k}: ${v}.`).join("");
      setMessage({ kind: "alert", text: `${res.error?.message ?? "Failed"}${fields}` });
    }
  }

  return (
    <>
      <PageHeader title="Tables & seating groups" description="Configure the floor the host board and queue use." />
      <Notice notice={message?.text ? { role: message.kind, text: message.text } : null} className="mb-4" />
      <div className="grid gap-6 xl:grid-cols-[1fr_380px]">
        <Card role="region" aria-labelledby="tables-title">
          <CardHeader>
            <CardTitle>
              <h2 id="tables-title">Tables</h2>
            </CardTitle>
            <CardDescription>Label, seats and supported needs. Inactive tables are never offered for seating.</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="rounded-lg border border-dashed p-3">
              <TableForm submitLabel="Add table" onSubmit={(v) => save(api(`/branches/${branchId}/tables`, { method: "POST", body: v }), `Table ${v.label} added.`)} />
            </div>
            <ul className="grid gap-2">
              {tables.map((t) => (
                <li key={`${t.id}:${t.version}`} className="rounded-lg border p-3">
                  <TableForm table={t} submitLabel={`Save ${t.label}`}
                    onSubmit={(v) => save(api(`/tables/${t.id}`, { method: "PATCH", body: { ...v, expected_version: t.version } }), `Table ${v.label} saved.`)} />
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>

        <Card role="region" aria-labelledby="groups-title" className="self-start">
          <CardHeader>
            <CardTitle>
              <h2 id="groups-title">Seating groups</h2>
            </CardTitle>
            <CardDescription>Party-size bands used for queue positions. They can change only while nobody is waiting or called.</CardDescription>
          </CardHeader>
          <CardContent>
            <GroupsEditor groups={groups} onSave={(g) => save(api(`/branches/${branchId}/seating-groups`, { method: "PUT", body: { groups: g } }), "Seating groups saved.")} />
          </CardContent>
        </Card>
      </div>
    </>
  );
}

type TableValues = { label: string; capacity: number; needs: Need[]; active: boolean };

function TableForm({ table, submitLabel, onSubmit }: { table?: Table; submitLabel: string; onSubmit: (v: TableValues) => void }) {
  const [v, setV] = useState<TableValues>({
    label: table?.label ?? "",
    capacity: table?.capacity ?? 4,
    needs: table?.needs ?? [],
    active: table?.active ?? true,
  });
  const uid = useId();
  return (
    <form
      className="flex flex-wrap items-end gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        // Creation takes no "active" field; new tables start active.
        onSubmit(table ? v : ({ label: v.label, capacity: v.capacity, needs: v.needs } as TableValues));
      }}>
      <div className="grid gap-1.5">
        <Label htmlFor={`${uid}-label`} className="text-xs text-muted-foreground">Label</Label>
        <Input id={`${uid}-label`} value={v.label} maxLength={20} required onChange={(e) => setV({ ...v, label: e.target.value })} className="w-28" />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor={`${uid}-seats`} className="text-xs text-muted-foreground">Seats</Label>
        <Input id={`${uid}-seats`} type="number" min={1} max={50} value={v.capacity} onChange={(e) => setV({ ...v, capacity: Number(e.target.value) })} className="w-20" />
      </div>
      <div className="flex flex-wrap items-center gap-3 pb-1.5">
        {NEEDS.map((n) => (
          <Label key={n} className="font-normal">
            <Checkbox checked={v.needs.includes(n)} onCheckedChange={(c) => setV({ ...v, needs: c === true ? [...v.needs, n] : v.needs.filter((x) => x !== n) })} />
            {NEED_LABELS[n]}
          </Label>
        ))}
        {table && (
          <Label className="font-normal">
            <Checkbox checked={v.active} onCheckedChange={(c) => setV({ ...v, active: c === true })} /> Active
          </Label>
        )}
      </div>
      <div className="ml-auto flex items-center gap-2">
        {table && <StateBadge state={table.state} />}
        <Button type="submit" variant={table ? "outline" : "default"} size="sm">
          {table ? <Save aria-hidden /> : <Plus aria-hidden />} {submitLabel}
        </Button>
      </div>
    </form>
  );
}

function GroupsEditor({ groups, onSave }: { groups: Group[]; onSave: (g: Group[]) => void }) {
  const [rows, setRows] = useState<Group[] | null>(null);
  const current = rows ?? groups.map(({ label, min_party, max_party }) => ({ label, min_party, max_party }));
  const update = (i: number, patch: Partial<Group>) => setRows(current.map((g, j) => (i === j ? { ...g, ...patch } : g)));
  return (
    <form
      className="grid gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        onSave(current);
        setRows(null);
      }}>
      {current.map((g, i) => (
        <div key={i} className="grid grid-cols-[1fr_64px_64px_auto] items-end gap-2">
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">Label</span>
            <Input value={g.label} maxLength={40} onChange={(e) => update(i, { label: e.target.value })} />
          </label>
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">From</span>
            <Input type="number" min={1} max={50} value={g.min_party} onChange={(e) => update(i, { min_party: Number(e.target.value) })} />
          </label>
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">To</span>
            <Input type="number" min={1} max={50} value={g.max_party} onChange={(e) => update(i, { max_party: Number(e.target.value) })} />
          </label>
          <Button type="button" variant="ghost" size="icon" aria-label={`Remove group ${g.label}`} onClick={() => setRows(current.filter((_, j) => j !== i))}>
            <Trash2 aria-hidden />
          </Button>
        </div>
      ))}
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => {
          const last = current.at(-1);
          const from = (last?.max_party ?? 0) + 1;
          setRows([...current, { label: `${from}–${from + 1}`, min_party: from, max_party: from + 1 }]);
        }}>
          <Plus aria-hidden /> Add group
        </Button>
        <Button type="submit" size="sm">
          <Save aria-hidden /> Save groups
        </Button>
      </div>
    </form>
  );
}
