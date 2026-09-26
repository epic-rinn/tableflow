"use client";

import { RefreshCw, WifiOff } from "lucide-react";
import { useEffect, useState } from "react";
import type { ApiError } from "@/lib/api/client";
import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";

// Shows when data was last refreshed and warns when it may be stale (PWA-003).
export function Freshness({ updatedAt, error, intervalMs, className }: { updatedAt: Date | null; error: ApiError | null; intervalMs: number; className?: string }) {
  const { t, errorText } = useI18n();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  const stale = !!error || (updatedAt !== null && now - updatedAt.getTime() > intervalMs * 3);
  return (
    <p
      className={cn(
        "inline-flex items-center gap-1.5 text-xs",
        stale ? "rounded-lg bg-warning-soft px-2 py-1 font-medium text-warning" : "text-muted-foreground",
        className,
      )}
      role="status"
      aria-live="polite"
    >
      {stale ? <WifiOff className="size-3.5" aria-hidden /> : <RefreshCw className="size-3.5" aria-hidden />}
      {updatedAt ? t("common.updated", { time: updatedAt.toLocaleTimeString() }) : t("common.loading")}
      {stale && <strong>{t("common.stale")}{error ? ` (${errorText(error)})` : ""}</strong>}
    </p>
  );
}
