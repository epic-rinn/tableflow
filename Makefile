# Development and verification commands. Local credentials below are the dummy
# values created by compose.yaml for disposable databases only.
DB_PORT ?= 54318
TEST_DATABASE_URL ?= postgres://postgres:postgres_dev_only@127.0.0.1:$(DB_PORT)/postgres?sslmode=disable
MIGRATION_DATABASE_URL ?= postgres://tableflow_owner:owner_dev_only@127.0.0.1:$(DB_PORT)/tableflow?sslmode=disable
DATABASE_URL ?= postgres://tableflow_app:app_dev_only@127.0.0.1:$(DB_PORT)/tableflow?sslmode=disable
API_INTERNAL_URL ?= http://127.0.0.1:8080
PNPM ?= corepack pnpm
FRONTENDS := admin pwa

.PHONY: specs-check services-up services-down services-reset \
	migrate-up migrate-down migrate-status api-run api-check api-test-db api-build \
	admin-check pwa-check frontends-check smoke artifact-check check verify

specs-check:
	python3 tooling/specs/check.py

services-up:
	docker compose up -d --wait

services-down:
	docker compose down

# Destroys local PostgreSQL data.
services-reset:
	docker compose down --volumes

migrate-up migrate-down migrate-status:
	cd src/api && MIGRATION_DATABASE_URL='$(MIGRATION_DATABASE_URL)' go run ./cmd/migrate $(subst migrate-,,$@)

api-run:
	cd src/api && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/api

api-check:
	cd src/api && test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt required"; exit 1; }
	cd src/api && go vet ./...
	cd src/api && go test -race -count=1 ./...

api-test-db:
	cd src/api && TEST_DATABASE_URL='$(TEST_DATABASE_URL)' TABLEFLOW_REQUIRE_DB=1 go test -race -count=1 ./...

api-build:
	mkdir -p tmp
	cd src/api && go build -trimpath -o ../../tmp/tableflow-api ./cmd/api

$(FRONTENDS:%=%-check):
	cd src/$(@:-check=) && $(PNPM) install --frozen-lockfile
	cd src/$(@:-check=) && $(PNPM) lint
	cd src/$(@:-check=) && $(PNPM) typecheck
	cd src/$(@:-check=) && NEXT_TELEMETRY_DISABLED=1 $(PNPM) build

frontends-check: admin-check pwa-check

# Requires built frontends and a running API (make services-up migrate-up api-run).
smoke:
	cd src/admin && API_INTERNAL_URL='$(API_INTERNAL_URL)' $(PNPM) test:smoke
	cd src/pwa && API_INTERNAL_URL='$(API_INTERNAL_URL)' $(PNPM) test:smoke

artifact-check: api-build
	python3 tooling/runtime/check_artifacts.py \
		src/admin/.next/standalone src/admin/.next/static \
		src/pwa/.next/standalone src/pwa/.next/static tmp/tableflow-api

check: specs-check api-check

# Full local gate: services, Go, frontends, browser smoke, artifacts.
verify:
	tooling/runtime/verify.sh
