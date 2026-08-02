-- 0030: Unread-notification escalation state (spec 083). A single-row table
-- holding the high-water mark of the last emitted digest so the digest is not
-- re-sent while the unread set is unchanged, and survives restarts.
CREATE TABLE IF NOT EXISTS notification_escalation_state (
    id                    TEXT PRIMARY KEY,
    last_digest_at        TIMESTAMPTZ,
    watermark_occurred_at TIMESTAMPTZ,
    updated_at            TIMESTAMPTZ NOT NULL
);
