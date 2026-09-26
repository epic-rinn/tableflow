"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { ApiError, ApiResult } from "@/lib/api/client";

export type Poll<T> = {
  data: T | null;
  error: ApiError | null;
  updatedAt: Date | null;
  refresh: () => void;
};

const MAX_BACKOFF_MS = 60_000;

// Polls with ±20% jitter. Stops while the tab is hidden and fetches at once
// when it becomes visible again or the network returns. Failures back off
// exponentially. In-flight requests are aborted on unmount or re-poll.
export function usePolling<T>(fetcher: (signal: AbortSignal) => Promise<ApiResult<T>>, intervalMs: number): Poll<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const fetcherRef = useRef(fetcher);
  const runRef = useRef<() => void>(() => {});

  useEffect(() => {
    fetcherRef.current = fetcher;
  }, [fetcher]);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;
    let failures = 0;
    let stopped = false;

    const schedule = (ms: number) => {
      clearTimeout(timer);
      if (stopped || document.hidden) return;
      timer = setTimeout(run, ms);
    };

    async function run() {
      clearTimeout(timer);
      if (stopped || document.hidden) return;
      controller?.abort();
      controller = new AbortController();
      const res = await fetcherRef.current(controller.signal);
      if (stopped || res.status === -1) return; // unmounted or superseded
      if (res.ok) {
        failures = 0;
        setData(res.data);
        setError(null);
        setUpdatedAt(new Date());
        schedule(intervalMs * (0.8 + Math.random() * 0.4));
      } else {
        failures += 1;
        setError(res.error);
        schedule(Math.min(intervalMs * 2 ** failures, MAX_BACKOFF_MS));
      }
    }

    const onVisibility = () => {
      if (document.hidden) {
        clearTimeout(timer);
        controller?.abort();
      } else {
        void run();
      }
    };
    const onOnline = () => void run();
    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("online", onOnline);
    runRef.current = () => void run();
    void run();
    return () => {
      stopped = true;
      clearTimeout(timer);
      controller?.abort();
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("online", onOnline);
    };
  }, [intervalMs]);

  const refresh = useCallback(() => runRef.current(), []);
  return { data, error, updatedAt, refresh };
}
