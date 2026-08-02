-- 0029: Agent-down alert state (spec 083). See postgres/0029 for rationale.
CREATE TABLE IF NOT EXISTS host_alert_state (
    host_id       TEXT PRIMARY KEY REFERENCES hosts(id) ON DELETE CASCADE,
    state         TEXT NOT NULL,
    offline_since DATETIME,
    alerted       INTEGER NOT NULL,
    updated_at    DATETIME NOT NULL
);
