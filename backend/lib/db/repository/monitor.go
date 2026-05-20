package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// MonitorRepository wraps sqlc Queries for monitor domain operations.
type MonitorRepository struct {
	q *idcdmain.Queries
}

// NewMonitorRepository returns a MonitorRepository backed by the given pool.
func NewMonitorRepository(pool *pgxpool.Pool) *MonitorRepository {
	return &MonitorRepository{q: idcdmain.New(pool)}
}

// GetByID returns a monitor by ID (excludes archived).
func (r *MonitorRepository) GetByID(ctx context.Context, id string) (idcdmain.Monitor, error) {
	m, err := r.q.GetMonitorByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.Monitor{}, ErrNotFound
		}
		return idcdmain.Monitor{}, fmt.Errorf("monitor.GetByID: %w", err)
	}
	return m, nil
}

// ListByUser returns paginated monitors for a user (excludes archived).
func (r *MonitorRepository) ListByUser(ctx context.Context, userID string, limit, offset int32) ([]idcdmain.Monitor, error) {
	ms, err := r.q.ListMonitorsByUser(ctx, idcdmain.ListMonitorsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("monitor.ListByUser: %w", err)
	}
	return ms, nil
}

// ListActiveMonitorsDue returns monitors that are due for their next check.
func (r *MonitorRepository) ListActiveMonitorsDue(ctx context.Context) ([]idcdmain.Monitor, error) {
	ms, err := r.q.ListActiveMonitorsDue(ctx)
	if err != nil {
		return nil, fmt.Errorf("monitor.ListActiveMonitorsDue: %w", err)
	}
	return ms, nil
}

// Create inserts a new monitor and returns it.
func (r *MonitorRepository) Create(ctx context.Context, p idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
	m, err := r.q.CreateMonitor(ctx, p)
	if err != nil {
		if isDuplicate(err) {
			return idcdmain.Monitor{}, ErrDuplicate
		}
		return idcdmain.Monitor{}, fmt.Errorf("monitor.Create: %w", err)
	}
	return m, nil
}

// UpdateStatus sets the monitor's status field.
func (r *MonitorRepository) UpdateStatus(ctx context.Context, id, status string) (idcdmain.Monitor, error) {
	m, err := r.q.UpdateMonitorStatus(ctx, idcdmain.UpdateMonitorStatusParams{ID: id, Status: status})
	if err != nil {
		if isNoRows(err) {
			return idcdmain.Monitor{}, ErrNotFound
		}
		return idcdmain.Monitor{}, fmt.Errorf("monitor.UpdateStatus: %w", err)
	}
	return m, nil
}

// UpdateNextCheck advances last_check_at and next_check_at by intervalSeconds.
func (r *MonitorRepository) UpdateNextCheck(ctx context.Context, id string, intervalSeconds float64) error {
	if err := r.q.UpdateMonitorNextCheck(ctx, idcdmain.UpdateMonitorNextCheckParams{
		ID:   id,
		Secs: intervalSeconds,
	}); err != nil {
		return fmt.Errorf("monitor.UpdateNextCheck: %w", err)
	}
	return nil
}

// Archive soft-deletes a monitor by setting status='archived'.
func (r *MonitorRepository) Archive(ctx context.Context, id string) error {
	if err := r.q.DeleteMonitor(ctx, id); err != nil {
		return fmt.Errorf("monitor.Archive: %w", err)
	}
	return nil
}
