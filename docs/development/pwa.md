# Customer PWA development

Runtime location: `src/pwa/`. Guest screens implemented through M3 and redesigned in UI-02 (Tailwind CSS v4 + shadcn/ui, Grab-style mobile patterns); the service worker is MVP-17. Read [system architecture](../../specs/architecture/system.md) and [PWA requirements](../../specs/features/05-access-pwa.md).

The project has its own `package.json`, pnpm lockfile, TypeScript/Next.js config, `app/`, and Playwright smoke tests (dev port 3000). Add `features/<domain>` for UI behavior, `components/` for shared UI, `lib/api/` for contract-derived transport, and `public/` for static PWA assets only as needed. These paths are relative to `src/pwa/`.

The browser calls same-origin `/api/v1`; Go owns business mutations. Environment variable `API_INTERNAL_URL` is server-only. Implemented screens: entrance join `/join/<branch_id>` (MVP-05), QR entry `/q` with live queue tracking and `/t` with menu, per-phone cart (sessionStorage per visit), shared orders, assistance and the read-only bill (MVP-03/05/06/08–11) and member account pages under `/account` (MVP-04); single-use tokens are read from the URL fragment by `lib/useFragmentToken.ts`. Commands: [setup](setup.md) (`make pwa-check`, `make smoke`). `proxy.ts` forwards `/api/v1/*` to `API_INTERNAL_URL` at request time.

This application contains guest queue, dining/order, bill-view, login, and loyalty screens. Staff queue management, kitchen, cashier, and manager functions belong to the separate admin application.

Use Server Components for initial reads and narrow client components for interaction. Client validation is a usability aid; Go remains authoritative. Never share-cache private reads or put sessions in localStorage. The service worker must exclude private data and must not replay offline mutations. Review polling cleanup, fetch waterfalls, payload sizes, and accessibility with every affected feature. Keep this guide and AI instructions outside the runtime project.

## UI conventions (from UI-02)

Stack and visual language: [ADR-0006](../../specs/decisions/0006-ui-stack.md) and [UI design](../../specs/product/ui-design.md).

- **Stack:** Tailwind CSS v4 plus shadcn/ui in `components/ui/`, `lucide-react` icons, and self-hosted Inter and Noto Sans Thai.
- **Layout:**
  - mobile-first at 360–430 px, content capped at about 480 px;
  - safe-area padding;
  - touch targets of at least 44 px.
- **Patterns:**
  - hero header with a status pill;
  - sticky category chips;
  - item options in a bottom `Sheet`;
  - a sticky cart bar opening the cart sheet;
  - order timeline with status chips;
  - receipt-style bill (never a pay button);
  - a "Need something?" help card with quick actions.
- **Brand:** the look is Grab-like in patterns only. Use TableFlow tokens; never Grab's name, logo, colours or illustrations.
- **Client JavaScript:** keep first-load JS within the budget in [performance](../../specs/quality/performance.md). Prefer Server Components for static parts and keep client components narrow. Do not import heavy components into the entry routes without measuring.
- **Tests:** keep accessible names stable (tests use role, label and name). PWA journeys run at 390×844.

