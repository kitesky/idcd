// Package scheduler provides scheduled background tasks for the Gateway service.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupScheduler periodically marks stale nodes as inactive.
type CleanupScheduler struct {
	pool     *pgxpool.Pool
	interval time.Duration
	logger   *slog.Logger
}

// NewCleanupScheduler creates a new CleanupScheduler.
func NewCleanupScheduler(pool *pgxpool.Pool, interval time.Duration, logger *slog.Logger) *CleanupScheduler {
	return &CleanupScheduler{
		pool:     pool,
		interval: interval,
		logger:   logger,
	}
}

// Run starts the cleanup scheduler loop.
// It marks nodes as 'inactive' if their last_seen_at timestamp is older than the interval.
// This should be run in a separate goroutine.
func (s *CleanupScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("cleanup scheduler started", "interval", s.interval)

	// Run immediately on start
	s.cleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("cleanup scheduler stopped")
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

// cleanup performs the actual cleanup operation.
func (s *CleanupScheduler) cleanup(ctx context.Context) {
	query := `
		UPDATE enrolled_nodes
		SET status = 'offline'
		WHERE last_seen_at < NOW() - $1::interval
		  AND status = 'active'
	`

	// Mark nodes as offline if they haven't been seen in the last interval
	result, err := s.pool.Exec(ctx, query, s.interval)
	if err != nil {
		s.logger.Error("failed to cleanup stale nodes", "error", err)
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		s.logger.Info("marked stale nodes as offline", "count", rowsAffected)
	}
}
