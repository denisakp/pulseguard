-- 0024: In-app notification feed. Single-tenant: user_id NULL = instance-wide.
-- Global read state: a single read_at per row (shared by all users).
CREATE TABLE IF NOT EXISTS notifications (
    id          TEXT PRIMARY KEY,
    user_id     TEXT REFERENCES users(id) ON DELETE CASCADE,
    category    TEXT NOT NULL,
    severity    TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    deep_link   TEXT,
    payload     JSONB,
    occurred_at TIMESTAMPTZ NOT NULL,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_occurred ON notifications(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_occurred ON notifications(occurred_at DESC);
