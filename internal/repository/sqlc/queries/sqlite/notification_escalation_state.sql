-- name: GetNotificationEscalationState :one
SELECT id, last_digest_at, watermark_occurred_at, updated_at
FROM notification_escalation_state
WHERE id = sqlc.arg('id');

-- name: UpsertNotificationEscalationState :exec
INSERT INTO notification_escalation_state (id, last_digest_at, watermark_occurred_at, updated_at)
VALUES (sqlc.arg('id'), sqlc.arg('last_digest_at'), sqlc.arg('watermark_occurred_at'), sqlc.arg('updated_at'))
ON CONFLICT (id) DO UPDATE SET
    last_digest_at = excluded.last_digest_at,
    watermark_occurred_at = excluded.watermark_occurred_at,
    updated_at = excluded.updated_at;
