-- PR1 of US1: CRUD without M2M and without 1-to-1 preloads.
-- Mirror of postgres/resource.sql with SQLite-specific date math for
-- FindMissedHeartbeats (strftime instead of EXTRACT EPOCH).

-- name: CreateResource :exec
INSERT INTO resources (
    id, created_at, updated_at, name, type, interval, timeout, target,
    last_checked, status, is_active, failure_count,
    ssl_expiration_date, ssl_issuer, domain_expiration_date, domain_registrar,
    component_id, confirmation_checks, confirmation_interval, expiry_alert_thresholds,
    flap_detection_enabled, flap_threshold, flap_window_seconds, flap_max_duration_minutes,
    last_status_transition, flap_started_at, reminder_interval_minutes,
    heartbeat_slug, heartbeat_interval, heartbeat_grace, last_ping_at,
    keyword, keyword_mode, protocol_type, protocol_port,
    host_id
)
VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?
);

-- name: FindResourceByID :one
SELECT * FROM resources WHERE id = ? AND is_active = 1;

-- name: FindResourceByHeartbeatSlug :one
SELECT * FROM resources
WHERE heartbeat_slug = ?
  AND type = 'heartbeat'
  AND is_active = 1;

-- name: ListResources :many
SELECT * FROM resources
WHERE is_active = 1
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListActiveResources :many
SELECT * FROM resources
WHERE is_active = 1
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListScheduledResources :many
SELECT * FROM resources
WHERE is_active = 1
ORDER BY id ASC;

-- name: FindResourcesByComponentID :many
SELECT * FROM resources
WHERE component_id = ? AND is_active = 1
ORDER BY created_at DESC;

-- name: CountResourcesByComponentID :one
SELECT COUNT(*) FROM resources
WHERE component_id = ? AND is_active = 1;

-- name: UpdateResourceMain :execrows
UPDATE resources
SET name                      = ?2,
    type                      = ?3,
    target                    = ?4,
    interval                  = ?5,
    timeout                   = ?6,
    is_active                 = ?7,
    confirmation_checks       = ?8,
    confirmation_interval     = ?9,
    component_id              = ?10,
    expiry_alert_thresholds   = ?11,
    flap_detection_enabled    = ?12,
    flap_threshold            = ?13,
    flap_window_seconds       = ?14,
    flap_max_duration_minutes = ?15,
    reminder_interval_minutes = ?16,
    heartbeat_interval        = ?17,
    heartbeat_grace           = ?18,
    updated_at                = ?19
WHERE id = ?1;

-- name: SoftDeleteResource :execrows
UPDATE resources SET is_active = 0
WHERE id = ? AND is_active = 1;

-- name: UpdateResourceStatus :execrows
UPDATE resources SET status = ?2 WHERE id = ?1;

-- name: UpdateResourceLastPingAt :execrows
UPDATE resources SET last_ping_at = ?2
WHERE id = ?1 AND type = 'heartbeat' AND is_active = 1;

-- M2M: resource_tags ---------------------------------------------------------

-- name: LinkResourceTag :exec
INSERT INTO resource_tags (resource_id, tag_id) VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: UnlinkResourceTag :exec
DELETE FROM resource_tags WHERE resource_id = ? AND tag_id = ?;

-- name: ListTagIDsByResourceID :many
SELECT tag_id FROM resource_tags WHERE resource_id = ?;

-- name: ListTagsByResourceIDs :many
SELECT rt.resource_id, t.id, t.name, t.color, t.description, t.created_at, t.updated_at
FROM resource_tags rt
JOIN tags t ON rt.tag_id = t.id
WHERE rt.resource_id IN (sqlc.slice('resource_ids'));

-- M2M: resource_notification_channels ---------------------------------------

-- name: LinkResourceChannel :exec
INSERT INTO resource_notification_channels (resource_id, notification_channel_id)
VALUES (?, ?)
ON CONFLICT DO NOTHING;

-- name: UnlinkResourceChannel :exec
DELETE FROM resource_notification_channels
WHERE resource_id = ? AND notification_channel_id = ?;

-- name: ListChannelIDsByResourceID :many
SELECT notification_channel_id FROM resource_notification_channels
WHERE resource_id = ?;

-- name: ListChannelsByResourceIDs :many
SELECT rnc.resource_id,
       nc.id, nc.name, nc.type, nc.config, nc.enabled_by_default,
       nc.created_at, nc.updated_at
FROM resource_notification_channels rnc
JOIN notification_channels nc ON rnc.notification_channel_id = nc.id
WHERE rnc.resource_id IN (sqlc.slice('resource_ids'));

-- 1-to-1 preloads ------------------------------------------------------------

-- name: ListComponentsByIDs :many
SELECT id, created_at, updated_at, name, description,
       last_notification_status, grouping_window_seconds
FROM components
WHERE id IN (sqlc.slice('ids'));

-- name: ListCredentialsByResourceIDs :many
SELECT id, resource_id, username, password, options, created_at, updated_at
FROM resource_credentials
WHERE resource_id IN (sqlc.slice('resource_ids'));

-- name: FindResourceIDsByTagName :many
SELECT r.id
FROM resources r
JOIN resource_tags rt ON r.id = rt.resource_id
JOIN tags t ON rt.tag_id = t.id
WHERE t.name = ? AND r.is_active = 1
ORDER BY r.created_at DESC
LIMIT ? OFFSET ?;

-- name: FindResourcesByIDs :many
SELECT * FROM resources WHERE id IN (sqlc.slice('ids'));

-- name: FindMissedHeartbeatsSQLite :many
SELECT * FROM resources
WHERE type = 'heartbeat'
  AND status = 'up'
  AND is_active = 1
  AND last_ping_at IS NOT NULL
  AND (CAST(strftime('%s', substr(last_ping_at, 1, 19)) AS INTEGER) + heartbeat_interval + heartbeat_grace) < CAST(sqlc.arg('now_unix') AS INTEGER)
ORDER BY last_ping_at ASC
LIMIT CAST(sqlc.arg('row_limit') AS INTEGER);

-- name: SetResourceHostID :execrows
UPDATE resources SET host_id = ?, updated_at = ? WHERE id = ?;

-- name: ClearResourceHostIDByHost :exec
UPDATE resources SET host_id = NULL WHERE host_id = ?;
