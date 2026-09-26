"use client";

import { QrCode, UserRound } from "lucide-react";
import Link from "next/link";
import { MobileShell } from "@/components/common/MobileShell";
import { InstallHint } from "@/components/InstallHint";
import { useI18n } from "@/lib/i18n";

export default function Home() {
  const { t } = useI18n();
  return (
    <MobileShell title={t("app.name")} eyebrow={t("home.eyebrow")} hero={<p className="mt-1 text-sm">{t("home.tagline")}</p>}>
      <div className="grid gap-3">
        <InstallHint />
        <div className="flex items-center gap-3 rounded-2xl border bg-card p-4">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-accent text-accent-foreground">
            <QrCode className="size-6" aria-hidden />
          </span>
          <p className="text-sm">{t("home.scan")}</p>
        </div>
        <Link href="/account" className="flex items-center gap-3 rounded-2xl border bg-card p-4 no-underline hover:bg-accent">
          <span className="flex size-12 items-center justify-center rounded-2xl bg-secondary">
            <UserRound className="size-6" aria-hidden />
          </span>
          <span className="text-sm font-medium text-foreground">{t("home.account")}</span>
        </Link>
      </div>
    </MobileShell>
  );
}
