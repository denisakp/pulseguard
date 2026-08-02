-- 0026: Generated monthly reports. One row per period (idempotency), terminal status only.
CREATE TABLE IF NOT EXISTS report_history (
    id                 TEXT PRIMARY KEY,
    period             TEXT NOT NULL,
    sent_at            DATETIME NOT NULL,
    status             TEXT NOT NULL,
    uptime_pct         REAL NOT NULL,
    incident_count     INTEGER NOT NULL,
    downtime_seconds   INTEGER NOT NULL,
    recipient_email    TEXT NOT NULL,
    resource_breakdown TEXT NOT NULL,
    created_at         DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_report_history_period ON report_history(period);
CREATE INDEX IF NOT EXISTS idx_report_history_sent ON report_history(sent_at DESC);
