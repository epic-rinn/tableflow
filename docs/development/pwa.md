# Customer PWA development

Runtime location: `src/pwa/`. Shell implemented in MVP-01; no customer features or service worker yet. Read [system architecture](../../specs/architecture/system.md) and [PWA requirements](../../specs/features/05-access-pwa.md).

The project has its own `package.json`, pnpm lockfile, TypeScript/Next.js config, `app/`, and Playwright smoke tests (dev port 3000). Add `features/<domain>` for UI behavior, `components/` for shared UI, `lib/api/` for contract-derived transport, and `public/` for static PWA assets only as needed. These paths are relative to `src/pwa/`.

The browser calls same-origin `/api/v1`; Go owns business mutations. Environment variable `API_INTERNAL_URL` is server-only. Commands: [setup](setup.md) (`make pwa-check`, `make smoke`). `proxy.ts` forwards `/api/v1/*` to `API_INTERNAL_URL` at request time.

This application contains guest queue, dining/order, bill-view, login, and loyalty screens. Staff queue management, kitchen, cashier, and manager functions belong to the separate admin application.

Use Server Components for initial reads and narrow client components for interaction. Client validation is a usability aid; Go remains authoritative. Never share-cache private reads or put sessions in localStorage. The service worker must exclude private data and must not replay offline mutations. Review polling cleanup, fetch waterfalls, payload sizes, and accessibility with every affected feature. Keep this guide and AI instructions outside the runtime project.
