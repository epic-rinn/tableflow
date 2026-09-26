# ADR-0001: Stack and persistence

Date: 2026-09-26. Status: accepted for MVP.

## Context

The owner selected Next.js PWA, Go, PostgreSQL, handwritten SQL, and no ORM. Queueing and payment require consistent shared state and modest operational overhead.

## Decision

Use one Go modular monolith with `net/http`, `pgx/v5` and `pgxpool`, plus Next.js App Router/TypeScript. PostgreSQL is the only state store. Choose **Goose** with SQL-only migrations because it provides an ordered migration history and transaction-aware SQL changes without introducing model mapping. See [Goose documentation](https://github.com/pressly/goose) and [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool).

Co-locate handwritten SQL with the Go module using it. Use explicit row scanning; no SQL generation in the initial scope. Migration placement and the separate admin/PWA application boundaries are refined by [ADR-0002](0002-runtime-boundaries.md): migrations live in `src/api/db/migrations/`.

## Alternatives and consequences

- `golang-migrate` would also satisfy SQL migration requirements; standardize on one tool to avoid split histories.
- sqlc is not an ORM, but generated query bindings add a build step; defer until repetitive scanning justifies it.
- Microservices would make seating/payment transactions and deployment harder without demonstrated need.
- Polling is simpler for the initial load, but must be measured with all open clients included.
- Explicit SQL gives plan visibility and control; developers must maintain scanning, constraints, indexes, and migration compatibility deliberately.

Revisit this ADR for measured scaling limits or integration needs, not merely to add familiar infrastructure.
