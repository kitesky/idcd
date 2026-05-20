-- status_page.sql — sqlc queries for status_pages custom domain management
-- Run `sqlc generate` after migration 00011 has been applied.

-- name: GetStatusPageByCustomDomain :one
SELECT * FROM status_pages WHERE custom_domain = $1;

-- name: SetStatusPageCustomDomain :one
UPDATE status_pages
SET custom_domain = $2,
    custom_domain_verified_at = NULL,
    custom_domain_cert_expires_at = NULL,
    updated_at = NOW()
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: MarkCustomDomainVerified :exec
UPDATE status_pages
SET custom_domain_verified_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: UpdateCertExpiry :exec
UPDATE status_pages
SET custom_domain_cert_expires_at = $2, updated_at = NOW()
WHERE custom_domain = $1;

-- name: GetStatusPageByID :one
SELECT * FROM status_pages WHERE id = $1;
