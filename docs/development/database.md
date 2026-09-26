# Database changes

PostgreSQL 18 with Goose SQL migrations; application SQL is embedded next to the Go module that uses it. Runtime migrations belong to `src/api/db/migrations/`. Only the baseline migration (PostgreSQL 18 version guard) exists; no domain schema is implemented yet.

See [data model](../../specs/architecture/data-model.md). Goose v3.28.0 is pinned as a library in `src/api/go.mod`; `src/api/cmd/migrate` is a PostgreSQL-only wrapper (the upstream CLI would add every Goose driver to the module graph). Migrations run as `tableflow_owner`; the API connects as `tableflow_app`, which has DML but no DDL rights (local roles: `src/api/db/local/init.sql`).

Commands: `make migrate-status`, `make migrate-up`, `make migrate-down` (one step) with `MIGRATION_DATABASE_URL`. Fresh apply, full reversal and re-apply are tested by `TestFreshMigrationAndDatabaseSmoke` on a disposable database (`make api-test-db`); upgrade-path tests start when a second migration exists. Reset local data with `make services-reset`. Do not paste connection strings into review artifacts.

Use unique timestamp-prefixed migrations and never edit applied files. Review constraints, existing-data backfills, lock duration, index build mode, and compatibility with both deployment versions. Test fresh and upgrade paths on disposable PostgreSQL. Document recovery for destructive changes; a Down section alone does not prove safe rollback. See [migration authoring](migrations.md).
