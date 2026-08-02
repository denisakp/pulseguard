-- 0030: Unread-notification escalation state (spec 083). See postgres/0030.
CREATE TABLE IF NOT EXISTS notification_escalation_state (
    id                    TEXT PRIMARY KEY,
    last_digest_at        DATETIME,
    watermark_occurred_at DATETIME,
    updated_at            DATETIME NOT NULL
);
