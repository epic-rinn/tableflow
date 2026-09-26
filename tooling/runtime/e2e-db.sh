#!/usr/bin/env bash
# Recreates the disposable tableflow_e2e database in the compose PostgreSQL,
# migrates it, bootstraps one branch, seeds a dining capability, a served
# bill at E2E-2 and prints
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

# A seated party at table E2E-1 with its dining capability. Only the SHA-256
# digest of the token is stored, exactly as the API does.
visit_token="$(python3 -c 'import base64,secrets;print(base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip("="))')"
docker compose exec -T postgres psql -q -U tableflow_owner -d tableflow_e2e -v ON_ERROR_STOP=1 -v tok="$visit_token" >/dev/null <<'SQL'
WITH t AS (
    INSERT INTO dining_tables (branch_id, label, capacity, state)
    SELECT id, 'E2E-1', 4, 'occupied' FROM branches RETURNING id, branch_id
), v AS (
    INSERT INTO visits (branch_id, table_id, party_size) SELECT branch_id, id, 2 FROM t RETURNING id, branch_id, table_id
), c AS (
    INSERT INTO table_claims (table_id, branch_id, visit_id) SELECT table_id, branch_id, id FROM v
)
INSERT INTO capabilities (branch_id, kind, resource_id, token_hash)
SELECT branch_id, 'visit', id, sha256(convert_to(:'tok', 'UTF8')) FROM v;

-- A two-item menu (revision 2).
INSERT INTO menus (branch_id, revision) SELECT id, 2 FROM branches;
WITH c AS (
    INSERT INTO menu_categories (branch_id, name_th, name_en, sort)
    SELECT id, 'เครื่องดื่ม', 'Drinks', 0 FROM branches RETURNING id, branch_id
)
INSERT INTO menu_items (branch_id, category_id, name_th, name_en, price_satang, sort, changed_revision)
SELECT branch_id, id, 'ชาไทย', 'Thai Tea', 6000, 0, 2 FROM c
UNION ALL SELECT branch_id, id, 'กาแฟเย็น', 'Iced Coffee', 7000, 1, 2 FROM c;
-- A party at E2E-2 whose two Thai Teas were already served: ready to settle.
WITH t AS (
    INSERT INTO dining_tables (branch_id, label, capacity, state)
    SELECT id, 'E2E-2', 4, 'occupied' FROM branches RETURNING id, branch_id
), v AS (
    INSERT INTO visits (branch_id, table_id, party_size) SELECT branch_id, id, 2 FROM t RETURNING id, branch_id, table_id
), c AS (
    INSERT INTO table_claims (table_id, branch_id, visit_id) SELECT table_id, branch_id, id FROM v
), o AS (
    INSERT INTO orders (branch_id, visit_id, actor_kind, menu_revision) SELECT branch_id, id, 'guest', 2 FROM v RETURNING id, branch_id, visit_id
)
INSERT INTO order_lines (branch_id, order_id, visit_id, item_id, name_th, name_en, unit_price_satang, quantity, state)
SELECT o.branch_id, o.id, o.visit_id, i.id, i.name_th, i.name_en, i.price_satang, 2, 'served'
FROM o JOIN menu_items i ON i.name_en = 'Thai Tea';
SQL
echo "E2E_VISIT_TOKEN=$visit_token"
echo "E2E_BRANCH_ID=$(docker compose exec -T postgres psql -U tableflow_owner -d tableflow_e2e -Atc 'SELECT id FROM branches')"
