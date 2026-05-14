package scheduler

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/scheduler/internal/queue"
)

// DBNodeSelector selects an active enrolled node from the database.
// It replaces StaticNodeSelector for production use.
type DBNodeSelector struct {
	pool *pgxpool.Pool
}

// NewDBNodeSelector creates a DBNodeSelector backed by the given pool.
func NewDBNodeSelector(pool *pgxpool.Pool) *DBNodeSelector {
	return &DBNodeSelector{pool: pool}
}

// SelectNode returns a random active node from enrolled_nodes.
// Falls back to an error if no active nodes are available.
func (s *DBNodeSelector) SelectNode(ctx context.Context, _ *queue.ProbeTask) (string, error) {
	var nodeID string
	err := s.pool.QueryRow(ctx, `
		SELECT node_id
		FROM enrolled_nodes
		WHERE status = 'active'
		ORDER BY RANDOM()
		LIMIT 1
	`).Scan(&nodeID)
	if err != nil {
		return "", fmt.Errorf("DBNodeSelector: no active nodes available: %w", err)
	}
	return nodeID, nil
}
