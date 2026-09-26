"use client";

import { useEffect, useState } from "react";

// Reads a single-use token from the URL fragment (never sent to servers or in
// Referer), removes it from the address bar/history, and re-reads when another
// link is opened in the same tab. null = not read yet, "" = missing.
export function useFragmentToken(): string | null {
  const [token, setToken] = useState<string | null>(null);
  useEffect(() => {
    function read() {
      const fragment = window.location.hash.slice(1);
      if (fragment) window.history.replaceState(null, "", window.location.pathname);
      setToken(fragment);
    }
    read();
    window.addEventListener("hashchange", read);
    return () => window.removeEventListener("hashchange", read);
  }, []);
  return token;
}
