# MVP implementation roadmap

M0–M3 are **done** (MVP-01–13, gates passed 2026-09-26 UTC). MU (UI redesign, UI-01/02) is next, then M4–M6.

The [MVP task backlog](tasks.md) owns implementation-task status and dependencies. Claude implements those tasks; Codex primarily maintains planning and the original Word report. Begin with the prepared [001-foundation packet](../changes/001-foundation/plan.md), which covers only the runtime/testing portion of M0. Identity is delivered by MVP-02–04 before M0 is complete.

| Milestone | Scope / requirements | Dependencies | Exit evidence |
| --- | --- | --- | --- |
| M0 Foundation | Pinned toolchains, separate `src/admin` and `src/pwa` Next.js bootstraps, `src/api` Go bootstrap, local PostgreSQL, Goose, origin routing, health checks, OpenAPI/local verification gate/test harness, staff/member identity (ACC-001–004, ADM-001) | None | Runnable setup, separate frontend builds, real DB tests, auth/origin isolation, contract validation; review |
| M1 Queue and seating | QUE-001–004, SEA-001–004; tables/roles config subset | M0 | Join/call/seat UI and API, claim constraints, competing-host tests, board/poll plans; review |
| M2 Menu and ordering | MEN-001, ORD-001–007; kitchen/assistance and menu config | M1 | Two-device ordering, kitchen transitions, conflict/idempotency tests and query plans; review |
| M3 Cashier settlement | BIL-001–008; charge policy config | M2 | End-to-end nonmember payment, rounding fixtures, confirmation/refund races, plans; review |
| MU UI foundation | UI-001–005: Tailwind + shadcn/ui design system, modern admin panel, Grab-style mobile PWA ([UI design](../product/ui-design.md), [ADR-0006](../decisions/0006-ui-stack.md)) | M3 | Redesigned admin and PWA pass all journeys, axe scans, viewport screenshots, client-JS budget; review |
| M4 Loyalty | LOY-001–007; member UI and loyalty policy | M3 | Claim/discount/earn/refund integration, contention tests; review |
| M5 PWA and operations | PWA-001–005, OPS-001–002, ADM-005–006; finish manager configuration/reporting | M1–M4 | Offline/privacy/browser checks, admin/PWA isolation, reports reconcile, accessibility checks; review |
| M6 Pilot readiness | Entire MVP | M0–M5 | Representative load run, restore drill, operator policies/training, complete journey with real staff |

Guest UI in `src/pwa` and staff UI in `src/admin` are delivered with each vertical slice; from MU onward every new screen uses the design system: ADM-002 with M1/M2, ADM-003 with M2, ADM-004 with M3. M5 is not permission to delay essential offline/error handling or security until the end. M3 can record nonmember settlements first; M4 must integrate loyalty atomically into that transaction before member billing is enabled.

First task: execute MVP-01 using change `001-foundation`, pin versions, add local development services and minimal Go/Next.js health paths. Continue with the dependency-ordered identity tasks after its review gates pass. The entire MVP is too large for one unreviewable change.
