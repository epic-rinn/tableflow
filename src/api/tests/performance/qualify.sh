#!/usr/bin/env bash
# MVP-20 performance qualification under the canonical profile
# (specs/quality/performance.md): Go API limited to 2 vCPU/1 GiB and
# PostgreSQL 18 at 2 vCPU/2 GiB in local containers, the full synthetic
# fixture built from the seed scripts, 2 min warm-up and 10 min measured
# (override with WARMUP_S/MEASURE_S). Writes evidence to $OUT and always
# removes its containers. Disposable data only; never point at a real DB.
set -euo pipefail
cd "$(dirname "$0")/../../../.."
ROOT=$PWD
PERF=src/api/tests/performance
OUT=${OUT:-specs/changes/020-performance/evidence}
WARM=${WARMUP_S:-120}
DUR=${MEASURE_S:-600}
NET=tableflow-perf DB=tableflow-perf-db API=tableflow-perf-api IMG=postgres:18.6
DBPORT=54320 APIPORT=8098
TMPD=
mkdir -p "$OUT"
step() { printf '\n==> %s\n' "$*"; }
cleanup() {
  [[ -n "${STATS_PID:-}" ]] && kill "$STATS_PID" 2>/dev/null || true
  [[ -n "${LOCKS_PID:-}" ]] && kill "$LOCKS_PID" 2>/dev/null || true
  docker rm -fv "$API" "$DB" >/dev/null 2>&1 || true  # -v: drop the anonymous data volumes too
  docker network rm "$NET" >/dev/null 2>&1 || true
  [[ -n "$TMPD" ]] && rm -rf "$TMPD"
  return 0
}
trap cleanup EXIT
cleanup
TMPD=$(mktemp -d)
psqlq() { docker exec -i "$DB" psql -q -U tableflow_owner -d tableflow_perf -v ON_ERROR_STOP=1 "$@"; }
psqla() { docker exec -i "$DB" psql -U tableflow_owner -d tableflow_perf -At "$@"; }

step "Containers (PostgreSQL 2 vCPU/2 GiB)"
docker network create "$NET" >/dev/null
docker run -d --name "$DB" --network "$NET" --cpus=2 --memory=2g -e POSTGRES_PASSWORD=perf_only \
  -p "127.0.0.1:$DBPORT:5432" "$IMG" -c shared_buffers=512MB -c effective_cache_size=1536MB \
  -c log_lock_waits=on -c max_connections=100 >/dev/null
until docker exec "$DB" pg_isready -U postgres -q; do sleep 1; done
sleep 2
sed -e 's/CREATE DATABASE tableflow OWNER/CREATE DATABASE tableflow_perf OWNER/' \
    -e 's/ON DATABASE tableflow FROM/ON DATABASE tableflow_perf FROM/' -e 's/ON DATABASE tableflow TO/ON DATABASE tableflow_perf TO/' \
    -e 's/^\\connect tableflow$/\\connect tableflow_perf/' src/api/db/local/init.sql | docker exec -i "$DB" psql -q -U postgres -v ON_ERROR_STOP=1 >/dev/null

step "Migrations and fixture (this takes several minutes)"
(cd src/api && MIGRATION_DATABASE_URL="postgres://tableflow_owner:owner_dev_only@127.0.0.1:$DBPORT/tableflow_perf?sslmode=disable" go run ./cmd/migrate up >/dev/null)
fixture_start=$(date +%s)
for seed in staff_access guest_member seating ordering billing loyalty reporting; do
  printf '  %s…\n' "$seed"
  psqlq < "$PERF/${seed}_seed.sql" 2>&1 | grep -v 'permission denied to analyze' || true
  if [[ $seed == seating ]]; then
    # Later seeds attribute policies/audit to a staff member of the branch.
    psqlq -c "INSERT INTO staff_accounts (id, branch_id, email, display_name, password_hash, status)
      VALUES ('0198f0c0-0000-7000-8000-00000000f0aa', '00000000-0000-7000-8000-0000000000c1', 'qualify@perf.test', 'Qualify', 'argon2id\$perf', 'active')"
  fi
done
psqlq -c 'ANALYZE' 2>&1 | grep -v 'permission denied' || true
echo "fixture built in $(( $(date +%s) - fixture_start ))s"
psqla <<'SQL' > "$OUT/fixture-counts.txt"
SELECT 'tables', count(*) FROM dining_tables UNION ALL
SELECT 'waiting parties', count(*) FROM queue_tickets WHERE state = 'waiting' UNION ALL
SELECT 'open visits', count(*) FROM visits WHERE state = 'open' UNION ALL
SELECT 'historical visits', count(*) FROM visits WHERE state IN ('departed', 'closed') UNION ALL
SELECT 'menu items', count(*) FROM menu_items UNION ALL
SELECT 'order lines', count(*) FROM order_lines UNION ALL
SELECT 'members', count(*) FROM member_accounts UNION ALL
SELECT 'settlements', count(*) FROM settlements UNION ALL
SELECT 'loyalty ledger', count(*) FROM loyalty_ledger UNION ALL
SELECT 'audit events', count(*) FROM audit_events;
SQL

