package scheduler

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/apps/scheduler/internal/queue"
)

// nodeQueryer is the minimal DB interface required by DBNodeSelector.
// *pgxpool.Pool satisfies this; pgxmock.PgxPoolIface satisfies it in tests.
type nodeQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DBNodeSelector selects an active enrolled node from the database.
// It replaces StaticNodeSelector for production use.
type DBNodeSelector struct {
	db nodeQueryer
}

// NewDBNodeSelector creates a DBNodeSelector backed by the given pool.
func NewDBNodeSelector(pool *pgxpool.Pool) *DBNodeSelector {
	return &DBNodeSelector{db: pool}
}

// NewDBNodeSelectorFromQuerier creates a DBNodeSelector from any nodeQueryer.
// Intended for testing with pgxmock.
func NewDBNodeSelectorFromQuerier(q nodeQueryer) *DBNodeSelector {
	return &DBNodeSelector{db: q}
}

// SelectNode returns a random active node from enrolled_nodes.
// Returns an error if no active nodes are available.
func (s *DBNodeSelector) SelectNode(ctx context.Context, _ *queue.ProbeTask) (string, error) {
	var nodeID string
	err := s.db.QueryRow(ctx, `
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
