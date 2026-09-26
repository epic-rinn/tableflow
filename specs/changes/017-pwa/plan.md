# Change: 017-pwa — Installable PWA and cache isolation

Status: done (M5 gate passed 2026-09-27). Date: 2026-09-27. Scope owner: Claude. Task: MVP-17. Verification: M5 gate (ADR-0004).

## Problem and behavior

Maps PWA-001–003 and ADM-006 ([access/PWA](../../features/05-access-pwa.md)).

- **Installable:** the guest PWA gets a manifest, icons and install guidance where the browser supports it. Normal browser use is unchanged.
- **Service worker:** it caches only versioned public static assets and a generic offline page. It never stores private data and never replays mutations.
- **Updates:** a new version waits until the guest chooses to reload, so carts are never silently lost.
- **Admin:** has no service worker.

## Decisions (reversible)

- **Handwritten service worker:** `public/sw.js`, with no Workbox, so every rule is readable and testable.
  - **Registration:** `/sw.js?v=<build version>`. The version is fixed at build time (`NEXT_PUBLIC_BUILD_VERSION`), and the cache name `tableflow-static-<version>`.
  - **Install:** precaches `/offline.html`, the icons and the manifest.
  - **Activate:** deletes other `tableflow-*` caches. There is no `skipWaiting` on install and no `clients.claim` before the guest agrees.
  - **Fetch rules:**

    | Request | Handling |
    | --- | --- |
    | Non-GET | not intercepted (no background replay) |
    | Cross-origin | not intercepted |
    | `/api/*` | not intercepted |
    | RSC or data requests (`RSC` header, `_rsc` query) | not intercepted |
    | Navigations | network-only; on failure the cached generic `/offline.html` (never a cached page, bill or QR route) |
    | `/_next/static/*` (content-hashed, immutable), `/icons/*` | cache-first |
    | `/manifest.webmanifest` | network with cache fallback |
    | Everything else | not intercepted |

- **Update flow:** an update found while a worker controls the page shows the "Update available" banner. "Reload to update" posts `SKIP_WAITING`, and the page reloads on `controllerchange`. Carts live in sessionStorage per visit, so they survive the reload.
- **Install guidance:** a dismissible card on the home page.
  - With `beforeinstallprompt`: an "Install app" button.
  - On iOS Safari: "Share → Add to Home Screen" text.
  - Otherwise nothing. There is no nagging on QR, queue or dining pages.
- **Icons:** 192, 512 and maskable 512 PNGs plus a 180 apple-touch icon, rendered from the TableFlow SVG mark (own brand, no third-party assets).
- **Admin:** never registers a worker, and its smoke test asserts that. The PWA worker is scoped to the PWA origin, so it can never see admin pages or staff cookies (ADM-A2).

## Verification map

| Requirement | Test | Status |
| --- | --- | --- |
| PWA-001 manifest, icons, worker registered on the PWA | `pwa.spec.ts` › installable | passing |
| PWA-A1 offline never restores a private bill; cache holds only public static assets | `pwa.spec.ts` › offline and cache contents | passing |
| PWA-A3 update does not discard the cart | `pwa.spec.ts` › update keeps cart | passing |
| ADM-A2 / ADM-006 no worker on admin | admin `smoke.spec.ts` | passing (existing) |