step "Load identities"
python3 - "$TMPD/tokens.tsv" <<'PY'
import base64, secrets, sys
t = lambda: base64.urlsafe_b64encode(secrets.token_bytes(32)).decode().rstrip("=")
with open(sys.argv[1], "w") as f:
    f.write(f"staff\t0\t{t()}\n")
    for kind, n in (("queue", 150), ("diner", 150), ("member", 50)):
        for i in range(1, n + 1):
            f.write(f"{kind}\t{i}\t{t()}\n")
PY
{ echo "CREATE TEMP TABLE qt (kind text, n int, tok text);"; echo "COPY qt FROM STDIN;"; cat "$TMPD/tokens.tsv"; echo '\.'; cat "$PERF/qualify_sessions.sql"; } | psqlq >/dev/null
psqla -F $'\t' -c "SELECT encode(g.token_hash, 'hex'), c.kind, c.resource_id FROM guest_sessions g JOIN capabilities c ON c.id = g.capability_id WHERE g.expires_at > now() + interval '10 hours'" > "$TMPD/sessions.tsv"
psqla -F $'\t' -c "SELECT id, CASE WHEN member_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM order_lines l WHERE l.visit_id = v.id AND l.state NOT IN ('served','rejected','cancelled')) THEN 'settle' ELSE 'order' END FROM visits v WHERE state = 'open'" > "$TMPD/visits.tsv"
psqla -F $'\t' -c "SELECT i.id, o.id FROM menu_items i JOIN option_groups g ON g.item_id = i.id AND g.min_choices = 1 JOIN menu_options o ON o.group_id = g.id AND o.sort = 1 WHERE i.branch_id = '00000000-0000-7000-8000-0000000000c1' AND i.retired_at IS NULL LIMIT 50" > "$TMPD/items.tsv"
psqla -c "SELECT id FROM menu_categories WHERE branch_id = '00000000-0000-7000-8000-0000000000c1'" > "$TMPD/categories.txt"
REVISION=$(psqla -c "SELECT revision FROM menus WHERE branch_id = '00000000-0000-7000-8000-0000000000c1'")
python3 - "$TMPD" "$REVISION" > "$TMPD/ids.json" <<'PY'
import hashlib, json, sys, datetime
d, revision = sys.argv[1], int(sys.argv[2])
toks = [l.rstrip("\n").split("\t") for l in open(f"{d}/tokens.tsv")]
sess = {h: (k, r) for h, k, r in (l.rstrip("\n").split("\t") for l in open(f"{d}/sessions.tsv"))}
def res(tok):
    return sess.get(hashlib.sha256(tok.encode()).hexdigest())
visits = [l.rstrip("\n").split("\t") for l in open(f"{d}/visits.tsv")]
today = datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=7))).date()
out = {
    "branch": "00000000-0000-7000-8000-0000000000c1",
    "staff": [t for k, _, t in toks if k == "staff"][0],
    "trackers": [(t, res(t)[1]) for k, _, t in toks if k == "queue" and res(t)],
    "diners": [(t, res(t)[1]) for k, _, t in toks if k == "diner" and res(t)],
    "members": [t for k, _, t in toks if k == "member"],
    "open_visits": [v for v, _ in visits],
    "settle_visits": [v for v, kind in visits if kind == "settle"],
    "order_visits": [v for v, kind in visits if kind == "order"],
    "items": [l.rstrip("\n").split("\t") for l in open(f"{d}/items.tsv")],
    "categories": [l.strip() for l in open(f"{d}/categories.txt") if l.strip()],
    "menu_revision": revision,
    "report_from": str(today - datetime.timedelta(days=30)), "report_to": str(today),
}
json.dump(out, sys.stdout)
PY
python3 -c "import json,sys; d=json.load(open('$TMPD/ids.json')); print({k: len(v) if isinstance(v, list) else v for k, v in d.items() if k != 'staff'})"

