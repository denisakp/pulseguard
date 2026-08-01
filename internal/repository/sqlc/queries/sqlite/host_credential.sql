-- name: CreateHostCredential :exec
INSERT INTO host_credentials (
    id, created_at, updated_at, host_id, hash, prefix, is_active, last_used_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindActiveHostCredentialByHash :one
SELECT * FROM host_credentials
WHERE hash = ? AND is_active = 1;

-- name: ListHostCredentialsByHost :many
SELECT * FROM host_credentials
WHERE host_id = ?
ORDER BY created_at DESC;

-- name: DeactivateHostCredentialByID :execrows
UPDATE host_credentials SET is_active = 0, updated_at = ?2 WHERE id = ?1;

-- name: DeactivateAllHostCredentialsForHost :exec
UPDATE host_credentials SET is_active = 0, updated_at = ?2 WHERE host_id = ?1 AND is_active = 1;

-- name: TouchHostCredentialLastUsed :exec
UPDATE host_credentials SET last_used_at = ?2 WHERE id = ?1;

-- name: DeleteHostCredentialsByHost :exec
DELETE FROM host_credentials WHERE host_id = ?;
