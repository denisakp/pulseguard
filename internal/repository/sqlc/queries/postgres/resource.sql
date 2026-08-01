-- PR1 of US1: CRUD without M2M and without 1-to-1 preloads.
-- Deferred to later PRs: FindByTag (M2M JOIN), dynamic UpdateMonitoringState /
-- UpdateMetadata, Tags / NotificationChannels / Component / Credential preloads.

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
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12,
    $13, $14, $15, $16,
    $17, $18, $19, $20,
    $21, $22, $23, $24,
    $25, $26, $27,
    $28, $29, $30, $31,
    $32, $33, $34, $35,
    $36
);

-- name: FindResourceByID :one
SELECT * FROM resources WHERE id = $1 AND is_active = TRUE;

-- name: FindResourceByHeartbeatSlug :one
SELECT * FROM resources
WHERE heartbeat_slug = $1
  AND type = 'heartbeat'
  AND is_active = TRUE;

-- name: ListResources :many
SELECT * FROM resources
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListActiveResources :many
SELECT * FROM resources
WHERE is_active = TRUE
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListScheduledResources :many
SELECT * FROM resources
WHERE is_active = TRUE
ORDER BY id ASC;

-- name: FindResourcesByComponentID :many
SELECT * FROM resources
WHERE component_id = $1 AND is_active = TRUE
ORDER BY created_at DESC;

-- name: CountResourcesByComponentID :one
SELECT COUNT(*) FROM resources
WHERE component_id = $1 AND is_active = TRUE;

-- name: UpdateResourceMain :execrows
UPDATE resources
SET name                      = $2,
    type                      = $3,
    target                    = $4,
    interval                  = $5,
    timeout                   = $6,
    is_active                 = $7,
    confirmation_checks       = $8,
    confirmation_interval     = $9,
    component_id              = $10,
    expiry_alert_thresholds   = $11,
    flap_detection_enabled    = $12,
    flap_threshold            = $13,
    flap_window_seconds       = $14,
    flap_max_duration_minutes = $15,
    reminder_interval_minutes = $16,
    heartbeat_interval        = $17,
    heartbeat_grace           = $18,
    updated_at                = $19
WHERE id = $1;

-- name: SoftDeleteResource :execrows
UPDATE resources SET is_active = FALSE
WHERE id = $1 AND is_active = TRUE;

-- name: UpdateResourceStatus :execrows
UPDATE resources SET status = $2 WHERE id = $1;

-- name: UpdateResourceLastPingAt :execrows
UPDATE resources SET last_ping_at = $2
WHERE id = $1 AND type = 'heartbeat' AND is_active = TRUE;

-- M2M: resource_tags ---------------------------------------------------------

-- name: LinkResourceTag :exec
INSERT INTO resource_tags (resource_id, tag_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkResourceTag :exec
DELETE FROM resource_tags WHERE resource_id = $1 AND tag_id = $2;

-- name: ListTagIDsByResourceID :many
SELECT tag_id FROM resource_tags WHERE resource_id = $1;

-- name: ListTagsByResourceIDs :many
SELECT rt.resource_id, t.id, t.name, t.color, t.description, t.created_at, t.updated_at
FROM resource_tags rt
JOIN tags t ON rt.tag_id = t.id
WHERE rt.resource_id = ANY($1::text[]);

-- name: FindResourcesByIDs :many
SELECT * FROM resources WHERE id = ANY($1::text[]);

-- M2M: resource_notification_channels ---------------------------------------

-- name: LinkResourceChannel :exec
INSERT INTO resource_notification_channels (resource_id, notification_channel_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkResourceChannel :exec
DELETE FROM resource_notification_channels
WHERE resource_id = $1 AND notification_channel_id = $2;

-- name: ListChannelIDsByResourceID :many
SELECT notification_channel_id FROM resource_notification_channels
WHERE resource_id = $1;

-- name: ListChannelsByResourceIDs :many
SELECT rnc.resource_id,
       nc.id, nc.name, nc.type, nc.config, nc.enabled_by_default,
       nc.created_at, nc.updated_at
FROM resource_notification_channels rnc
JOIN notification_channels nc ON rnc.notification_channel_id = nc.id
WHERE rnc.resource_id = ANY($1::text[]);

-- 1-to-1 preloads ------------------------------------------------------------

-- name: ListComponentsByIDs :many
SELECT id, created_at, updated_at, name, description,
       last_notification_status, grouping_window_seconds
FROM components
WHERE id = ANY($1::text[]);

-- name: ListCredentialsByResourceIDs :many
SELECT id, resource_id, username, password, options, created_at, updated_at
FROM resource_credentials
WHERE resource_id = ANY($1::text[]);

-- name: FindResourceIDsByTagName :many
SELECT r.id
FROM resources r
JOIN resource_tags rt ON r.id = rt.resource_id
JOIN tags t ON rt.tag_id = t.id
WHERE t.name = $1 AND r.is_active = TRUE
ORDER BY r.created_at DESC
LIMIT $2 OFFSET $3;

-- name: FindMissedHeartbeatsPG :many
SELECT * FROM resources
WHERE type = 'heartbeat'
  AND status = 'up'
  AND is_active = TRUE
  AND last_ping_at IS NOT NULL
  AND EXTRACT(EPOCH FROM last_ping_at) + heartbeat_interval + heartbeat_grace < sqlc.arg('now_unix')::double precision
ORDER BY last_ping_at ASC
LIMIT sqlc.arg('row_limit')::int;

-- name: SetResourceHostID :execrows
UPDATE resources SET host_id = $2, updated_at = $3 WHERE id = $1;

-- name: ClearResourceHostIDByHost :exec
UPDATE resources SET host_id = NULL WHERE host_id = $1;
