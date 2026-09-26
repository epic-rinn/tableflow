# ADR-0006: Tailwind CSS and shadcn/ui for both frontends

Date: 2026-09-27. Status: accepted (owner decision).

## Context

M0–M3 delivered working admin and PWA screens with minimal global CSS. The owner wants a modern look before more screens are added:
- **Admin:** an admin-panel style.
- **PWA:** a mobile super-app style, similar in feel to the Grab app.

M4 adds member screens. Building them on the final design system avoids restyling work later. The existing browser tests locate elements by role and accessible name, so a redesign can be checked against them without changing behavior.

## Decision

- Use **Tailwind CSS v4** and **shadcn/ui** in both `src/admin` and `src/pwa`:
  - shadcn/ui is copied component source built on Radix UI primitives, with `class-variance-authority`, `tailwind-merge` and `lucide-react` icons;
  - each app owns its copy under `components/ui/`, and there is no shared runtime package (ADR-0002 keeps the projects independent);
  - exact versions are pinned in each lockfile when the first UI task lands.
- **Styling:** design tokens are CSS variables (shadcn theming), with one token set per app. The admin supports light and dark themes; the PWA is light-first.
- **Fonts:**
  - Inter (Latin) and Noto Sans Thai are self-hosted through npm font packages, so there are no runtime requests to third-party font hosts;
  - the build needs no network;
  - Thai text uses a comfortable line height.
- **Where the rules live:** the visual language and UI requirements are in [UI design](../product/ui-design.md). Presentation changes do not move business rules out of Go.

## Alternatives

- **Plain CSS modules:** zero dependencies, but slower to reach a consistent, accessible component set (dialogs, sheets, menus, toasts).
- **MUI / Chakra / Mantine:** complete kits, but heavier runtime styling and harder to match the requested look. shadcn keeps the source local and editable.
- **A shared `src/ui` package:** less duplication, but it couples the two independently deployed apps and their lockfiles. Revisit this if the duplicated primitives drift.

## Consequences

- **Cost:** client JavaScript grows by the Radix primitives in use. The PWA must stay within the [performance](../quality/performance.md) client budgets, and heavy components load only where needed.
- **Accessibility:** Radix supplies focus management and ARIA. Custom compositions still need keyboard and screen-reader checks. Every control keeps a stable accessible name, because the tests depend on names.
- **Resemblance to Grab:** "Grab-like" means interaction patterns only. Grab's name, logo, illustrations, exact brand colours or screenshots must never be copied.
- **Dependency upkeep:** Tailwind and shadcn updates follow normal dependency review; generated components are ordinary source subject to code review.
