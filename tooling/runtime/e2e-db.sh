#!/usr/bin/env bash
# Recreates the disposable tableflow_e2e database in the compose PostgreSQL,
# migrates it, bootstraps one branch and prints the manager activation token.
# Database/grant statements mirror src/api/db/local/init.sql.
set -euo pipefail
cd "$(dirname "$0")/../.."
DB_PORT="${DB_PORT:-54318}"

docker compose exec -T postgres psql -q -U postgres -v ON_ERROR_STOP=1 >/dev/null <<'SQL'
DROP DATABASE IF EXISTS tableflow_e2e WITH (FORCE);
CREATE DATABASE tableflow_e2e OWNER tableflow_owner;
REVOKE CONNECT ON DATABASE tableflow_e2e FROM PUBLIC;
GRANT CONNECT ON DATABASE tableflow_e2e TO tableflow_app;
\connect tableflow_e2e
ALTER SCHEMA public OWNER TO tableflow_owner;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO tableflow_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO tableflow_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO tableflow_app;
SQL

(cd src/api && MIGRATION_DATABASE_URL="postgres://tableflow_owner:owner_dev_only@127.0.0.1:${DB_PORT}/tableflow_e2e?sslmode=disable" \
  go run ./cmd/migrate up >/dev/null)
(cd src/api && DATABASE_URL="postgres://tableflow_app:app_dev_only@127.0.0.1:${DB_PORT}/tableflow_e2e?sslmode=disable" \
  go run ./cmd/tableflowctl bootstrap-branch -name "E2E Branch" -email manager@e2e.test -display-name "E2E Manager") |
  sed -n 's/^activation_token=//p'
