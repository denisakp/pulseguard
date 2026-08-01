-- name: CreateHostCredential :exec
INSERT INTO host_credentials (
    id, created_at, updated_at, host_id, hash, prefix, is_active, last_used_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: FindActiveHostCredentialByHash :one
SELECT * FROM host_credentials
WHERE hash = $1 AND is_active = TRUE;

-- name: ListHostCredentialsByHost :many
SELECT * FROM host_credentials
WHERE host_id = $1
ORDER BY created_at DESC;

-- name: DeactivateHostCredentialByID :execrows
UPDATE host_credentials SET is_active = FALSE, updated_at = $2 WHERE id = $1;

-- name: DeactivateAllHostCredentialsForHost :exec
UPDATE host_credentials SET is_active = FALSE, updated_at = $2 WHERE host_id = $1 AND is_active = TRUE;

-- name: TouchHostCredentialLastUsed :exec
UPDATE host_credentials SET last_used_at = $2 WHERE id = $1;

-- name: DeleteHostCredentialsByHost :exec
DELETE FROM host_credentials WHERE host_id = $1;
