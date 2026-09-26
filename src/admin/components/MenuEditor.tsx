"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { Menu } from "@/lib/api/types";
import { bahtToSatang, satangToBaht } from "@/lib/money";

// Editable draft: prices as typed baht strings; ids kept for existing rows.
type DOption = { id?: string; name_th: string; name_en: string; delta: string };
type DGroup = { id?: string; name_th: string; name_en: string; min: number; max: number; options: DOption[] };
type DItem = { id?: string; name_th: string; name_en: string; price: string; groups: DGroup[] };
type DCategory = { id?: string; name_th: string; name_en: string; items: DItem[] };

function toDraft(m: Menu): DCategory[] {
  return m.categories.map((c) => ({
    id: c.id, name_th: c.name_th, name_en: c.name_en,
    items: c.items.map((i) => ({
      id: i.id, name_th: i.name_th, name_en: i.name_en, price: satangToBaht(i.price_satang),
      groups: i.option_groups.map((g) => ({
        id: g.id, name_th: g.name_th, name_en: g.name_en, min: g.min_choices, max: g.max_choices,
        options: g.options.map((o) => ({ id: o.id, name_th: o.name_th, name_en: o.name_en, delta: satangToBaht(o.price_delta_satang) })),
      })),
    })),
  }));
}

function toBody(d: DCategory[]): { ok: true; categories: unknown[] } | { ok: false; error: string } {
  const bad: string[] = [];
  const price = (s: string, where: string) => {
    const v = bahtToSatang(s);
    if (v === null) bad.push(where);
    return v ?? 0;
  };
  const categories = d.map((c) => ({
    ...(c.id ? { id: c.id } : {}), name_th: c.name_th, name_en: c.name_en,
    items: c.items.map((i) => ({
      ...(i.id ? { id: i.id } : {}), name_th: i.name_th, name_en: i.name_en, price_satang: price(i.price, i.name_en || "item"),
      option_groups: i.groups.map((g) => ({
        ...(g.id ? { id: g.id } : {}), name_th: g.name_th, name_en: g.name_en, min_choices: g.min, max_choices: g.max,
        options: g.options.map((o) => ({ ...(o.id ? { id: o.id } : {}), name_th: o.name_th, name_en: o.name_en, price_delta_satang: price(o.delta, o.name_en || "option") })),
      })),
    })),
  }));
  return bad.length ? { ok: false, error: `Invalid price for: ${bad.join(", ")}` } : { ok: true, categories };
}

function Names({ value, onChange, label }: { value: { name_th: string; name_en: string }; onChange: (v: { name_th: string; name_en: string }) => void; label: string }) {
  return (
    <>
      <label>
        {label} (ไทย) <input value={value.name_th} maxLength={80} onChange={(e) => onChange({ ...value, name_th: e.target.value })} />
      </label>{" "}
      <label>
        {label} (English) <input value={value.name_en} maxLength={80} onChange={(e) => onChange({ ...value, name_en: e.target.value })} />
      </label>
    </>
  );
}

