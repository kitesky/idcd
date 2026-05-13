-- name: CreateAPIKey :one
INSERT INTO api_key (id, owner_type, owner_id, name, prefix, secret_hash, scopes, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAPIKeyByID :one
SELECT * FROM api_key WHERE id = $1 AND status = 'active';

-- name: GetAPIKeyByPrefix :one
SELECT * FROM api_key WHERE prefix = $1 AND status = 'active';

-- name: ListAPIKeysByOwner :many
SELECT * FROM api_key
WHERE owner_type = $1 AND owner_id = $2 AND status = 'active'
ORDER BY created_at DESC;

-- name: RevokeAPIKey :exec
UPDATE api_key
SET status = 'revoked', revoked_at = now()
WHERE id = $1 AND status = 'active';

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_key
SET last_used_at = now(), last_used_ip = $2, usage_total = usage_total + 1
WHERE id = $1;

-- name: ExpireAPIKey :exec
UPDATE api_key
SET status = 'expired'
WHERE expires_at IS NOT NULL AND expires_at < now() AND status = 'active';
