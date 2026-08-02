-- name: GetHostAlertState :one
SELECT host_id, state, offline_since, alerted, updated_at
FROM host_alert_state
WHERE host_id = sqlc.arg('host_id');

-- name: UpsertHostAlertState :exec
INSERT INTO host_alert_state (host_id, state, offline_since, alerted, updated_at)
VALUES (sqlc.arg('host_id'), sqlc.arg('state'), sqlc.arg('offline_since'), sqlc.arg('alerted'), sqlc.arg('updated_at'))
ON CONFLICT (host_id) DO UPDATE SET
    state = excluded.state,
    offline_since = excluded.offline_since,
    alerted = excluded.alerted,
    updated_at = excluded.updated_at;
