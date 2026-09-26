"use client";

import { RefreshCw } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";

// Registers the guest service worker (production only) and offers updates
// instead of applying them silently: carts live in sessionStorage per visit,
// so "Reload to update" keeps them (PWA-A3).
export function ServiceWorkerManager() {
  const { t } = useI18n();
  const [waiting, setWaiting] = useState<ServiceWorker | null>(null);

  useEffect(() => {
    if (process.env.NODE_ENV !== "production" || !("serviceWorker" in navigator)) return;
    let reloaded = false;
    const onControllerChange = () => {
      if (reloaded) return;
      reloaded = true;
      window.location.reload();
    };
    navigator.serviceWorker.addEventListener("controllerchange", onControllerChange);
    const version = process.env.NEXT_PUBLIC_BUILD_VERSION ?? "dev";
    void navigator.serviceWorker.register(`/sw.js?v=${version}`, { scope: "/" }).then((reg) => {
      // An update found while an older worker controls the page waits for the guest.
      const offer = (w: ServiceWorker | null) => {
        if (w && navigator.serviceWorker.controller) setWaiting(w);
      };
      offer(reg.waiting);
      reg.addEventListener("updatefound", () => {
        const w = reg.installing;
        w?.addEventListener("statechange", () => {
          if (w.state === "installed") offer(w);
        });
      });
    }).catch(() => {
      // Registration failure only disables offline support; the site still works.
    });
    return () => navigator.serviceWorker.removeEventListener("controllerchange", onControllerChange);
  }, []);

  if (!waiting) return null;
  return (
    <div role="status" className="fixed inset-x-0 top-0 z-50 mx-auto flex max-w-md items-center gap-3 bg-foreground px-4 py-3 text-sm text-background shadow-lg">
      <span className="flex-1">{t("update.available")}</span>
      <Button type="button" size="sm" variant="secondary" onClick={() => waiting.postMessage("SKIP_WAITING")}>
        <RefreshCw aria-hidden /> {t("update.reload")}
      </Button>
    </div>
  );
}
