#!/usr/bin/env bash
# Recreates the disposable tableflow_e2e database in the compose PostgreSQL,
# migrates it, bootstraps one branch, seeds a dining capability and prints
# E2E_MANAGER_TOKEN=... and E2E_VISIT_TOKEN=... lines.
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
manager_token="$(cd src/api && DATABASE_URL="postgres://tableflow_app:app_dev_only@127.0.0.1:${DB_PORT}/tableflow_e2e?sslmode=disable" \
  go run ./cmd/tableflowctl bootstrap-branch -name "E2E Branch" -email manager@e2e.test -display-name "E2E Manager" |
  sed -n 's/^activation_token=//p')"
echo "E2E_MANAGER_TOKEN=$manager_token"

# A dining capability for a synthetic visit (visits arrive in MVP-06). Only
# the SHA-256 digest is stored, exactly as the API does.
visit_token="$(python3 -c 'import base64,secrets;print(base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip("="))')"
docker compose exec -T postgres psql -q -U tableflow_owner -d tableflow_e2e -v ON_ERROR_STOP=1 -v tok="$visit_token" >/dev/null <<'SQL'
INSERT INTO capabilities (branch_id, kind, resource_id, token_hash)
SELECT id, 'visit', '0198f0c0-0000-7000-8000-0000000000e2', sha256(convert_to(:'tok', 'UTF8')) FROM branches;
SQL
echo "E2E_VISIT_TOKEN=$visit_token"
