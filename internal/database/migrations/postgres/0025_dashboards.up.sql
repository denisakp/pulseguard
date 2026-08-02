-- 0025: Custom dashboards. Single-tenant: instance-wide read; owner_id governs mutation.
-- Config-only: scope + widgets are JSON config; widget data renders frontend-side.
CREATE TABLE IF NOT EXISTS dashboards (
    id                 TEXT PRIMARY KEY,
    owner_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    scope              JSONB NOT NULL,
    widgets            JSONB NOT NULL,
    default_time_range TEXT NOT NULL,
    refresh_interval   TEXT NOT NULL,
    visibility         TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_dashboards_owner ON dashboards(owner_id);
CREATE INDEX IF NOT EXISTS idx_dashboards_updated ON dashboards(updated_at DESC);
