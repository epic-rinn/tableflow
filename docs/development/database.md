# Database changes

PostgreSQL 18 with Goose SQL migrations; application SQL is embedded next to the Go module that uses it. Runtime migrations belong to `src/api/db/migrations/`. No schema is implemented yet.

See [data model](../../specs/architecture/data-model.md). M0 pins Goose and adds local database setup plus commands for migration status, up, validation, and disposable-test reset. Use a migration role separate from the restricted app role.

Typical CLI shape after tooling exists: `goose -dir src/api/db/migrations postgres "$DATABASE_URL" status` and `goose -dir src/api/db/migrations postgres "$DATABASE_URL" up`. Run from repository root against the intended local database. Do not paste connection strings into review artifacts.

Use unique timestamp-prefixed migrations and never edit applied files. Review constraints, existing-data backfills, lock duration, index build mode, and compatibility with both deployment versions. Test fresh and upgrade paths on disposable PostgreSQL. Document recovery for destructive changes; a Down section alone does not prove safe rollback. See [migration authoring](migrations.md).
