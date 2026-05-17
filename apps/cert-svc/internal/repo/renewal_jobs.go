package repo

import (
	"context"
	"fmt"
	"time"
)

// RenewalJobsRepo handles cert.renewal_jobs — the queue of scheduled
// renewal attempts. Workers pull queued jobs in scheduled_at order.
type RenewalJobsRepo struct {
	pool Pool
}

const renewalJobsColumns = `id, cert_id, scheduled_at, attempt_count, last_error,
	status, new_order_id, created_at`

const renewalJobsInsertSQL = `
	INSERT INTO cert.renewal_jobs (cert_id, scheduled_at, status)
	VALUES ($1, $2, $3)
	RETURNING id, created_at
`

// Insert enqueues a renewal job. attempt_count starts at 0; the worker
// bumps it via IncrementAttempt on each pull.
func (r *RenewalJobsRepo) Insert(ctx context.Context, j *RenewalJob) (int64, error) {
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, renewalJobsInsertSQL,
		j.CertID,
		j.ScheduledAt,
		j.Status,
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("renewal_jobs insert: %w", err)
	}
	j.ID = id
	j.CreatedAt = createdAt
	return id, nil
}

const renewalJobsListQueuedSQL = `
	SELECT ` + renewalJobsColumns + `
	FROM cert.renewal_jobs
	WHERE status = 'queued' AND scheduled_at <= NOW()
	ORDER BY scheduled_at ASC
	LIMIT $1
`

// ListQueued returns jobs whose scheduled_at has passed and whose status
// is 'queued', ordered ASC. Workers MUST UpdateStatus to 'in_flight' (or
// similar) immediately to avoid duplicate pulls; this layer does not
// implement SELECT … FOR UPDATE SKIP LOCKED on purpose — the service
// layer wraps the call in a transaction when it needs that semantic.
func (r *RenewalJobsRepo) ListQueued(ctx context.Context, limit int) ([]*RenewalJob, error) {
	rows, err := r.pool.Query(ctx, renewalJobsListQueuedSQL, limit)
	if err != nil {
		return nil, fmt.Errorf("renewal_jobs list queued: %w", err)
	}
	defer rows.Close()

	out := make([]*RenewalJob, 0)
	for rows.Next() {
		var j RenewalJob
		if err := rows.Scan(
			&j.ID,
			&j.CertID,
			&j.ScheduledAt,
			&j.AttemptCount,
			&j.LastError,
			&j.Status,
			&j.NewOrderID,
			&j.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("renewal_jobs list scan: %w", err)
		}
		out = append(out, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("renewal_jobs list rows: %w", err)
	}
	return out, nil
}

const renewalJobsUpdateStatusSQL = `
	UPDATE cert.renewal_jobs
	SET status = $1, last_error = $2, new_order_id = $3
	WHERE id = $4
`

// UpdateStatus writes the new status + optional last_error + optional
// new_order_id. lastError == nil and newOrderID == nil clear those
// columns. Returns ErrNotFound when the row id does not exist.
func (r *RenewalJobsRepo) UpdateStatus(ctx context.Context, id int64, status string, lastError *string, newOrderID *int64) error {
	tag, err := r.pool.Exec(ctx, renewalJobsUpdateStatusSQL, status, lastError, newOrderID, id)
	if err != nil {
		return fmt.Errorf("renewal_jobs update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const renewalJobsIncrementAttemptSQL = `
	UPDATE cert.renewal_jobs
	SET attempt_count = attempt_count + 1
	WHERE id = $1
`

// IncrementAttempt bumps attempt_count by 1.
func (r *RenewalJobsRepo) IncrementAttempt(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, renewalJobsIncrementAttemptSQL, id)
	if err != nil {
		return fmt.Errorf("renewal_jobs increment attempt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
