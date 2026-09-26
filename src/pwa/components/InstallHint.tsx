"use client";

import { Download, Share, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";

type InstallEvent = Event & { prompt: () => Promise<void>; userChoice: Promise<{ outcome: string }> };

const DISMISSED = "tableflow.install-dismissed";

// Install guidance only where supported (PWA-001): Chrome/Android get a button
// via beforeinstallprompt; iOS Safari gets Share → Add to Home Screen. Normal
// browser use is always fine, so this is optional and dismissible.
export function InstallHint() {
  const [prompt, setPrompt] = useState<InstallEvent | null>(null);
  const [ios, setIos] = useState(false);
  const [hidden, setHidden] = useState(true);

  useEffect(() => {
    let dismissed = false;
    try {
      dismissed = localStorage.getItem(DISMISSED) === "1";
    } catch {
      // storage unavailable: show the hint
    }
    const standalone = window.matchMedia("(display-mode: standalone)").matches || (navigator as { standalone?: boolean }).standalone === true;
    if (dismissed || standalone) return;
    const onPrompt = (e: Event) => {
      e.preventDefault();
      setPrompt(e as InstallEvent);
      setHidden(false);
    };
    window.addEventListener("beforeinstallprompt", onPrompt);
    if (/iPhone|iPad|iPod/.test(navigator.userAgent) && /Safari/.test(navigator.userAgent) && !/CriOS|FxiOS/.test(navigator.userAgent)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setIos(true);
      setHidden(false);
    }
    return () => window.removeEventListener("beforeinstallprompt", onPrompt);
  }, []);

  function dismiss() {
    setHidden(true);
    try {
      localStorage.setItem(DISMISSED, "1");
    } catch {
      // ignore
    }
  }

  if (hidden) return null;
  return (
    <section aria-labelledby="install-title" className="flex items-start gap-3 rounded-2xl border bg-card p-4">
      <Download className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden />
      <div className="grid flex-1 gap-2 text-sm">
        <h2 id="install-title" className="font-semibold">Add TableFlow to your home screen</h2>
        {ios ? (
          <p>
            Tap <Share className="inline size-4" aria-label="Share" /> then “Add to Home Screen”.
          </p>
        ) : (
          <Button type="button" size="sm" className="justify-self-start" onClick={() => void prompt?.prompt().then(() => setHidden(true))}>
            Install app
          </Button>
        )}
      </div>
      <Button type="button" variant="ghost" size="icon" aria-label="Dismiss install hint" onClick={dismiss}>
        <X aria-hidden />
      </Button>
    </section>
  );
}
