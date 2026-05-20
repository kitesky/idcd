-- name: GetUserByID :one
SELECT * FROM "user"
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM "user"
WHERE email = $1 AND deleted_at IS NULL;

-- name: GetUserByUsername :one
SELECT * FROM "user"
WHERE username = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO "user" (id, email, password_hash, display_name, locale, timezone)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE "user"
SET status = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserEmailVerified :one
UPDATE "user"
SET email_verified_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE "user"
SET last_login_at = now(), last_login_ip = $2
WHERE id = $1;

-- name: UpdateUserProfile :one
UPDATE "user"
SET display_name = $2, avatar_url = $3, bio = $4, locale = $5, timezone = $6
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserPasswordHash :exec
UPDATE "user"
SET password_hash = $2, password_changed_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteUser :exec
UPDATE "user"
SET deleted_at = now(), status = 'pending_deletion', pending_deletion_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateUserCredential :one
INSERT INTO user_credential (id, user_id, type, external_id, metadata)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserCredentialByTypeAndExternal :one
SELECT * FROM user_credential
WHERE type = $1 AND external_id = $2;

-- name: ListUserCredentialsByUser :many
SELECT * FROM user_credential WHERE user_id = $1;

-- name: CreateUserOTP :one
INSERT INTO user_otp (id, user_id, type, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserOTPByIDAndType :one
SELECT * FROM user_otp
WHERE id = $1 AND type = $2 AND used_at IS NULL AND expires_at > now();

-- name: MarkUserOTPUsed :exec
UPDATE user_otp SET used_at = now() WHERE id = $1;

-- name: IncrementUserOTPAttempts :exec
UPDATE user_otp SET attempts = attempts + 1 WHERE id = $1;
