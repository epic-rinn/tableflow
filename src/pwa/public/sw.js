/* TableFlow guest PWA service worker (MVP-17, PWA-001–003).
 *
 * Caches only public, versioned static assets and a generic offline page.
 * Never intercepts: non-GET requests (no background replay), other origins,
 * /api/*, React Server Component/data requests, or pages (network-only; on
 * failure the generic offline page — never a cached page, bill or QR route).
 * A new version waits until the guest chooses to reload (SKIP_WAITING).
 */
const VERSION = new URL(self.location.href).searchParams.get("v") || "dev";
const CACHE = `tableflow-static-${VERSION}`;
const OFFLINE = "/offline.html";
const PRECACHE = [OFFLINE, "/icons/icon-192.png", "/icons/icon-512.png", "/icons/maskable-512.png", "/manifest.webmanifest"];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(keys.filter((k) => k.startsWith("tableflow-") && k !== CACHE).map((k) => caches.delete(k)))),
  );
});

self.addEventListener("message", (event) => {
  if (event.data === "SKIP_WAITING") self.skipWaiting();
});

function isRSC(request, url) {
  return request.headers.get("RSC") === "1" || request.headers.has("Next-Router-State-Tree") || url.searchParams.has("_rsc");
}

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api/") || isRSC(request, url)) return;

  if (request.mode === "navigate") {
    event.respondWith(fetch(request).catch(() => caches.match(OFFLINE)));
    return;
  }
  if (url.pathname.startsWith("/_next/static/") || url.pathname.startsWith("/icons/")) {
    event.respondWith(
      caches.open(CACHE).then(async (cache) => {
        const hit = await cache.match(request);
        if (hit) return hit;
        const response = await fetch(request);
        if (response.ok && response.type === "basic") cache.put(request, response.clone());
        return response;
      }),
    );
    return;
  }
  if (url.pathname === "/manifest.webmanifest") {
    event.respondWith(fetch(request).catch(() => caches.match(request)));
  }
});
