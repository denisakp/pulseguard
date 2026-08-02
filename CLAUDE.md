# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Ogoune

Uptime monitoring app that confirms failures before alerting (N consecutive failures required). Open-core model: Community Edition (Apache 2.0, SQLite + TimingWheel) and Enterprise Edition (LicenseRef-Ogoune-EE, Postgres + Redis/Asynq).

## Commands

```bash
# Build
make build              # frontend + backend
make build-be           # Go binary → dist/ogoune
make build-fe           # Vue SPA → web/dist

# Test
make test               # both
make test-be            # go test -race ./... (SQLite-only; Postgres tests skip)
make test-be-pg         # dual-dialect (SQLite + Postgres via testcontainers); needs Docker
make test-fe            # cd web && pnpm test
go test -race ./internal/scheduler/...  # single package
make ci-local           # full local CI gate: sqlc-check + drift + lint + fe type-check (vue-tsc) + race tests + frontend tests + license-audit
                        # `make run-ci` kept as alias. Run before every push to catch CI breaks locally.

# Lint
make lint               # go vet + pnpm lint

# Perf / fuzz (manual; not part of ci-local)
make bench-api          # Go httptest bench of GET /api/v1/{monitors,incidents} — SC-005/006 baseline + post-change check
make fuzz-dynquery      # 30s × 2 fuzz campaigns over the dynquery SQL builders (SQL injection guard)

# Run locally (community mode, zero deps)
cp .env.example .env    # set APP_SECRET_KEY (openssl rand -hex 32)
DB_DRIVER=sqlite SQLITE_PATH=./ogoune.db SCHEDULER_MODE=timingwheel go run ./cmd/api

# Run locally (full stack)
docker compose up -d                              # Postgres + Redis (prod-like)
docker compose -f docker-compose.dev.yml --profile full up -d  # dev stack; profiles gate services, plain `up` starts nothing
go run ./cmd/api

# Frontend dev
cd web && pnpm install && pnpm dev  # http://localhost:5173, needs VITE_API_BASE_URL

# OpenAPI (spec 074) — Go annotations are the source of truth
make openapi            # regenerate api/openapi/v1.{yaml,json} (OpenAPI 3.1, swag v2), commit it
make gen-fe-types       # regenerate web/packages/api-types/generated/schema.d.ts from the contract
make lint-openapi       # Spectral lint (workspace binary, no global install)
                        # After changing a v1 handler/DTO → run both + commit the artifacts.
                        # ci-local fails on drift. Runtime serves the embedded spec at
                        # /api/v1/openapi.json (no disk dep); UI at /api/v1/docs/* (ENABLE_SWAGGER=true).
                        # Frontend types: import from @ogoune/api-types (not hand-maintained).

# sqlc (type-safe DB queries)
make sqlc-generate      # regenerate Go code under internal/repository/sqlc/{pg,sqlite}/
make sqlc-check         # fail if generated code is drift vs queries (run by build-be + CI)

# Migrations drift (file-pair + column name + nullability across dialects)
make migrations-drift-check  # run by CI before tests

# Docker
make docker             # builds ogoune:test image
```

## Architecture

### Runtime flow

```
HTTP Router (Chi) → Handlers → Services → Repositories → DB (SQLite or Postgres)
                                  ↓
                            Scheduler (TimingWheel or Asynq)
                                  ↓
                            Worker pool → Check Strategies → Incident lifecycle → Notifications
```

Handlers never run checks or query DB directly. Scheduling goes through the Scheduler service. Workers execute checks via strategies, persist results, manage incidents, and dispatch notifications.

### Two runtime modes

| | Community | Production |
|---|---|---|
| DB | SQLite (in-process) | PostgreSQL |
| Scheduler | TimingWheel (in-process) | Asynq (Redis) |
| Scaling | Single binary | Stateless API + external workers |

### Key layers

