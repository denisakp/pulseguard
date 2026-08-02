.DEFAULT_GOAL := help
SHELL := /bin/sh

# --- Variables ---
BINARY := dist/ogoune
SQLC_VERSION := v1.30.0

# --- Versioning ---
VERSION           := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT            := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS           := -s -w \
                     -X main.version=$(VERSION) \
                     -X main.commit=$(COMMIT) \
                     -X main.buildDate=$(BUILD_DATE)

# --- Go flags ---
GO                := go
GOFLAGS           := -trimpath
GO_TEST_FLAGS     := -race -count=1
GO_LINT_TIMEOUT   := 5m

.PHONY: build build-be build-fe test test-be test-be-pg test-be-bench bench-api test-fe type-check-fe lint clean docker run-ci ci-local license-audit sqlc-bin sqlc-generate sqlc-check migrations-drift-check fuzz-dynquery

build: build-fe build-be

build-be: sqlc-check
	mkdir -p dist
	go build -o $(BINARY) ./cmd/api/main.go

# Host monitoring agent binary (spec 080). Version is stamped from git.
build-agent:
	mkdir -p dist
	go build -ldflags "-X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o dist/ogoune-agent ./cmd/agent

# Cross-compile the agent for Linux (spec 082 — release binaries). Default arm64;
# override with ARCH=amd64. Static, version stamped from git.
AGENT_VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
ARCH ?= arm64
build-agent-linux:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build \
		-ldflags "-w -s -X main.version=$(AGENT_VERSION)" \
		-o dist/ogoune-agent-linux-$(ARCH) ./cmd/agent

SQLC_BIN := $(shell go env GOPATH)/bin/sqlc

sqlc-bin:
	@if [ ! -x "$(SQLC_BIN)" ] || [ "$$($(SQLC_BIN) version)" != "$(SQLC_VERSION:v%=%)" ]; then \
		go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION); \
	fi

sqlc-generate: sqlc-bin
	$(SQLC_BIN) generate -f sqlc.yaml

sqlc-check: sqlc-bin
	@$(SQLC_BIN) generate -f sqlc.yaml
	@drift=$$(git status --porcelain -- internal/repository/sqlc/pg internal/repository/sqlc/sqlite \
		| grep -Ev '^A  |_test\.go$$' || true); \
	if [ -n "$$drift" ]; then \
		echo "sqlc drift: run 'make sqlc-generate' and commit the result"; \
		printf '%s\n' "$$drift"; \
		exit 1; \
	fi

migrations-drift-check:
	go run ./cmd/migrations-drift-check

build-fe:
	cd web && pnpm build

test: test-be test-fe

test-be:
	go test -race ./...

# Run the backend tests with Postgres enabled. Provisioning is owned by
# testcontainers-go inside internal/repository/internaltest; the helper
# boots postgres:16-alpine on first use and tears it down at process exit.
# Skips gracefully when Docker is not reachable.
test-be-pg:
	@if ! docker info >/dev/null 2>&1; then \
		echo "Docker not available — skipping Postgres tests"; \
		exit 0; \
	fi
	go test -race -timeout 300s ./internal/repository/store/... ./internal/repository/internaltest/...

# Paired benches (spec 049): GORM vs sqlc p95 ratio gates on Resource.List
# and Incident.GetIncidentStats. Runs WITHOUT -race (race detector inflates
# p95 ~10× and drowns the signal). SQLite only — paired ratio is dialect-
# invariant; adding the Postgres testcontainer would double bench time
# without changing the signal.
#
# Output: each bench emits a structured `paired_bench …` line for CI capture.
# Gate: default warn-and-pass; set PAIRED_BENCH_STRICT=true to escalate
# ratio > 1.10 to a hard failure.
test-be-bench:
	go test -bench=Paired -benchtime=1x -run=^$$ -count=1 \
	  ./internal/repository/store/... | tee bench-output.txt

