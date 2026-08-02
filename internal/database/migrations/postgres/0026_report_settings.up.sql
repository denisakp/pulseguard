-- 0026: Monthly report settings. Single-tenant: one instance-wide config row.
CREATE TABLE IF NOT EXISTS report_settings (
    id              TEXT PRIMARY KEY,
    enabled         BOOLEAN NOT NULL,
    recipient_email TEXT NOT NULL,
    schedule        TEXT NOT NULL,
    scope           TEXT NOT NULL,
    last_sent_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);