export function MenuEditor({ branchId }: { branchId: string }) {
  const [revision, setRevision] = useState(0);
  const [draft, setDraft] = useState<DCategory[]>([]);
  const [message, setMessage] = useState<{ role: "alert" | "status"; text: string } | null>(null);

  const load = useCallback(async () => {
    const r = await api<Menu>(`/branches/${branchId}/menu`);
    if (r.ok) {
      setRevision(r.data.revision);
      setDraft(toDraft(r.data));
    } else setMessage({ role: "alert", text: r.error.message });
  }, [branchId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function save() {
    const body = toBody(draft);
    if (!body.ok) {
      setMessage({ role: "alert", text: body.error });
      return;
    }
    const r = await api<Menu>(`/branches/${branchId}/menu`, { method: "PUT", body: { expected_revision: revision, categories: body.categories } });
    if (r.ok) {
      setRevision(r.data.revision);
      setDraft(toDraft(r.data));
      setMessage({ role: "status", text: `Menu saved (revision ${r.data.revision}).` });
    } else {
      const fields = Object.entries(r.error.fields ?? {}).map(([k, v]) => ` ${k}: ${v}.`).join("");
      setMessage({ role: "alert", text: `${r.error.message}${fields}` });
    }
  }

  const set = (fn: (d: DCategory[]) => void) => setDraft((prev) => {
    const next = structuredClone(prev);
    fn(next);
    return next;
  });

  return (
    <>
      <h1>Menu</h1>
      <p>Removed entries are retired, not deleted: past orders keep their names and prices. Revision {revision}.</p>
      {message && <p role={message.role}>{message.text}</p>}
      {draft.map((c, ci) => (
        <fieldset key={ci} className="menu-category">
          <legend>Category {ci + 1}</legend>
          <Names label="Category" value={c} onChange={(v) => set((d) => Object.assign(d[ci], v))} />{" "}
          <button type="button" onClick={() => set((d) => d.splice(ci, 1))}>Remove category</button>
          {c.items.map((it, ii) => (
            <fieldset key={ii} className="menu-item">
              <legend>{it.name_en || "New item"}</legend>
              <Names label="Item" value={it} onChange={(v) => set((d) => Object.assign(d[ci].items[ii], v))} />{" "}
              <label>
                Price (฿) <input inputMode="decimal" value={it.price} size={8} onChange={(e) => set((d) => { d[ci].items[ii].price = e.target.value; })} />
              </label>{" "}
              <button type="button" onClick={() => set((d) => d[ci].items.splice(ii, 1))}>Remove item</button>
              {it.groups.map((g, gi) => (
                <fieldset key={gi} className="menu-group">
                  <legend>Options: {g.name_en || "new group"}</legend>
                  <Names label="Group" value={g} onChange={(v) => set((d) => Object.assign(d[ci].items[ii].groups[gi], v))} />{" "}
                  <label>
                    Min <input type="number" min={0} max={20} value={g.min} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].min = Number(e.target.value); })} />
                  </label>{" "}
                  <label>
                    Max <input type="number" min={1} max={20} value={g.max} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].max = Number(e.target.value); })} />
                  </label>{" "}
                  <button type="button" onClick={() => set((d) => d[ci].items[ii].groups.splice(gi, 1))}>Remove group</button>
                  {g.options.map((o, oi) => (
                    <p key={oi}>
                      <Names label="Option" value={o} onChange={(v) => set((d) => Object.assign(d[ci].items[ii].groups[gi].options[oi], v))} />{" "}
                      <label>
                        +฿ <input inputMode="decimal" size={6} value={o.delta} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].options[oi].delta = e.target.value; })} />
                      </label>{" "}
                      <button type="button" onClick={() => set((d) => d[ci].items[ii].groups[gi].options.splice(oi, 1))}>Remove option</button>
                    </p>
                  ))}
                  <button type="button" onClick={() => set((d) => d[ci].items[ii].groups[gi].options.push({ name_th: "", name_en: "", delta: "0" }))}>Add option</button>
                </fieldset>
              ))}
              <button type="button" onClick={() => set((d) => d[ci].items[ii].groups.push({ name_th: "", name_en: "", min: 0, max: 1, options: [{ name_th: "", name_en: "", delta: "0" }] }))}>
                Add option group
              </button>
            </fieldset>
          ))}
          <button type="button" onClick={() => set((d) => d[ci].items.push({ name_th: "", name_en: "", price: "0", groups: [] }))}>Add item</button>
        </fieldset>
      ))}
      <p>
        <button type="button" onClick={() => set((d) => d.push({ name_th: "", name_en: "", items: [] }))}>Add category</button>{" "}
        <button type="button" onClick={save}>Save menu</button>{" "}
        <button type="button" onClick={() => void load()}>Discard changes</button>
      </p>
    </>
  );
}
