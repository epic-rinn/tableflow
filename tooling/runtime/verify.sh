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
while IFS='=' read -r name value; do
  [[ "$name" =~ ^E2E_[A-Z_]+$ && ( "$value" =~ ^[A-Za-z0-9_-]{43}$ || "$value" =~ ^[0-9a-f-]{36}$ ) ]] || { echo "unexpected e2e-db output" >&2; exit 1; }
  export "$name=$value"
done < <(tooling/runtime/e2e-db.sh)
[[ -n "${E2E_MANAGER_TOKEN:-}" && -n "${E2E_VISIT_TOKEN:-}" && -n "${E2E_BRANCH_ID:-}" ]] || { echo "E2E setup produced no tokens" >&2; exit 1; }
make api-build
# Automated tests read emails from a file outbox instead of a real inbox.
E2E_MAIL_DIR="$PWD/$LOG_DIR/mail"
rm -rf "$E2E_MAIL_DIR" && mkdir -p "$E2E_MAIL_DIR"
export E2E_MAIL_DIR
MAIL_ADAPTER=file MAIL_OUTBOX_DIR="$E2E_MAIL_DIR" \
DATABASE_URL="postgres://tableflow_app:app_dev_only@127.0.0.1:${DB_PORT}/tableflow_e2e?sslmode=disable" \
  ADMIN_ORIGINS="http://127.0.0.1:3001" PWA_ORIGINS="http://127.0.0.1:3000" PWA_PUBLIC_URL="http://127.0.0.1:3000" \
  DATA_ENCRYPTION_KEY="$(make -s -f Makefile print-data-key)" \
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
