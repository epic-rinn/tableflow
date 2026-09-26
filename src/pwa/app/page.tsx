import { QrCode, UserRound } from "lucide-react";
import Link from "next/link";
import { MobileShell } from "@/components/common/MobileShell";
import { InstallHint } from "@/components/InstallHint";

export default function Home() {
  return (
    <MobileShell title="TableFlow" eyebrow="Welcome" hero={<p className="mt-1 text-sm">Queue, order and track your table from your phone.</p>}>
      <div className="grid gap-3">
        <InstallHint />
        <div className="flex items-center gap-3 rounded-2xl border bg-card p-4">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-accent text-accent-foreground">
            <QrCode className="size-6" aria-hidden />
          </span>
          <p className="text-sm">Scan the QR code at the restaurant entrance to join the queue, or the one on your table to order.</p>
        </div>
        <Link href="/account" className="flex items-center gap-3 rounded-2xl border bg-card p-4 no-underline hover:bg-accent">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-secondary">
            <UserRound className="size-6" aria-hidden />
          </span>
          <span className="text-sm font-medium text-foreground">Member account (optional)</span>
        </Link>
      </div>
    </MobileShell>
  );
}
