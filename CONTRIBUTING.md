# Contributing to Ogoune

First off — thank you. Ogoune is a small project and every contribution matters.

This document tells you everything you need to know to go from "I want to help" to "my PR is merged."

---

## Table of Contents

- [Ways to contribute](#ways-to-contribute)
- [Before you start](#before-you-start)
- [Development setup](#development-setup)
- [Project structure](#project-structure)
- [Making a change](#making-a-change)
- [Pull request guidelines](#pull-request-guidelines)
- [For Go contributors](#for-go-contributors)
- [For Vue 3 contributors](#for-vue-3-contributors)
- [For DevOps / SRE contributors](#for-devops--sre-contributors)
- [For technical writers](#for-technical-writers)
- [Issue labels explained](#issue-labels-explained)
- [Code of conduct](#code-of-conduct)
- [Outbound licensing of your contributions](#outbound-licensing-of-your-contributions)

---

## Ways to contribute

You don't need to write code to contribute.

| Contribution | How |
|---|---|
| Report a bug | [Open an issue](https://github.com/denisakp/ogoune/issues/new) |
| Request a feature | [Start a discussion](https://github.com/denisakp/ogoune/discussions/new) |
| Fix a bug | Pick a [`bug`](https://github.com/denisakp/ogoune/labels/bug) issue and open a PR |
| Add a feature | Discuss first, then PR |
| Improve docs | Edit any `.md` file and open a PR |
| Share feedback | [Anonymous form](https://kawa-bunga.notion.site/2d1e5ad0a17d80dc8859e77817d901e3) — 2 minutes |
| Tell others | Star the repo, share on Reddit, write a blog post |

**Not sure where to start?** Look for issues labeled [`good first issue`](https://github.com/denisakp/ogoune/labels/good%20first%20issue) — these are self-contained and well-documented.

---

## Before you start

### For bug fixes and small improvements

Just open a PR. No need to ask first.

### For new features or significant changes

**Open a discussion or issue first.** Explain what you want to build and why. This avoids wasted effort if the direction doesn't fit the project.

The rule of thumb: if the change takes more than a day to implement, discuss it first.

### For security vulnerabilities

**Do not open a public issue.** See [SECURITY.md](./SECURITY.md) for responsible disclosure instructions.

---

## Development setup

### Prerequisites

| Tool | Version | Install |
|---|---|---|
| Go | 1.24+ | [go.dev](https://go.dev/dl/) |
| Node.js | 22+ | [nodejs.org](https://nodejs.org) |
| Docker | Latest | [docker.com](https://www.docker.com) |
| Git | Latest | [git-scm.com](https://git-scm.com) |

### Get started

```bash
# Clone the repo
git clone https://github.com/denisakp/ogoune.git
cd ogoune

# Start the full stack (PostgreSQL + Redis)
cp .env.example .env
docker compose up -d

# Or start with zero dependencies (SQLite + in-process scheduler)
docker compose -f docker-compose.yml up -d
```

### Run backend only

```bash
cd backend
go mod download
go run ./cmd/api
```

### Run frontend only

```bash
cd frontend
npm install
npm run dev
# → http://localhost:5173
```

### Run tests

```bash
# Backend — all tests
cd backend
go test ./...

# Backend — with race detector (recommended before opening a PR)
go test -race ./...

# Frontend
cd frontend
npm run test
```

---

## Project structure

```
ogoune/
├── cmd/api/              → thin entry point (delegates to bootstrap)
├── internal/platform/bootstrap/ → application composition root (wiring)
├── internal/
│   ├── api/              → HTTP router, handlers, middleware
│   ├── domain/           → models, constants (models.go is the source of truth)
│   ├── monitoring/       → check strategies (HTTP, TCP, DNS) + incident logic
│   ├── worker/           → check execution handlers
│   ├── scheduler/        → TimingWheel (Community) + Asynq (production)
│   ├── database/         → SQLite + PostgreSQL drivers
│   ├── service/          → business logic (resource, incident, notification...)
│   └── maintenance/      → maintenance window scheduler
├── pkg/                  → reusable packages (notifier, apikey...)
│
├── web/
│   └── src/
│       ├── router/       → Vue Router routes
│       ├── views/        → page components
│       ├── components/   → shared UI components
│       ├── composables/  → Vue 3 composables (useMonitorLive, etc.)
│       └── stores/       → Pinia state management
│
├── docker-compose.yml              → full stack (PostgreSQL + Redis)
├── docker-compose.community.yml   → zero dependencies (SQLite)
└── .env.example                    → all available environment variables
```

The best starting point for understanding the codebase:
- `internal/domain/models.go` — all data models
- `internal/api/router.go` — all API routes
- `internal/platform/bootstrap/` — how everything is wired together (one file per concern)

---

## Making a change

```bash
# 1. Fork the repo and clone your fork
git clone https://github.com/YOUR_USERNAME/ogoune.git

# 2. Create a branch — use a descriptive name
git checkout -b fix/ssl-expiry-nil-panic
git checkout -b feat/google-chat-notification
git checkout -b docs/improve-api-key-setup

# 3. Make your changes

# 4. Run tests
cd backend && go test -race ./...
cd frontend && npm run test

# 5. Commit with a clear message (see below)
git commit -m "fix: handle nil SSL expiration date in enrichment service"

# 6. Push and open a PR
git push origin fix/ssl-expiry-nil-panic
```

### Commit message format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <short description>

Types:
  feat     → new feature
  fix      → bug fix
  docs     → documentation only
  refactor → code change that neither fixes a bug nor adds a feature
  test     → adding or updating tests
  chore    → build process, dependencies, tooling
```

Examples:
```
feat: add Google Chat notification channel
fix: prevent duplicate expiry alerts on cert renewal
docs: add webhook configuration example
test: add flap detection integration tests
refactor: extract confirmation window logic into service
```

---

## Pull request guidelines

### What makes a good PR

- **One thing per PR** — a fix and an unrelated refactor in the same PR will be asked to split
- **Tests included** — bug fixes need a test that reproduces the bug; features need tests covering the happy path
- **No breaking changes without discussion** — API changes, model changes, config changes — open an issue first
- **Description explains the why** — not just what changed, but why it matters

### PR description template

```markdown
## What this does
Brief description of the change.

## Why
Why is this needed? Link to the issue if applicable.

## How to test
Steps to verify the change works.

## Checklist
- [ ] Tests added or updated
- [ ] go test -race ./... passes
- [ ] No unrelated changes
```

### What to expect

- **Response time** — I try to review PRs within a few days. If you haven't heard back in a week, ping the issue.
- **Feedback** — I'll leave comments if something needs changing. This is normal — it's not a rejection.
- **Merging** — once approved, I'll merge it. I may squash commits to keep the history clean.

---

## For Go contributors

### Code style

- Follow standard Go conventions — `gofmt` is mandatory
- Run `go vet ./...` before submitting
- Errors are explicit — no silent ignores (`_ = err` needs a comment explaining why)
- Interfaces are small — define them where they're used, not in a central file
- No global state — dependencies are injected

### Where to add a new monitor type

1. Create `internal/monitoring/strategy/yourtype.go`
2. Implement the `Strategy` interface
3. Add `ResourceYourType ResourceType = "yourtype"` to `domain/models.go`
4. Register it in `internal/platform/bootstrap/strategies.go`

### Where to add a new notification channel

1. Create `pkg/notifier/yournotifier.go`
2. Implement the `Notifier` interface
3. Add `NotificationChannelTypeYours NotificationChannelType = "yours"` to `domain/models.go`
4. Add dispatch case in `internal/monitoring/incident_service.go`
5. Add config validation in `internal/service/notification_service.go`

### Database migrations

- Add migration files to both `database/migrations/postgres/` and `database/migrations/sqlite/`
- SQLite does not support `ADD COLUMN IF NOT EXISTS` or multiple columns in one `ALTER TABLE`
- Use `GORM serializer:json` instead of `type:jsonb` for JSON fields — works on both drivers
- Follow the existing naming convention: `XXXX_description.up.sql` / `.down.sql`

### Testing

- Use `setupTestDB(t)` helper (SQLite in-memory) for DB tests — no external infrastructure
- Table-driven tests for anything with multiple cases
- `go test -race ./...` must pass — no data races

---

## For Vue 3 contributors

### Code style

- Composition API only — no Options API
- `<script setup>` syntax preferred
- TypeScript for all new files
- Pinia for shared state — no Vuex, no event bus

### Component conventions

- One component per file
- Props typed with TypeScript interfaces
- Emits declared explicitly
- No direct DOM manipulation — use Vue refs

### API calls

- Follow the existing pattern in `web/src/composables/`
- Use the `useApi()` composable for all HTTP calls — no raw `fetch` or `axios` directly in views
- Handle loading, error, and empty states explicitly — never leave the user staring at a blank screen

### Styling

- Follow the existing design system — don't introduce new color variables or component libraries
- Dark mode must work — test it before submitting

### Adding a new page

1. Create the view in `web/src/views/`
2. Add the route in `web/src/router/index.ts`
3. Add navigation entry if needed

---

## For DevOps / SRE contributors

Your perspective is uniquely valuable — you're the target user.

**What we most need from you:**

- **Battle testing** — deploy Ogoune in your real environment and tell us what breaks, what's confusing, or what's missing
- **Performance feedback** — how does it behave with 100+ monitors? 500+? What degrades first?
- **Docker / deployment feedback** — is the `docker-compose.community.yml` actually simple to use? What's missing?
- **Alert quality feedback** — are there false positives you're still seeing? Edge cases the confirmation window misses?

**How to share this:**

- Open an issue with the `feedback` label
- Or use the [anonymous feedback form](https://kawa-bunga.notion.site/2d1e5ad0a17d80dc8859e77817d901e3)

You don't need to write code. Detailed feedback about real-world usage is worth more than most PRs.

---

## For technical writers

### What needs documenting

- `QUICKSTART.md` — the detailed setup guide (currently sparse)
- `docs/architecture/ARCHITECTURE.md` — how the backend is designed
- Inline code comments — especially in `handler_monitoring.go` and `incident_service.go`
- API reference — the endpoints, request/response shapes

### Style guide

- English only
- Short sentences — monitoring tool users are in a hurry
- Show, don't tell — prefer code examples over prose descriptions
- Second person — "you" not "the user"
- No filler — "simply", "just", "easily" add no information

### How to submit doc changes

Same as code — fork, branch, PR. Doc-only PRs are reviewed quickly.

---

## Issue labels explained

| Label | Meaning |
|---|---|
| `good first issue` | Self-contained, well-scoped, good starting point |
| `bug` | Something is broken |
| `enhancement` | New feature or improvement |
| `help wanted` | Maintainer needs input or a contributor to pick this up |
| `feedback` | Asking for real-world usage feedback |
| `documentation` | Docs only |
| `backend` | Go / backend change |
| `frontend` | Vue / frontend change |
| `wontfix` | Out of scope or intentional behavior |

---

## Code of conduct

Be direct. Be respectful. Assume good intent.

- Criticism of code is not criticism of the person
- If something is unclear, ask — don't assume
- English is not everyone's first language — be patient

Issues or PRs that are disrespectful will be closed without comment.

---

## Questions?

- **GitHub Discussions** — [ask anything](https://github.com/denisakp/ogoune/discussions)
- **GitHub Issues** — [report a bug](https://github.com/denisakp/ogoune/issues)

---

*Thanks for taking the time to contribute — it means a lot.*
---

## Security Configuration

Ogoune includes three security controls configured via environment variables. All are active by default with safe defaults.

### CORS (Cross-Origin Request Protection)

```bash
# Comma-separated allowed origins. Empty = same-origin only (no cross-origin).
CORS_ALLOWED_ORIGINS=https://app.example.com,https://staging.example.com
```

When empty, all cross-origin requests are rejected. Only listed origins receive `Access-Control-Allow-Origin` headers. Rejected origins are logged with `[security] event=cors_reject`.

### SSRF Protection (Server-Side Request Forgery)

Monitor targets are validated at both creation and execution time against a blocklist of internal/private IP ranges (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, ::1/128, fe80::/10, fc00::/7). This is always on and not configurable — there is no legitimate reason to monitor internal addresses from a public instance.

- **Creation time**: `ValidateResourceTarget()` rejects blocked targets with a 422 error
- **Execution time**: `SafeTransport` / `SafeDial` block DNS rebinding attacks where a hostname resolves to an internal IP at check time

Blocked attempts are logged with `[security] event=ssrf_block`.

### Rate Limiting

```bash
# Format: <count>/<duration>  (e.g., 10/1m = 10 requests per minute)
RATE_LIMIT_AUTH=10/1m      # Auth endpoints (/auth/login, /auth/verify-2fa, etc.)
RATE_LIMIT_GLOBAL=100/1m   # All authenticated endpoints
```

Rate-limited requests receive HTTP 429 with a `Retry-After` header. Events are logged with `[security] event=rate_limit`.

---

## API Versioning Policy

### Stable public API (`/api/v1/`)

All endpoints under `/api/v1/` form the stable public API surface. Changes to these endpoints must follow the rules below:

- **Non-breaking changes** (adding optional fields, new endpoints) are allowed in any release.
- **Breaking changes** (removing fields, changing field types, removing endpoints, changing auth requirements) require a new API version (`/api/v2/`) and a deprecation notice in the release notes.
- **Internal APIs** under `/api/` (non-versioned) are not subject to semver guarantees and may change at any time.

### Adding a new v1 endpoint

1. Define the handler in `internal/api/handler/v1/` using the v1 handler pattern (interface + implementation).
2. Add request/response DTOs to `internal/dto/v1/`.
3. Wire the route in `internal/api/router.go` inside the authenticated or unauthenticated v1 sub-group as appropriate.
4. Add swaggo annotations (`@Summary`, `@Tags`, `@Security`, `@Router`) to the handler method.
5. Run `make openapi` (and `make gen-fe-types` if DTOs changed) to regenerate the spec, then commit `api/openapi/v1.{yaml,json}`.
6. Write tests for scope enforcement (read-only key must get 403 on write endpoints).

### Swagger / OpenAPI spec

- The spec is generated from source annotations via `make openapi` (OpenAPI 3.1, swag v2).
- `GET /api/v1/openapi.json` always serves the spec (no auth required).
- The Swagger UI at `GET /api/v1/docs/*` is gated by `ENABLE_SWAGGER=true` (default: false in production).
- Always commit the generated `api/openapi/v1.{yaml,json}` after running `make openapi`. Run `make lint-openapi` (Spectral) before pushing.

---

## Outbound licensing of your contributions

Ogoune follows an **Open Core** dual-licensing model:

- The **core** (everything outside `internal/ee/`) is distributed under **Apache License 2.0** — see [`LICENSE`](./LICENSE).
- The **Enterprise Edition** (`internal/ee/` and any file carrying the SPDX identifier `LicenseRef-Ogoune-EE`) is distributed under a separate commercial source-available licence — see [`LICENSE.ee`](./LICENSE.ee).

All contributions are subject to the Ogoune **Contributor License Agreement** ([`cla.md`](./cla.md), currently v1.1). By signing the CLA you grant us the right to distribute your contribution under any OSI-approved open source licence and/or any proprietary or commercial licence. In concrete terms today: a contribution to the core ships under Apache 2.0; a contribution to `internal/ee/` ships under `LicenseRef-Ogoune-EE`. The CLA bot prompts you to accept the current CLA version on your first pull request.

**Prior licensing**: Ogoune's core was previously licensed under **AGPL v3**. The relicensing to Apache 2.0 predates any tagged release. Any copy obtained under AGPL remains AGPL; the dual model above governs the current tree and all releases from `v1.0.0-beta` onward.

If you have a question about how the licence applies to a specific contribution, open a GitHub Discussion or email `hello@ogoune.com` before submitting the PR.