- **Entry point**: `cmd/api/main.go` — thin orchestrator (~35 lines), delegates to `internal/platform/bootstrap/`
- **HTTP**: `internal/api/router.go` (Chi router), handlers in `internal/api/handler/`
- **Domain models**: `internal/domain/models.go` — source of truth, IDs are ULIDs (set in `EnsureID()` called explicitly by sqlc Create wrappers)
- **Services**: `internal/service/` — business logic, domain-level errors (not HTTP errors)
- **Ports (contracts)**: `internal/port/` — all interface definitions (repository, scheduler, notifier, monitoring)
- **Repositories**: `internal/repository/store/*_sqlc.go` — hand-written wrappers over sqlc-generated query bindings (under `internal/repository/sqlc/{pg,sqlite}/`). See `internal/repository/sqlc/README.md` for the contributor onboarding workflow. `internal/repository/interfaces.go` holds only error sentinels.
- **Scheduler**: `internal/scheduler/` — TimingWheel and Asynq implementations of `port.Scheduler`
- **Workers**: `internal/worker/` — `handler_monitoring.go` (check execution + incident triggering), `handler_expiry.go`, `handler_notification.go`, `handler_report.go` (spec 076: monthly report catch-up scan — daily tick generates the previous completed month if absent, idempotent per `period`; delivery via the oldest `smtp` notification channel with the report's recipient; failed send → `report_history.status=failed`, never aborts the job)
- **Check strategies**: `internal/monitoring/strategy/` — HTTP, TCP, DNS, ICMP, Keyword, Protocol
- **Incident logic**: `internal/monitoring/incident_service.go` — confirmation window, flap detection, alert grouping
- **Notifications**: `pkg/notifier/` — SMTP, Slack, Discord, Google Chat, Teams, webhooks
- **Encryption**: `pkg/crypto/` — AES-256-GCM for notification channel credentials
- **Feature plans**: `specs/NNN-name/` — speckit-driven plan/spec/tasks for each feature. Read the relevant `plan.md` before touching a feature area
- **Edition detection**: `internal/ee/license/` — `License.Get()` returns `community` or `enterprise` based on `ENTERPRISE_LICENSE_KEY` prefix (`pg_ent_` → enterprise). Runtime metadata only, does not gate behavior yet
- **Migrations**: `internal/database/migrations/sqlite/` and `postgres/` — dual trees, keep in sync

### Frontend (web/)

Vue 3 Composition API + TypeScript + Pinia + **NuxtUI 4** (Tailwind v4) + **Ky** + **Zod** + Iconify (lucide/heroicons). Migrated off Ant Design Vue + Axios (specs 057–073, ADR 0010).

- API calls only through `web/src/services/*.ts` using the **Ky** client (`web/src/core/http/client.ts` — `getAuthenticatedClient()` + `request<T>()`). v1 endpoints return a `{ data }` envelope (unwrap in the service).
- Forms: **Zod schemas under `web/src/schemas/`** consumed by `<UForm :schema :state>`; map server `ValidationError.fieldErrors` back via `formRef.setErrors`. See `web/src/schemas/README.md` and the oracle `web/src/views/_dev/UFormExampleView.vue`.
- Shared UI: `U*` components under `web/src/components/ui/` — catalogue in `web/src/components/ui/README.md`. Prefer NuxtUI built-ins (`UTable`, `UForm`, `USelect`, `UModal`, …); icons via Iconify (`i-lucide-*`).
- State: Pinia stores + composables (e.g., `web/src/composables/useResources.ts`). Types centralized in `web/src/types/index.ts`.
- No Options API; **no `ant-design-vue`/`@ant-design/*`/`axios` imports** — enforced by `no-restricted-imports` in `web/eslint.config.ts` (runs in `make lint` / `ci-local`).

### API versioning

- `/api/v1/` — stable public API, semver-protected
- `/api/` (non-versioned) — internal, may change anytime

**The root (`/api/`, non-versioned) API is FROZEN.** Do NOT add new handlers under
`internal/api/handler/*.go` or new routes to the non-`/v1` groups. All new
endpoints go to `internal/api/handler/v1/` under `/api/v1/`. Legacy root handlers
(resources, incidents, components, tags, notifications — duplicated in v1) are
migrated to v1 opportunistically, domain by domain, deleting the root handler +
routes once the frontend service is repointed. New features are v1-only.

## Patterns to follow

### Feature development flow (speckit — MANDATORY)

Every new feature MUST go through the speckit SDD cycle in this exact order. Do
not skip a step, do not reorder:

1. **specify** (`/speckit-specify`) — draft the feature spec from the request
2. **clarify** (`/speckit-clarify`) — resolve underspecified areas before planning
3. **plan** (`/speckit-plan`) — design artifacts + Constitution Check
4. **tasks** (`/speckit-tasks`) — dependency-ordered `tasks.md`
5. **analyze** (`/speckit-analyze`) — cross-artifact consistency (spec/plan/tasks)
6. **taskstoissues** (`/speckit-taskstoissues`) — turn tasks into GitHub issues
7. **implement** (`/speckit-implement`) — execute the tasks

Each step gates the next: don't plan on an unclarified spec, don't implement on
tasks that failed analyze. Artifacts land under `specs/NNN-name/` (gitignored per
the chantier convention). The bundled workflow is defined in
`.specify/workflows/speckit/workflow.yml`; the order is enforced by the
constitution's Delivery Workflow (`.specify/memory/constitution.md`).

### Adding a new monitor type

1. Implement `CheckStrategy` in `internal/monitoring/strategy/yourtype.go`
2. Add `ResourceYourType` constant to `internal/domain/models.go`
3. Register in `internal/platform/bootstrap/strategies.go`

### Adding a new notification channel

1. Implement `port.Notifier` in `pkg/notifier/yournotifier.go`
2. Add compile-time check in `pkg/notifier/verify.go`: `var _ Notifier = (*YourNotifier)(nil)`
3. Add constant to `internal/domain/models.go`
4. Add dispatch case in `internal/monitoring/incident_service.go`
5. Add config validation in `internal/service/notification_service.go`

### Adding a new repository

sqlc-only workflow. Full walkthrough at `internal/repository/sqlc/README.md`; canonical patterns at `internal/repository/sqlc/PATTERNS.md`.

1. Write paired migrations under `internal/database/migrations/{postgres,sqlite}/NNNN_yourtable.up.sql` + `.down.sql`. Run `make migrations-drift-check`.
2. Write paired sqlc queries under `internal/repository/sqlc/queries/{postgres,sqlite}/yourtable.sql`. Run `make sqlc-generate` and commit the result.
3. Add the domain struct to `internal/domain/models.go` (no `gorm:"..."` tags, no hooks).
4. Define the port interface in `internal/port/repository.go`.
5. Author the wrapper at `internal/repository/store/yourrepo_repository_sqlc.go` following the `PATTERNS.md` shape (dual-dialect dispatch, explicit `EnsureID()` + timestamps in `Create`).
6. Add compile-time check in `internal/repository/store/verify.go`: `var _ port.YourRepository = (*YourRepositorySQLC)(nil)`.
7. Author the dual-dialect contract test at `internal/repository/store/yourrepo_repository_contract_test.go` using `internaltest.ForEachDialect`.
8. Author the in-memory fake at `internal/repository/fake/yourrepo_fake.go` for handler/service unit tests.
9. Wire in `internal/platform/bootstrap/database.go`: `app.YourRepo = store.NewYourRepositorySQLC(rt)`.
10. `make ci-local`.

### Adding a new v1 API endpoint

1. Handler in `internal/api/handler/v1/` (interface + impl pattern)
2. DTOs in `internal/dto/v1/`
3. Route in `internal/api/router.go` (v1 sub-group)
4. Swaggo annotations, then `make openapi` + `make gen-fe-types`, commit `api/openapi/v1.{yaml,json}`
5. Test scope enforcement (read-only API key → 403 on writes)

### Database migrations

- Add to both `sqlite/` and `postgres/` trees
- SQLite: no `ADD COLUMN IF NOT EXISTS`, no multi-column `ALTER TABLE`
- Naming: `XXXX_description.up.sql` / `.down.sql`
- One migration = two files with the same `NNNN_` prefix and the same intent. Drift between trees is enforced by `make migrations-drift-check`
- Column **name + nullability MUST match** across dialects. Type tokens are intentionally NOT enforced cross-dialect (`JSONB`↔`TEXT`, `TIMESTAMPTZ`↔`TEXT`, `BIGINT`↔`INTEGER`)
- JSON columns: `JSONB` (Postgres) / `TEXT` (SQLite). See `internal/database/migrations/README.md` for the full type-mapping table
- `PRAGMA`, triggers, and stored functions in `.sql` migration files require tech-lead validation (sqlc compatibility risk)

### Testing

- DB tests use `internaltest.SetupSQLite(t)` for one-shot SQLite fixtures, or `internaltest.ForEachDialect(t, fn)` for dual-dialect contract tests
- Repository contract tests are dual-dialect (SQLite + Postgres) via `internal/repository/internaltest.ForEachDialect`. See `internal/repository/internaltest/README.md` for usage
- Table-driven tests for multi-case scenarios
- `go test -race ./...` must pass (SQLite-only path; Postgres tests skip gracefully)
- `make test-be-pg` runs the dual-dialect matrix locally; requires Docker. CI job `test-be-postgres` runs it on every PR
- Frontend: Vitest + jsdom, `*.spec.ts` colocated next to source or under `web/src/test/`

## Code Quality — SonarQube (MANDATORY)

Quality gate must pass before merge (in addition to `make lint` + `make test`).

```bash
make test-be && make test-fe        # coverage/unit.out + web/coverage/lcov.info
sonar-scanner                       # uses SONAR_TOKEN env var; config in sonar-project.properties
```

Dashboard: http://localhost:9009 (project `ogoune`). Block on CRITICAL/BLOCKER in either stack — Go (`cmd/`, `internal/`, `pkg/`) or Vue (`web/src/`). Two coverage reports are required because each stack scores independently. Cap at 3 fix-rescan cycles; report remaining if still failing.

## Gotchas

- `APP_SECRET_KEY` env var is mandatory — app refuses to start without it
- `.private/` is gitignored — drop personal scratch / strategy docs there. `specs/` is also gitignored per the chantier convention.
- Two docs trees, don't confuse: `docs/` = internal engineering docs (adrs/architecture/runbooks); `nebula/` = public VitePress site (docs.ogoune.com, auto-deployed on Vercel). Public-facing changes go in `nebula/`
- Repository errors: map `repository.ErrNotFound` to service-level errors, handlers map those to HTTP status
- Incident event steps (`detected`, `resource_down_alert`, `resolved`, `resource_up_alert`) may not all be present
- Never block on scheduler failures — log and return `ErrSchedulerSync`
- Commits use Conventional Commits format (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`)
- `web/.npmrc` sets `onlyBuiltDependencies=[]` — pnpm skips all install scripts. If a new dep needs native build, allowlist it explicitly
- After editing any `.sql` file under `internal/repository/sqlc/queries/`, run `make sqlc-generate` and commit the result. CI runs `make sqlc-check` and fails on drift. Generated code lives in `internal/repository/sqlc/{pg,sqlite}/` and is versioned
- After editing any migration `.sql` under `internal/database/migrations/`, run `make migrations-drift-check` locally. CI runs it before tests and fails on file-pair / column name / nullability divergence between dialects
- SQLite `strftime('%s', col)` returns NULL on modernc.org/sqlite-bound `time.Time` values (the driver binds Go's `String()` format, not RFC 3339). Use `strftime('%s', substr(col, 1, 19))` to extract the parseable `YYYY-MM-DD HH:MM:SS` prefix. See `internal/repository/sqlc/README.md` gotchas + `FindMissedHeartbeatsSQLite` for the canonical workaround.

<!-- SPECKIT START -->
For additional context about technologies to be used, project structure,
shell commands, and other important information, read the current plan:
`specs/085-resources-v1-migration/plan.md`
<!-- SPECKIT END -->
