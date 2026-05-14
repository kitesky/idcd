-- name: GetMonitorByID :one
SELECT * FROM monitors
WHERE id = $1 AND status != 'archived';

-- name: ListMonitorsByUser :many
SELECT * FROM monitors
WHERE user_id = $1 AND status != 'archived'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActiveMonitorsDue :many
SELECT * FROM monitors
WHERE status = 'active' AND next_check_at <= NOW()
ORDER BY next_check_at ASC
LIMIT 100;

-- name: CreateMonitor :one
INSERT INTO monitors (id, user_id, name, type, target, config, interval_s, node_count, status, next_check_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', NOW())
RETURNING *;

-- name: UpdateMonitorStatus :one
UPDATE monitors
SET status = $2, updated_at = NOW()
WHERE id = $1 AND status != 'archived'
RETURNING *;

-- name: UpdateMonitorNextCheck :exec
UPDATE monitors
SET last_check_at = NOW(), next_check_at = NOW() + make_interval(secs => $2), updated_at = NOW()
WHERE id = $1;

-- name: DeleteMonitor :exec
UPDATE monitors
SET status = 'archived', updated_at = NOW()
WHERE id = $1 AND status != 'archived';
