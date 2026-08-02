# Changelog

All notable changes to this project will be documented in this file. Format
follows [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- **Monitors API parity with the root resources API (spec 085, Phase 1)** — additive
  endpoints on `/api/v1/monitors` so the versioned API can become the single source of
  truth: `GET /{id}/live` (live snapshot), `GET /{id}/uptime-stats`, `POST /{id}/tags`
  + `DELETE /{id}/tags/{tagID}` (204), `PATCH /{id}` (partial update; `PUT` retained), and
  the resource-credential routes re-mounted under `/api/v1/monitors/{id}/credentials*`. All
  reuse the existing services (no new logic, no migration); writes are read/write-scoped.
  The frontend repoint + root-handler deletion (Phase 2) is a deferred follow-up.
- **Server-side search endpoint (spec 084)** — `GET /api/v1/search?q=&limit=&categories=`
  powering the ⌘K command palette beyond the client-side (in-memory) window. Aggregates
  case-insensitive substring/prefix matches across monitors (name/target), incidents
  (cause), and app pages, returns a ranked, capped result set
  (`{results,total,query_duration_ms}`). Read-only (any valid credential), dual-dialect
  (SQLite + Postgres via the portable `LOWER() LIKE LOWER() ESCAPE` dynquery predicate,
  injection-safe), no pagination, no new migration. The palette frontend swap (with a
  client-side fallback) is a deferred follow-up.

---

## [1.0.0-beta] - 2026-08-02

First public release of Ogoune — uptime monitoring that **confirms failures
before alerting** (N consecutive failures required before an incident is
raised). Distributed under the Open Core model: Community Edition (Apache 2.0,
SQLite + TimingWheel) and Enterprise Edition (`LicenseRef-Ogoune-EE`, Postgres
+ Redis/Asynq).

This beta consolidates all pre-release development, including the completed
sqlc migration (specs 041–052) that made sqlc the sole data layer. See
[ADR 0003](docs/adrs/0003-sqlc-replaces-gorm.md) for the decision record and
lessons learned.

Beta scope: the public HTTP API (`/api/v1/*`) is considered stable. Please
report issues on the
[GitHub Discussions](https://github.com/denisakp/ogoune/discussions) thread
linked in the release notes.

### Added

- **Monitor types** — HTTP, TCP, DNS, ICMP, Keyword/content, and application
  Protocol checks.
- **Confirmation window** — incidents are only raised after N consecutive
  failed checks, eliminating false alerts from transient blips.
- **Incident lifecycle** — detection, confirmation, flap detection, alert
  grouping, and resolution, with per-step event history.
- **Multi-channel notifications** — SMTP, Slack, Discord, Google Chat, Teams,
  and generic webhooks. Channel credentials encrypted at rest (AES-256-GCM).
- **Status pages, monthly reports, and YAML bulk import/export** for
  resources.
- **Keyword / content check monitor** — HTTP GET that verifies the response
  body contains (`contains`) or does not contain (`not_contains`) a literal
  string, catching content failures behind an HTTP 200. Reads at most 512 KB of
  the body (`body_truncated` flag set beyond the cap). Failure diagnostics
  (`keyword`, `keyword_mode`, `keyword_found`) surface in the incident detail
  view and in enriched alert emails / webhook payloads.
- **DB migration `0011_keyword_fields`** — additive nullable columns on
  `resources` and `incident_diagnostics` for both SQLite and PostgreSQL.
- **Agent device monitoring — backend foundation (spec 079)** — a first-class
  **Host** domain plus authenticated agent metrics ingestion. New operator
  endpoints under `/api/v1/hosts` (register / list / get / delete, credential
  rotate & revoke, metric history) and a `POST/DELETE /api/v1/monitors/{id}/host`
  link. Agents stream CPU / memory / per-mount disk / network samples over a
  WebSocket at `/api/v1/agent/stream`, authenticated by a per-host `ag_live_…`
  bearer credential (hashed at rest, shown once). Bounded relational time-series
  with a daily retention job (decimate beyond 48h to ≤1/min, purge beyond 7d) in
  both TimingWheel and Asynq modes. DB migration `0028_hosts` adds `hosts`,
  `host_credentials`, `host_metrics`, and a nullable `resources.host_id`. The
  agent is strictly optional — no core monitoring path depends on a host. The
  agent binary and UI are separate, forthcoming chantiers.
- **Host agent binary (spec 080)** — `cmd/agent`, the Go daemon installed on a
  monitored Linux host. Collects OS, CPU %, memory %, per-mount disk %, and
  network counters via gopsutil and streams them to `/api/v1/agent/stream` every
  ~10s, authenticated by the host's `ag_live_…` credential. Config from
  `/etc/ogoune/agent.cfg` (env-style `KEY=value`) with env/flag overrides; a
  systemd unit is provided (`packaging/agent/`). Fail-safe: reconnects with
  exponential back-off, and on a revoked credential slows to a capped (~5 min)
  infinite back-off so it self-heals when the credential is re-validated — never
  crashing the host, never tight-looping. The agent↔backend frame is a single
  shared contract (`pkg/agentwire`, carrying `schema_version`) imported by both
  sides so they cannot drift; the 079 ingestion handler adopts it
  backward-compatibly (a frame with no `schema_version` is treated as v1).
- **Hosts UI (spec 081)** — the operator frontend for agent device monitoring. A
  **Hosts** nav section, a list page (online/offline,
  live CPU/memory/disk, hosted-service count, last-seen; trouble-first sort), a
  detail page (identity + install helper, CPU/memory/per-mount-disk/network graphs
  over 1h/6h/24h/7d, and the linked-monitors list), an in-app register/onboard flow
  that shows the `ag_live_…` credential once plus rotate/revoke/delete, and a Host
  context panel + link/unlink control on the monitor page. Frontend-only, no
  backend change (host metrics graphs are hand-rolled SVG — no charting dependency);
  monitor↔host data is read from the versioned `/api/v1/monitors` endpoint.
- **Agent packaging & distribution (spec 082)** — the host agent ships as real
  artifacts instead of a build-from-source step. A multi-arch
  (`linux/amd64,arm64`) container image `ghcr.io/denisakp/ogoune-agent` (distroless
  static, nonroot) plus static release binaries (`ogoune-agent-linux-{amd64,arm64}`
  + `SHA256SUMS`) are built and published by `release.yml` on every tag, versioned
  in lockstep with the API image. Docker Compose gains an agent service: the dev
  stack auto-registers a `local` host and hands the agent its credential
  (zero-touch dogfood), while the prod stack keeps it opt-in behind
  `profiles: [agent]` with an operator-supplied credential — no secret is ever
  baked into an image, layer, or committed compose file. The agent's default
  config path moves to `/etc/ogoune/agent.cfg` (env-style, doubling as the systemd
  `EnvironmentFile` / `docker --env-file`), and it now logs a warning when
  connecting over plaintext `ws://` to a remote host (use `wss://` in production).
- **Agent-down alerting (spec 083)** — the server now notifies you when an
  agent-backed host stops reporting. A recurring liveness scan raises a single
  in-app feed notification ("Host X went offline") once a host has been
  continuously offline past `HOST_FRESHNESS_THRESHOLD`, and a matching "back
  online" notification on recovery — one alert per offline episode (flap- and
  restart-safe, tracked in a new `host_alert_state` table). Optional email
  delivery via the oldest SMTP channel (`AGENT_DOWN_EXTERNAL_DELIVERY=true`).
  Configurable via `AGENT_DOWN_ALERTS_ENABLED` / `AGENT_DOWN_SCAN_INTERVAL`;
  fail-safe (alert/delivery failures never disturb monitoring).
- **Unread-notification escalation (spec 083)** — the server now escalates
  actionable feed notifications (incidents and agent-down alerts) that stay
  **unread** too long. A recurring scan emails a single grouped digest ("N unread
  notifications need attention" plus a short list) via the oldest SMTP channel,
  batched (one digest, not one email per notification) and deduplicated — re-sent
  only when a newer qualifying notification appears, never once the operator has
  read them; purely informational entries are ignored. Configurable via
  `NOTIFICATION_ESCALATION_ENABLED` (default `true`),
  `NOTIFICATION_ESCALATION_SCAN_INTERVAL` (`15m`), and
  `NOTIFICATION_ESCALATION_UNREAD_AGE` (`30m`). Fail-safe: with no SMTP channel it
  silently no-ops, and a send failure never disturbs monitoring.

### Changed

- All repositories are backed by sqlc-generated query bindings. Every
  `internal/repository/store/*_sqlc.go` wrapper is the sole implementation of
  its `port.*Repository` interface.
- Postgres is driven directly by `pgx/v5` + `pgxpool`; SQLite by
  `modernc.org/sqlite` via `database/sql`.
- Migrations are applied by a thin `database/sql` runner over the SQL files
  under `internal/database/migrations/{postgres,sqlite}/`. Startup fails fast on
  any apply error, wrapping the failing file + dialect in the returned error.

### Removed

- No GORM dependency: the tree ships without `gorm.io/*` or
  `github.com/glebarez/sqlite`, and without GORM struct tags or lifecycle
  hooks. ID generation, encryption, and decryption live explicitly in the sqlc
  Create/Update wrappers; `EnsureID()` is the remaining plain method on `Base`.

### Deprecated

- **Legacy `SQLC_*` environment flags** (16 vars) are no longer read by the
  binary. They are silently ignored — leaving them in your `.env` is safe and
  has no effect. The regression test `TestLegacyFlagsSilentlyIgnored` guards
  this behaviour going forward.
- **Agent YAML config at `/etc/ogoune-agent.yaml`** — replaced by the env-style
  `/etc/ogoune/agent.cfg` (spec 082). Existing installs keep working via
  environment variables and flags; point `--config` / `OGOUNE_CONFIG` at the old
  file if you must. Docs and the shipped example now use the new path/format.

### Performance

- `GET /api/v1/monitors` and `GET /api/v1/incidents` p95 latency, and cold-boot
  p95, are all within ±10 % of the pre-migration baseline captured before the
  GORM-removal commit.

### Documentation

- **ADR reference**: [`docs/adrs/0003-sqlc-replaces-gorm.md`](docs/adrs/0003-sqlc-replaces-gorm.md)
  — the strategic decision record covering context, alternatives, and the
  migration plan.
- **Contributor guide**: `internal/repository/sqlc/README.md` — 9-step
  walkthrough for adding a new repository, sqlc-only.
- **Patterns catalogue**: `internal/repository/sqlc/PATTERNS.md`.
- **Public documentation site** — VitePress site under `nebula/`, published at
  [docs.ogoune.com](https://docs.ogoune.com), with a live OpenAPI reference
  rendered from `api/openapi/v1.json`. Auto-deployed on Vercel.

### Fixed

- **Docker image build** — the go-builder stage now copies `api/` so the
  embedded OpenAPI spec (`//go:embed v1.json`) resolves. The release image
  previously failed to compile (`no required module provides package
  .../api/openapi`).
- **CI (license guards)** — pin pnpm via `web/package.json`, add the missing
  `web/.nvmrc` (node 24), and bump `pnpm/action-setup` + `actions/upload-artifact`
  for the updated GitHub runner.
