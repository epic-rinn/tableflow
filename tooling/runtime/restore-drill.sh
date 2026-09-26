#!/usr/bin/env bash
# Backup/restore drill (MVP-21). Dumps SOURCE_DB (default tableflow_e2e,
# populated by `make verify`) from the local compose PostgreSQL with
# pg_dump -Fc, restores it into a new disposable database, reconciles bills,
# settlements, refunds, member claims and the loyalty ledger between source
# and restore, checks migrations are current, then drops the restore.
# Usage: tooling/runtime/restore-drill.sh [evidence.json]
set -euo pipefail
cd "$(dirname "$0")/../.."
SOURCE_DB=${SOURCE_DB:-tableflow_e2e}
TARGET_DB=tableflow_restore_drill
OUT=${1:-}
DUMP=$(mktemp -t tableflow-drill.XXXXXX)
trap 'rm -f "$DUMP"; docker compose exec -T postgres psql -q -U postgres -c "DROP DATABASE IF EXISTS $TARGET_DB WITH (FORCE)" >/dev/null 2>&1 || true' EXIT
pg() { docker compose exec -T postgres "$@"; }

start=$(date +%s)
pg pg_dump -Fc -U tableflow_owner -d "$SOURCE_DB" > "$DUMP"
dump_bytes=$(wc -c < "$DUMP" | tr -d ' ')
pg psql -q -U postgres -v ON_ERROR_STOP=1 >/dev/null <<SQL
DROP DATABASE IF EXISTS $TARGET_DB WITH (FORCE);
CREATE DATABASE $TARGET_DB OWNER tableflow_owner;
REVOKE CONNECT ON DATABASE $TARGET_DB FROM PUBLIC;
GRANT CONNECT ON DATABASE $TARGET_DB TO tableflow_app;
SQL
pg pg_restore -U tableflow_owner -d "$TARGET_DB" --no-owner --role=tableflow_owner --exit-on-error < "$DUMP"
restore_s=$(( $(date +%s) - start ))

# Business invariants compared between source and restore.
read -r -d '' CHECKS <<'SQL' || true
SELECT json_build_object(
  'settlements', (SELECT count(*) FROM settlements),
  'settled_satang', (SELECT coalesce(sum(amount_satang), 0) FROM settlements),
  'snapshot_totals', (SELECT coalesce(sum(total_satang), 0) FROM bill_snapshots),
  'refunds', (SELECT count(*) FROM refunds),
  'refunded_satang', (SELECT coalesce(sum(amount_satang), 0) FROM refunds),
  'claimed_visits', (SELECT count(*) FROM visits WHERE member_id IS NOT NULL),
  'ledger_rows', (SELECT count(*) FROM loyalty_ledger),
  'ledger_points', (SELECT coalesce(sum(points_delta), 0) FROM loyalty_ledger),
  'profiles_match_ledger', (SELECT bool_and(p.points_balance = coalesce(l.p, 0) AND p.qualifying_spend_satang = coalesce(l.q, 0))
                            FROM member_profiles p LEFT JOIN (SELECT branch_id, member_id, sum(points_delta) p, sum(qualifying_delta_satang) q
                            FROM loyalty_ledger GROUP BY 1, 2) l USING (branch_id, member_id)),
  'order_lines', (SELECT count(*) FROM order_lines),
  'audit_events', (SELECT count(*) FROM audit_events),
  'staff_accounts', (SELECT count(*) FROM staff_accounts),
  'members', (SELECT count(*) FROM member_accounts),
  'migration_version', (SELECT max(version_id) FROM goose_db_version)
)
SQL
src=$(pg psql -U tableflow_owner -d "$SOURCE_DB" -At -c "$CHECKS")
dst=$(pg psql -U tableflow_owner -d "$TARGET_DB" -At -c "$CHECKS")
# The app role must work against the restore (grants survive).
app_ok=$(pg psql -U tableflow_app -d "$TARGET_DB" -At -c "SELECT count(*) >= 0 FROM settlements")
latest=$(ls src/api/db/migrations/*.sql | sed -E 's#.*/([0-9]+)_.*#\1#' | sort | tail -1)
python3 - "$src" "$dst" "$app_ok" "$latest" "$dump_bytes" "$restore_s" "$OUT" <<'PY'
import json, sys
src, dst = json.loads(sys.argv[1]), json.loads(sys.argv[2])
app_ok, latest, size, secs, out = sys.argv[3], int(sys.argv[4]), int(sys.argv[5]), int(sys.argv[6]), sys.argv[7]
ok = src == dst and dst["profiles_match_ledger"] in (True, None) and app_ok == "t" and dst["migration_version"] == latest and dst["settlements"] > 0
result = {"ok": ok, "source": src, "restored": dst, "app_role_can_read": app_ok == "t",
          "migrations_current": dst["migration_version"] == latest, "dump_bytes": size, "dump_and_restore_seconds": secs}
text = json.dumps(result, indent=1)
print(text)
if out:
    open(out, "w").write(text + "\n")
sys.exit(0 if ok else 1)
PY
