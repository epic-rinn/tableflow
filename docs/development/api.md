# Go API development

Runtime location: `src/api/`. Service not initialized. Read [architecture](../../specs/architecture/system.md), [data model](../../specs/architecture/data-model.md), and [HTTP contract](../../specs/api/http.md).

M0 creates the pinned Go module, `cmd/api`, shared HTTP/config/database infrastructure, and integration-test harness. Feature modules live under `internal/<domain>` and contain handlers, use cases, repository code, module-local embedded SQL, and tests when implemented.

Use `pgx/v5` and `pgxpool` with handwritten parameterized SQL and explicit scanning. Database schema lives in `src/api/db/migrations/`. Configuration includes `DATABASE_URL`, `HTTP_ADDR`, allowed admin/PWA origins, session lifetimes, and a secret-encryption key; examples must contain dummy values only.

Document actual Go test/vet/race and local start commands when source exists. Production credentials never belong in repository fixtures.

Enforce branch/resource authorization even on idempotency replay. Use context-aware queries, bounded pools, explicit columns, checked affected-row counts, closed rows, and iteration-error checks. Use cases own transactions and pass them into repositories. Follow the documented lock order; avoid external network calls while holding locks. No process-local business locks or float money. Changed data paths require database/API review and real PostgreSQL concurrency tests.
