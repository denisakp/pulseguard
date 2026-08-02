-- 0026: Generated monthly reports. One row per period (idempotency), terminal status only.
CREATE TABLE IF NOT EXISTS report_history (
    id                 TEXT PRIMARY KEY,
    period             TEXT NOT NULL,
    sent_at            TIMESTAMPTZ NOT NULL,
    status             TEXT NOT NULL,
    uptime_pct         DOUBLE PRECISION NOT NULL,
    incident_count     INTEGER NOT NULL,
    downtime_seconds   BIGINT NOT NULL,
    recipient_email    TEXT NOT NULL,
    resource_breakdown JSONB NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_report_history_period ON report_history(period);
CREATE INDEX IF NOT EXISTS idx_report_history_sent ON report_history(sent_at DESC);
