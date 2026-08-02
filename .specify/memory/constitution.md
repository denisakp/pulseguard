<!--
Sync Impact Report
Version change: 1.0.0 -> 1.1.0 (MINOR — Delivery Workflow now mandates the full
7-step speckit cycle: specify → clarify → plan → tasks → analyze →
taskstoissues → implement)
Dependent files updated in same change:
- .specify/workflows/speckit/workflow.yml (added clarify, analyze,
  taskstoissues steps + gates; version 1.0.0 -> 1.1.0)
- .specify/workflows/workflow-registry.json (description + version)
- CLAUDE.md ("Feature development flow (speckit — MANDATORY)" section)

Prior report (initial ratification):
Version change: template -> 1.0.0
Modified principles:
- Template principle 1 -> I. Layered Boundary Integrity
- Template principle 2 -> II. Community Simplicity, Hosted Continuity
- Template principle 3 -> III. Automated Verification for Runtime Changes
- Template principle 4 -> IV. Migration and Startup Safety
- Template principle 5 -> V. Spec-to-Execution Traceability
Added sections:
- Engineering Constraints
- Delivery Workflow
Removed sections:
- None
Templates requiring updates:
- ✅ /.specify/templates/plan-template.md
- ✅ /.specify/templates/spec-template.md
- ✅ /.specify/templates/tasks-template.md
- ⚠ pending: /.specify/templates/commands/*.md (directory not present in repository)
Follow-up TODOs:
- None
-->
# Ogoune Constitution

## Core Principles

### I. Layered Boundary Integrity

Ogoune MUST preserve its explicit layering. Backend HTTP handlers MUST delegate business logic to services, services MUST orchestrate repositories and schedulers instead of performing transport concerns, and repositories MUST remain the only persistence boundary. Frontend components MUST stay presentational and MUST use services, stores, or composables rather than performing direct API orchestration in views. Any proposed shortcut across these boundaries requires a documented justification in the implementation plan.

Rationale: Ogoune already relies on clear backend and frontend boundaries to keep monitoring behavior, persistence, and UI state understandable and maintainable.

### II. Community Simplicity, Hosted Continuity

Changes intended to reduce self-hosting friction MUST not silently regress the hosted or PostgreSQL-backed path. Community-mode improvements MUST explicitly state which external dependencies are removed, which remain, and how operators select modes. If a feature affects runtime infrastructure, the hosted/default path MUST remain supported unless the spec declares otherwise.

Rationale: Ogoune serves both self-hosters who need low-friction setup and existing deployments that depend on stable PostgreSQL-backed behavior.

### III. Automated Verification for Runtime Changes

Any change touching persistence, configuration loading, startup flow, scheduling, incident lifecycle, or notification dispatch MUST add or update automated tests. When the goal is simplified local or CI operation, tests MUST run without external infrastructure whenever technically feasible. Missing automation for a runtime-critical change is a policy violation unless the limitation and temporary mitigation are documented in the plan and review.

Rationale: Ogoune's most expensive failures are runtime regressions, not isolated code-style issues; verification must target actual operational risk.

### IV. Migration and Startup Safety

Schema, migration, and startup behavior MUST be deterministic, operator-visible, and safe by default. Authoritative schema evolution mechanisms MUST be declared in the spec and honored by implementation. Startup MUST fail before serving traffic when migrations or required runtime initialization fail, and resulting errors MUST identify the broken input or operator action needed. Security-sensitive local artifacts SHOULD be hardened by default and MUST emit a warning when full hardening cannot be applied.

Rationale: Monitoring systems are infrastructure; partial startup or opaque failure modes create incidents rather than preventing them.

### V. Spec-to-Execution Traceability

Every non-trivial feature MUST maintain traceability from specification to planning to task execution. Specs MUST describe independently testable user stories, explicit out-of-scope items, and operational impact when runtime behavior changes. Plans MUST document constitution gates, technical constraints, and rollout implications. Tasks MUST map work to user stories, concrete files, validation evidence, and documentation/config updates when behavior changes.

Rationale: Ogoune work spans runtime behavior, operational docs, and UI/API boundaries; traceability prevents hidden scope drift.

## Engineering Constraints

- Backend changes MUST follow the existing Go layering documented in `.github/copilot-instructions.md` and backend architecture docs.
- Frontend changes MUST preserve the current Vue service/store/composable architecture and MUST NOT introduce direct Axios usage in components.
- All public API behavior remains JSON-first; runtime configuration changes MUST be reflected in environment documentation.
- Changes to persistence formats, IDs, or incident semantics MUST consider existing PostgreSQL data and current operator expectations.
- Documentation updates are mandatory when a change affects setup, environment variables, startup behavior, deployment topology, or feature scope.

## Delivery Workflow

- Non-trivial work MUST proceed through the speckit SDD cycle in this exact order: **specify → clarify → plan → tasks → analyze → taskstoissues → implement**. No step may be skipped or reordered. Each step gates the next: planning requires a clarified spec, task generation requires an approved plan, and issue creation plus implementation require a passing cross-artifact analysis. The bundled workflow definition lives at `.specify/workflows/speckit/workflow.yml`.
- Constitution Check in every plan MUST explicitly evaluate all five core principles and list any justified exceptions.
- Runtime or infrastructure changes MUST include: automated test tasks, negative-path validation, deployment/config documentation tasks, and final verification evidence.
- Reviews MUST confirm architecture boundaries, non-regression coverage, operator-facing documentation updates, and traceability from requirements to tasks.
- Features may ship incrementally, but each user story MUST remain independently testable and demonstrable.

## Governance

This constitution supersedes conflicting local planning habits and template defaults. Amendments MUST update this file and any affected templates or guidance documents in the same change. Versioning follows semantic versioning: MAJOR for incompatible principle or governance changes, MINOR for new principles or materially expanded mandates, PATCH for wording clarifications that do not change obligations. Every plan and review MUST include a constitution compliance check. `README.md`, `QUICKSTART.md`, backend architecture guidance, and `.github/copilot-instructions.md` remain the authoritative companion references for runtime and codebase-specific practices.

**Version**: 1.1.0 | **Ratified**: 2026-03-23 | **Last Amended**: 2026-07-30
