-- 0025: Custom dashboards. Single-tenant: instance-wide read; owner_id governs mutation.
-- Config-only: scope + widgets are JSON config; widget data renders frontend-side.
CREATE TABLE IF NOT EXISTS dashboards (
    id                 TEXT PRIMARY KEY,
    owner_id           TEXT NOT NULL,
    name               TEXT NOT NULL,
    scope              TEXT NOT NULL,
    widgets            TEXT NOT NULL,
    default_time_range TEXT NOT NULL,
    refresh_interval   TEXT NOT NULL,
    visibility         TEXT NOT NULL,
    created_at         DATETIME NOT NULL,
    updated_at         DATETIME NOT NULL,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_dashboards_owner ON dashboards(owner_id);
CREATE INDEX IF NOT EXISTS idx_dashboards_updated ON dashboards(updated_at DESC);