# Spec 052 — API hot-path p95 bench for SC-005/SC-006 regression check.
# Outputs `api_bench name=… p50_us=… p95_us=… p99_us=…` lines.
bench-api:
	go test -bench=^BenchmarkAPI_ -benchtime=1x -run=^$$ -count=3 \
	  ./internal/api/handler/v1/... | tee api-bench-output.txt

test-fe:
	cd web && pnpm test

type-check-fe:
	cd web && pnpm type-check

lint:
	go vet ./...
	cd web && pnpm lint

clean:
	rm -rf dist
	rm -rf web/dist

docker:
	docker build -t ogoune:test .

run-ci: ci-local

# Tier 1 local CI gate — must pass before every push.
# Mirrors what GitHub Actions runs except for Docker-dependent lanes
# (dual-dialect Postgres + paired benches). Catches ~80% of CI breaks
# locally so we don't burn compute minutes on red lanes.
ci-local:
	@echo "=== 1/8 sqlc drift check ==="
	$(MAKE) sqlc-check
	@echo "=== 2/8 migrations drift check ==="
	$(MAKE) migrations-drift-check
	@echo "=== 3/8 OpenAPI contract + types drift guard ==="
	$(MAKE) openapi
	@git diff --exit-code -- api/openapi/ || { echo "OpenAPI contract stale: run 'make openapi' and commit api/openapi/"; exit 1; }
	$(MAKE) lint-openapi
	$(MAKE) gen-fe-types
	@git diff --exit-code -- web/packages/api-types/generated/ || { echo "FE types stale: run 'make gen-fe-types' and commit web/packages/api-types/generated/"; exit 1; }
	@echo "=== 4/8 Lint (go vet + pnpm lint) ==="
	$(MAKE) lint
	@echo "=== 5/8 Frontend type-check (vue-tsc) ==="
	$(MAKE) type-check-fe
	@echo "=== 6/8 Backend tests (race + timeout, SQLite) ==="
	go test -race -timeout 120s ./...
	@echo "=== 7/8 Frontend tests ==="
	$(MAKE) test-fe
	@echo "=== 8/8 License audit ==="
	$(MAKE) license-audit
	@echo "=== ci-local: ALL PASSED ==="

license-audit:
	@echo "=== SPDX coverage guard ==="
	scripts/license/check-spdx.sh
	@echo "=== Runtime-deps license guard ==="
	scripts/license/check-deps.sh
	@echo "=== Docs AGPL-drift guard ==="
	scripts/license/check-docs.sh
	@echo "=== License audit: ALL PASSED ==="

# Spec 051 — fuzz the dynquery SQL builders (30s per campaign).
fuzz-dynquery:
	go test -run=^$$ -fuzz=FuzzBuildMonitorsQuery -fuzztime=30s ./internal/repository/sqlc/dynquery/...
	go test -run=^$$ -fuzz=FuzzBuildIncidentsQuery -fuzztime=30s ./internal/repository/sqlc/dynquery/...

.PHONY: lint-openapi
lint-openapi: ## Lint the OpenAPI contract with Spectral (workspace binary — no global install)
	@echo ">> Lint OpenAPI..."
	cd web && pnpm exec spectral lint ../api/openapi/v1.yaml --ruleset ../.spectral.yaml --fail-severity=error

.PHONY: openapi
openapi: ## generate the canonical OpenAPI 3.1 contract from Go annotations (source of truth)
	go run github.com/swaggo/swag/v2/cmd/swag init -g cmd/api/main.go --v3.1 -o api/openapi --parseDependency --parseInternal
	@mv api/openapi/swagger.yaml api/openapi/v1.yaml
	@mv api/openapi/swagger.json api/openapi/v1.json
	@rm -f api/openapi/docs.go
	@echo ">> OpenAPI 3.1 contract → api/openapi/v1.{yaml,json}"

.PHONY: gen-fe-types
gen-fe-types: openapi ## regenerate committed frontend types from the contract
	cd web && pnpm --filter @ogoune/api-types generate
	@echo ">> Frontend types → web/packages/api-types/generated/schema.d.ts"

.PHONY: help
help: ## Affiche cette aide
	@echo "Ogoune — Commandes disponibles :"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort