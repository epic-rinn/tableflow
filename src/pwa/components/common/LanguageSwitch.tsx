"use client";

import { Languages } from "lucide-react";
import { useI18n } from "@/lib/i18n";

// TH/EN toggle in every page header (PWA-004).
export function LanguageSwitch() {
  const { locale, setLocale, t } = useI18n();
  return (
    <button
      type="button"
      onClick={() => setLocale(locale === "th" ? "en" : "th")}
      aria-label={t("lang.switchLabel")}
      className="inline-flex min-h-11 items-center gap-1.5 rounded-full border border-primary-foreground/60 px-3 text-sm font-semibold"
    >
      <Languages className="size-4" aria-hidden />
      <span lang={locale === "th" ? "en" : "th"}>{t("lang.switch")}</span>
    </button>
  );
}
