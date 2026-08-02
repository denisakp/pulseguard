-- name: CreateNotification :one
INSERT INTO notifications (id, user_id, category, severity, title, description, deep_link, payload, occurred_at, read_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListNotificationsForUser :many
SELECT * FROM notifications
WHERE (user_id IS NULL OR user_id = sqlc.arg('user_id'))
  AND (sqlc.narg('category') IS NULL OR category = sqlc.narg('category'))
ORDER BY occurred_at DESC
LIMIT sqlc.arg('lim') OFFSET sqlc.arg('off');

-- name: CountNotificationsForUser :one
SELECT COUNT(*) FROM notifications
WHERE (user_id IS NULL OR user_id = sqlc.arg('user_id'))
  AND (sqlc.narg('category') IS NULL OR category = sqlc.narg('category'));

-- name: MarkNotificationRead :execrows
UPDATE notifications
SET read_at = COALESCE(read_at, sqlc.arg('read_at')), updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id');

-- name: MarkAllNotificationsReadForUser :execrows
UPDATE notifications
SET read_at = sqlc.arg('read_at'), updated_at = sqlc.arg('updated_at')
WHERE read_at IS NULL
  AND occurred_at <= sqlc.arg('before_ts')
  AND (user_id IS NULL OR user_id = sqlc.arg('user_id'));

-- name: DeleteNotificationsOlderThan :execrows
DELETE FROM notifications WHERE occurred_at < sqlc.arg('cutoff');

-- name: ListUnreadForEscalation :many
SELECT * FROM notifications
WHERE read_at IS NULL
  AND user_id IS NULL
  AND occurred_at <= sqlc.arg('cutoff')
  AND category IN (sqlc.slice('categories'))
ORDER BY occurred_at DESC;

-- name: CountUnreadForEscalation :one
SELECT COUNT(*) FROM notifications
WHERE read_at IS NULL
  AND user_id IS NULL
  AND occurred_at <= sqlc.arg('cutoff')
  AND category IN (sqlc.slice('categories'));
