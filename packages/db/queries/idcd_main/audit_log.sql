-- name: CreateAuditLog :exec
INSERT INTO audit_log (id, ts, owner_id, actor_user_id, action, resource_type, resource_id, client_ip, user_agent, result, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListAuditLogsByOwner :many
SELECT * FROM audit_log
WHERE owner_id = $1 AND ts >= $2 AND ts < $3
ORDER BY ts DESC
LIMIT $4;

-- name: ListAuditLogsByActor :many
SELECT * FROM audit_log
WHERE actor_user_id = $1 AND ts >= $2 AND ts < $3
ORDER BY ts DESC
LIMIT $4;

-- name: ListAuditLogsByResource :many
SELECT * FROM audit_log
WHERE resource_type = $1 AND resource_id = $2
ORDER BY ts DESC
LIMIT $3;
