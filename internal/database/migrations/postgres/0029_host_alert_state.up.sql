-- 0029: Agent-down alert state (spec 083). One row per host tracks the current
-- offline episode so at most one offline alert + one recovery fire per episode,
-- and the state survives restarts. FK cascade cleans up when a host is deleted.
CREATE TABLE IF NOT EXISTS host_alert_state (
    host_id       TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    state         TEXT NOT NULL,
    offline_since TIMESTAMPTZ,
    alerted       BOOLEAN NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);
