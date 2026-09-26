"use client";

import { Plus, Save, Trash2, Undo2 } from "lucide-react";
import { useCallback, useEffect, useId, useState } from "react";
import { Notice, type NoticeValue } from "@/components/common/Notice";
import { PageHeader } from "@/components/common/PageHeader";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
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

function Field({ label, children }: { label: string; children: (id: string) => React.ReactNode }) {
  const id = useId();
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id} className="text-xs text-muted-foreground">{label}</Label>
      {children(id)}
    </div>
  );
}

function Names({ value, onChange, label }: { value: { name_th: string; name_en: string }; onChange: (v: { name_th: string; name_en: string }) => void; label: string }) {
  return (
    <>
      <label className="grid gap-1.5">
        <span className="text-xs text-muted-foreground">{label} (ไทย)</span>
        <Input lang="th" value={value.name_th} maxLength={80} onChange={(e) => onChange({ ...value, name_th: e.target.value })} />
      </label>
      <label className="grid gap-1.5">
        <span className="text-xs text-muted-foreground">{label} (English)</span>
        <Input value={value.name_en} maxLength={80} onChange={(e) => onChange({ ...value, name_en: e.target.value })} />
      </label>
    </>
  );
}

function RemoveButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <Button type="button" variant="ghost" size="sm" onClick={onClick} className="text-destructive hover:text-destructive">
      <Trash2 aria-hidden /> {label}
    </Button>
  );
}

export function MenuEditor({ branchId }: { branchId: string }) {
  const [revision, setRevision] = useState(0);
  const [draft, setDraft] = useState<DCategory[]>([]);
  const [selected, setSelected] = useState(0);
  const [message, setMessage] = useState<NoticeValue>(null);

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

  const ci = Math.min(selected, Math.max(draft.length - 1, 0));
  const c = draft[ci];
  return (
    <>
      <PageHeader
        title="Menu"
        description={<>Removed entries are retired, not deleted: past orders keep their names and prices. Revision {revision}.</>}
        actions={
          <>
            <Button type="button" variant="outline" onClick={() => void load()}>
              <Undo2 aria-hidden /> Discard changes
            </Button>
            <Button type="button" onClick={save}>
              <Save aria-hidden /> Save menu
            </Button>
          </>
        }
      />
      <Notice notice={message} className="mb-4" />
      <div className="grid gap-6 lg:grid-cols-[240px_1fr]">
        <nav aria-label="Menu categories" className="grid content-start gap-1">
          {draft.map((cat, i) => (
            <Button key={i} type="button" variant="ghost" aria-current={i === ci ? "true" : undefined}
              className={cn("justify-start", i === ci && "bg-accent text-accent-foreground")} onClick={() => setSelected(i)}>
              <span className="truncate">{cat.name_en || cat.name_th || `Category ${i + 1}`}</span>
              <span className="ml-auto text-xs text-muted-foreground">{cat.items.length}</span>
            </Button>
          ))}
          <Button type="button" variant="outline" className="mt-2 justify-start"
            onClick={() => { set((d) => d.push({ name_th: "", name_en: "", items: [] })); setSelected(draft.length); }}>
            <Plus aria-hidden /> Add category
          </Button>
        </nav>

        {c ? (
          <fieldset className="grid content-start gap-4">
            <legend className="sr-only">Category {ci + 1}</legend>
            <Card>
              <CardHeader>
                <CardTitle>Category {ci + 1}</CardTitle>
                <CardAction>
                  <RemoveButton label="Remove category" onClick={() => { set((d) => d.splice(ci, 1)); setSelected(Math.max(0, ci - 1)); }} />
                </CardAction>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2">
                <Names label="Category" value={c} onChange={(v) => set((d) => Object.assign(d[ci], v))} />
              </CardContent>
            </Card>

            {c.items.map((it, ii) => (
              <Card key={ii} role="group" aria-label={it.name_en || "New item"}>
                <CardHeader>
                  <CardTitle>{it.name_en || "New item"}</CardTitle>
                  <CardAction>
                    <RemoveButton label="Remove item" onClick={() => set((d) => d[ci].items.splice(ii, 1))} />
                  </CardAction>
                </CardHeader>
                <CardContent className="grid gap-4">
                  <div className="grid gap-3 sm:grid-cols-[1fr_1fr_140px]">
                    <Names label="Item" value={it} onChange={(v) => set((d) => Object.assign(d[ci].items[ii], v))} />
                    <Field label="Price (฿)">
                      {(id) => <Input id={id} inputMode="decimal" value={it.price} onChange={(e) => set((d) => { d[ci].items[ii].price = e.target.value; })} />}
                    </Field>
                  </div>
                  {it.groups.map((g, gi) => (
                    <fieldset key={gi} className="grid gap-3 rounded-lg border bg-muted/30 p-3">
                      <legend className="px-1 text-sm font-medium">Options: {g.name_en || "new group"}</legend>
                      <div className="grid gap-3 sm:grid-cols-[1fr_1fr_80px_80px]">
                        <Names label="Group" value={g} onChange={(v) => set((d) => Object.assign(d[ci].items[ii].groups[gi], v))} />
                        <Field label="Min">
                          {(id) => <Input id={id} type="number" min={0} max={20} value={g.min} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].min = Number(e.target.value); })} />}
                        </Field>
                        <Field label="Max">
                          {(id) => <Input id={id} type="number" min={1} max={20} value={g.max} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].max = Number(e.target.value); })} />}
                        </Field>
                      </div>
                      {g.options.map((o, oi) => (
                        <div key={oi} className="grid items-end gap-3 sm:grid-cols-[1fr_1fr_100px_auto]">
                          <Names label="Option" value={o} onChange={(v) => set((d) => Object.assign(d[ci].items[ii].groups[gi].options[oi], v))} />
                          <Field label="+฿">
                            {(id) => <Input id={id} inputMode="decimal" value={o.delta} onChange={(e) => set((d) => { d[ci].items[ii].groups[gi].options[oi].delta = e.target.value; })} />}
                          </Field>
                          <RemoveButton label="Remove option" onClick={() => set((d) => d[ci].items[ii].groups[gi].options.splice(oi, 1))} />
                        </div>
                      ))}
                      <div className="flex flex-wrap gap-2">
                        <Button type="button" variant="outline" size="sm" onClick={() => set((d) => d[ci].items[ii].groups[gi].options.push({ name_th: "", name_en: "", delta: "0" }))}>
                          <Plus aria-hidden /> Add option
                        </Button>
                        <RemoveButton label="Remove group" onClick={() => set((d) => d[ci].items[ii].groups.splice(gi, 1))} />
                      </div>
                    </fieldset>
                  ))}
                  <Button type="button" variant="outline" size="sm" className="justify-self-start"
                    onClick={() => set((d) => d[ci].items[ii].groups.push({ name_th: "", name_en: "", min: 0, max: 1, options: [{ name_th: "", name_en: "", delta: "0" }] }))}>
                    <Plus aria-hidden /> Add option group
                  </Button>
                </CardContent>
              </Card>
            ))}
            <Button type="button" variant="outline" className="justify-self-start" onClick={() => set((d) => d[ci].items.push({ name_th: "", name_en: "", price: "0", groups: [] }))}>
              <Plus aria-hidden /> Add item
            </Button>
          </fieldset>
        ) : (
          <p className="rounded-xl border border-dashed p-10 text-center text-sm text-muted-foreground">No categories yet. Add one to start the menu.</p>
        )}
      </div>
    </>
  );
}
