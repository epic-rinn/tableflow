"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

// Confirmation dialog for consequential actions that need an audited reason.
export function ReasonDialog({ trigger, title, description, reasonLabel, confirmLabel, destructive, disabled, onConfirm }: {
  trigger: string; title: string; description: string; reasonLabel: string; confirmLabel: string;
  destructive?: boolean; disabled?: boolean; onConfirm: (reason: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const id = `reason-${reasonLabel.replace(/\W+/g, "-").toLowerCase()}`;
  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) setReason(""); }}>
      <DialogTrigger asChild>
        <Button type="button" variant="outline" size="sm" disabled={disabled} className={destructive ? "text-destructive hover:text-destructive" : undefined}>
          {trigger}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            onConfirm(reason.trim());
            setOpen(false);
            setReason("");
          }}
        >
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor={id}>{reasonLabel}</Label>
            <Textarea id={id} value={reason} maxLength={500} required onChange={(e) => setReason(e.target.value)} />
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">Cancel</Button>
            </DialogClose>
            <Button type="submit" variant={destructive ? "destructive" : "default"} disabled={!reason.trim()}>
              {confirmLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
