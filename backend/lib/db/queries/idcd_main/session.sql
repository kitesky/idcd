-- name: CreateSession :one
INSERT INTO user_session (id, user_id, refresh_token_hash, device, client_ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM user_session
WHERE id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: GetSessionByTokenHash :one
SELECT * FROM user_session
WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: RevokeSession :exec
UPDATE user_session SET revoked_at = now() WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE user_session SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: ListActiveSessions :many
SELECT * FROM user_session
WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: PurgeExpiredSessions :exec
DELETE FROM user_session
WHERE expires_at < now() - INTERVAL '7 days';
