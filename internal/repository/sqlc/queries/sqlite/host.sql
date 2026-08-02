-- name: CreateHost :exec
INSERT INTO hosts (
    id, created_at, updated_at, name, os, agent_version,
    last_seen_at, last_cpu_pct, last_mem_pct, last_disk_pct,
    last_net_in, last_net_out, last_disks
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindHostByID :one
SELECT * FROM hosts WHERE id = ?;

-- name: ListHosts :many
SELECT * FROM hosts
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: CountHosts :one
SELECT COUNT(*) FROM hosts;

-- name: DeleteHost :execrows
DELETE FROM hosts WHERE id = ?;

-- name: UpdateHostSnapshot :execrows
UPDATE hosts
SET last_seen_at  = ?2,
    last_cpu_pct  = ?3,
    last_mem_pct  = ?4,
    last_disk_pct = ?5,
    last_net_in   = ?6,
    last_net_out  = ?7,
    last_disks    = ?8,
    os            = COALESCE(?9, os),
    agent_version = COALESCE(?10, agent_version),
    updated_at    = ?11
WHERE id = ?1;
