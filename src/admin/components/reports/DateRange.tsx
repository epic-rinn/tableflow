"use client";

import { Search } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// From/to business dates with quick presets; the API caps ranges at 31 days.
export function DateRange({ from, to, onApply, children }: { from: string; to: string; onApply: (from: string, to: string) => void; children?: React.ReactNode }) {
  const [f, setF] = useState(from);
  const [t, setT] = useState(to);
  return (
    <form
      className="flex flex-wrap items-end gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        onApply(f, t);
      }}
    >
      <div className="grid gap-1.5">
        <Label htmlFor="range-from">From</Label>
        <Input id="range-from" type="date" value={f} onChange={(e) => setF(e.target.value)} required className="w-40" />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="range-to">To</Label>
        <Input id="range-to" type="date" value={t} onChange={(e) => setT(e.target.value)} required className="w-40" />
      </div>
      {children}
      <Button type="submit" variant="outline">
        <Search aria-hidden /> Show
      </Button>
    </form>
  );
}
