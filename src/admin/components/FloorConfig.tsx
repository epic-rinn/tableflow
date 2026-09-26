"use client";

import { useCallback, useEffect, useState } from "react";
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
      <h1>Tables &amp; seating groups</h1>
      {message?.text && <p role={message.kind}>{message.text}</p>}

      <section aria-labelledby="groups-title">
        <h2 id="groups-title">Seating groups</h2>
        <p>Party-size bands used for queue positions. They can change only while nobody is waiting or called.</p>
        <GroupsEditor groups={groups} onSave={(g) => save(api(`/branches/${branchId}/seating-groups`, { method: "PUT", body: { groups: g } }), "Seating groups saved.")} />
      </section>

      <section aria-labelledby="tables-title">
        <h2 id="tables-title">Tables</h2>
        <TableForm submitLabel="Add table" onSubmit={(v) => save(api(`/branches/${branchId}/tables`, { method: "POST", body: v }), `Table ${v.label} added.`)} />
        <ul>
          {tables.map((t) => (
            <li key={`${t.id}:${t.version}`}>
              <TableForm table={t} submitLabel={`Save ${t.label}`}
                onSubmit={(v) => save(api(`/tables/${t.id}`, { method: "PATCH", body: { ...v, expected_version: t.version } }), `Table ${v.label} saved.`)} />
            </li>
          ))}
        </ul>
      </section>
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
  const id = table?.id ?? "new";
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        // Creation takes no "active" field; new tables start active.
        onSubmit(table ? v : ({ label: v.label, capacity: v.capacity, needs: v.needs } as TableValues));
      }}>
      <label>
        Label <input id={`label-${id}`} value={v.label} maxLength={20} required onChange={(e) => setV({ ...v, label: e.target.value })} />
      </label>{" "}
      <label>
        Seats <input type="number" min={1} max={50} value={v.capacity} onChange={(e) => setV({ ...v, capacity: Number(e.target.value) })} />
      </label>{" "}
      {NEEDS.map((n) => (
        <label key={n}>
          <input type="checkbox" checked={v.needs.includes(n)} onChange={(e) => setV({ ...v, needs: e.target.checked ? [...v.needs, n] : v.needs.filter((x) => x !== n) })} />{" "}
          {NEED_LABELS[n]}
        </label>
      ))}{" "}
      {table && (
        <label>
          <input type="checkbox" checked={v.active} onChange={(e) => setV({ ...v, active: e.target.checked })} /> Active
        </label>
      )}{" "}
      {table && <small>({table.state})</small>} <button type="submit">{submitLabel}</button>
    </form>
  );
}

function GroupsEditor({ groups, onSave }: { groups: Group[]; onSave: (g: Group[]) => void }) {
  const [rows, setRows] = useState<Group[] | null>(null);
  const current = rows ?? groups.map(({ label, min_party, max_party }) => ({ label, min_party, max_party }));
  const update = (i: number, patch: Partial<Group>) => setRows(current.map((g, j) => (i === j ? { ...g, ...patch } : g)));
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSave(current);
        setRows(null);
      }}>
      {current.map((g, i) => (
        <p key={i}>
          <label>
            Label <input value={g.label} maxLength={40} onChange={(e) => update(i, { label: e.target.value })} />
          </label>{" "}
          <label>
            From <input type="number" min={1} max={50} value={g.min_party} onChange={(e) => update(i, { min_party: Number(e.target.value) })} />
          </label>{" "}
          <label>
            To <input type="number" min={1} max={50} value={g.max_party} onChange={(e) => update(i, { max_party: Number(e.target.value) })} />
          </label>{" "}
          <button type="button" onClick={() => setRows(current.filter((_, j) => j !== i))}>
            Remove
          </button>
        </p>
      ))}
      <button type="button" onClick={() => {
        const last = current.at(-1);
        const from = (last?.max_party ?? 0) + 1;
        setRows([...current, { label: `${from}–${from + 1}`, min_party: from, max_party: from + 1 }]);
      }}>
        Add group
      </button>{" "}
      <button type="submit">Save groups</button>
    </form>
  );
}
