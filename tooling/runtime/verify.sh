#!/usr/bin/env bash
# Local verification gate (replaces hosted CI; see ADR-0003).
# Runs every runtime check against the compose PostgreSQL, starts the built API
# for browser smoke tests, and always stops it again. Run from the repo root:
#   make verify
set -euo pipefail

cd "$(dirname "$0")/../.."
DB_PORT="${DB_PORT:-54318}"
API_ADDR="${VERIFY_API_ADDR:-127.0.0.1:8080}"
LOG_DIR=tmp/verify
mkdir -p "$LOG_DIR"

step() { printf '\n==> %s\n' "$*"; }

node_major="$(node -p 'process.versions.node.split(".")[0]' 2>/dev/null || echo none)"
if [[ "$node_major" != "24" ]]; then
  echo "Node.js 24 is required (found: $node_major); see docs/development/setup.md" >&2
  exit 1
fi

step "Environment"
date -u +%FT%TZ
(cd src/api && go version)  # module toolchain (go.mod)
echo "node $(node --version), pnpm $(corepack pnpm --version), $(docker --version)"

step "Knowledge base"
make specs-check

step "Local services"
make services-up
make migrate-up

step "Go API"
make api-check
make api-test-db

step "Frontends"
make frontends-check

step "Browser tests (disposable tableflow_e2e database, API on $API_ADDR)"
E2E_MANAGER_TOKEN="$(tooling/runtime/e2e-db.sh)"
[[ -n "$E2E_MANAGER_TOKEN" ]] || { echo "E2E bootstrap produced no activation token" >&2; exit 1; }
export E2E_MANAGER_TOKEN
make api-build
DATABASE_URL="postgres://tableflow_app:app_dev_only@127.0.0.1:${DB_PORT}/tableflow_e2e?sslmode=disable" \
  ADMIN_ORIGINS="http://127.0.0.1:3001" \
  HTTP_ADDR="$API_ADDR" ./tmp/tableflow-api >"$LOG_DIR/api.log" 2>&1 &
api_pid=$!
trap 'kill -TERM "$api_pid" 2>/dev/null || true; wait "$api_pid" 2>/dev/null || true' EXIT
for _ in $(seq 1 30); do
  if curl -sf "http://$API_ADDR/api/v1/health/ready" >/dev/null; then break; fi
  sleep 1
done
curl -sf "http://$API_ADDR/api/v1/health/ready" >/dev/null || { cat "$LOG_DIR/api.log"; exit 1; }
make smoke API_INTERNAL_URL="http://$API_ADDR"

step "Artifact boundary"
make artifact-check

step "All local verification passed"