step "API container (Go 2 vCPU/1 GiB)"
(cd src/api && CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" go build -trimpath -o "$ROOT/tmp/tableflow-api-linux" ./cmd/api)
KEY=$(make -s print-data-key)
docker run -d --name "$API" --network "$NET" --cpus=2 --memory=1g -p "127.0.0.1:$APIPORT:8080" \
  -v "$ROOT/tmp/tableflow-api-linux:/tableflow-api:ro" --entrypoint /tableflow-api \
  -e HTTP_ADDR=0.0.0.0:8080 -e "DATABASE_URL=postgres://tableflow_app:app_dev_only@$DB:5432/tableflow_perf?sslmode=disable" \
  -e "DATA_ENCRYPTION_KEY=$KEY" -e ADMIN_ORIGINS=http://perf.test -e PWA_ORIGINS=http://pwa.perf.test \
  -e PWA_PUBLIC_URL=http://pwa.perf.test -e MAIL_ADAPTER=file -e MAIL_OUTBOX_DIR=/tmp -e DB_POOL_STATS_INTERVAL=10s "$IMG" >/dev/null
until curl -sf "http://127.0.0.1:$APIPORT/api/v1/health/ready" >/dev/null; do sleep 1; done

step "Environment"
{
  echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "host: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || uname -m), $(sysctl -n hw.ncpu 2>/dev/null) CPUs, $(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1073741824 )) GiB, $(uname -sr)"
  echo "docker: $(docker version --format '{{.Server.Version}} ({{.Server.Os}}/{{.Server.Arch}})')"
  echo "postgres: $(psqla -c 'SHOW server_version'); limits --cpus=2 --memory=2g; shared_buffers=512MB effective_cache_size=1536MB max_connections=100"
  echo "api: $(cd src/api && go version | cut -d' ' -f3) linux/$(cd src/api && go env GOARCH) static binary in $IMG; limits --cpus=2 --memory=1g; DB_MAX_CONNS=10 (default)"
  echo "load generator: python3 $(python3 -c 'import sys; print(sys.version.split()[0])') threads on the host (outside both limits)"
  echo "warmup_s: $WARM measure_s: $DUR"
} > "$OUT/environment.txt"
cat "$OUT/environment.txt"
psqla -c "SELECT sum(xact_commit), sum(deadlocks), sum(blks_hit), sum(blks_read), sum(temp_files) FROM pg_stat_database" > "$TMPD/pgstat-before.txt"

step "Load: ${WARM}s warm-up + ${DUR}s measured"
( while true; do docker stats --no-stream --format '{{json .}}' "$API" "$DB"; sleep 5; done ) > "$OUT/container-stats.jsonl" 2>/dev/null &
STATS_PID=$!
( while true; do
    printf '%s %s\n' "$(date +%s)" "$(docker exec "$DB" psql -U postgres -d tableflow_perf -At -c "SELECT count(*) FILTER (WHERE wait_event_type = 'Lock'), count(*) FILTER (WHERE state = 'active'), count(*) FROM pg_stat_activity WHERE datname = 'tableflow_perf'" 2>/dev/null)"
    sleep 1
  done ) > "$TMPD/locks.txt" &
LOCKS_PID=$!
python3 "$PERF/qualify_load.py" "http://127.0.0.1:$APIPORT" "$TMPD/ids.json" "$WARM" "$DUR" | tee "$OUT/load.jsonl"
kill "$STATS_PID" "$LOCKS_PID" 2>/dev/null || true
STATS_PID= LOCKS_PID=

step "Collect"
psqla -c "SELECT sum(xact_commit), sum(deadlocks), sum(blks_hit), sum(blks_read), sum(temp_files) FROM pg_stat_database" > "$TMPD/pgstat-after.txt"
python3 - "$TMPD" "$OUT" <<'PY'
import sys, json
d, out = sys.argv[1], sys.argv[2]
rows = [l.split() for l in open(f"{d}/locks.txt") if len(l.split()) == 2]
vals = [tuple(int(x) for x in r[1].split("|")) for r in rows if r[1].count("|") == 2]
b = [int(x) for x in open(f"{d}/pgstat-before.txt").read().strip().split("|")]
a = [int(x) for x in open(f"{d}/pgstat-after.txt").read().strip().split("|")]
names = ["commits", "deadlocks", "blocks_hit", "blocks_read", "temp_files"]
delta = {n: a[i] - b[i] for i, n in enumerate(names)}
delta["cache_hit_ratio"] = round(delta["blocks_hit"] / max(1, delta["blocks_hit"] + delta["blocks_read"]), 5)
summary = {"lock_samples": len(vals), "samples_with_lock_waits": sum(1 for v in vals if v[0] > 0),
           "max_lock_waiters": max((v[0] for v in vals), default=0), "max_active_backends": max((v[1] for v in vals), default=0),
           "max_connections_open": max((v[2] for v in vals), default=0), "pg_stat_database_delta": delta}
json.dump(summary, open(f"{out}/database.json", "w"), indent=1)
print(json.dumps(summary))
PY
docker logs "$API" 2>&1 | grep '"db pool"\|db pool' > "$OUT/pool-stats.log" || true
echo "api error log lines: $(docker logs "$API" 2>&1 | grep -c 'level=ERROR' || true)" | tee "$OUT/api-errors.txt"
docker logs "$DB" 2>&1 | grep -iE 'deadlock|still waiting for|lock' > "$OUT/postgres-lock-log.txt" || true
echo "lock-wait log lines: $(wc -l < "$OUT/postgres-lock-log.txt")"
echo "done: evidence in $OUT"
