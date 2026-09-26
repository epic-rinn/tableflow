# Customer PWA development

Runtime location: `src/pwa/`. App not initialized. Read [system architecture](../../specs/architecture/system.md) and [PWA requirements](../../specs/features/05-access-pwa.md).

M0 creates this project's `package.json`, pnpm lockfile, TypeScript/Next.js config, `app/`, test setup, and pinned tooling. Add `features/<domain>` for UI behavior, `components/` for shared UI, `lib/api/` for contract-derived transport, and `public/` for static PWA assets only as needed. These paths are relative to `src/pwa/`.

The browser calls same-origin `/api/v1`; Go owns business mutations. Environment variable `API_INTERNAL_URL` is server-only. Document working dev/build/typecheck/test commands when initialization makes them real.

This application contains guest queue, dining/order, bill-view, login, and loyalty screens. Staff queue management, kitchen, cashier, and manager functions belong to the separate admin application.

Use Server Components for initial reads and narrow client components for interaction. Client validation is a usability aid; Go remains authoritative. Never share-cache private reads or put sessions in localStorage. The service worker must exclude private data and must not replay offline mutations. Review polling cleanup, fetch waterfalls, payload sizes, and accessibility with every affected feature. Keep this guide and AI instructions outside the runtime project.
