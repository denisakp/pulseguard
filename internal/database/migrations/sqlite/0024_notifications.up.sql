-- 0024: In-app notification feed. Single-tenant: user_id NULL = instance-wide.
-- Global read state: a single read_at per row (shared by all users).
CREATE TABLE IF NOT EXISTS notifications (
    id          TEXT PRIMARY KEY,
    user_id     TEXT,
    category    TEXT NOT NULL,
    severity    TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT,
    deep_link   TEXT,
    payload     TEXT,
    occurred_at DATETIME NOT NULL,
    read_at     DATETIME,
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_occurred ON notifications(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_occurred ON notifications(occurred_at DESC);
