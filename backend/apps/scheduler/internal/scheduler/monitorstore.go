package scheduler

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kite365/idcd/lib/db/repository"
)

// PGMonitorStore is a MonitorStore backed by the shared MonitorRepository.
// It adapts repository.MonitorRepository (which returns idcdmain.Monitor) to the
// scheduler-local DueMonitor shape so monitorPoller can dispatch tasks.
type PGMonitorStore struct {
	repo *repository.MonitorRepository
}

// NewPGMonitorStore returns a PGMonitorStore wired to the given pgx pool.
func NewPGMonitorStore(pool *pgxpool.Pool) *PGMonitorStore {
	return &PGMonitorStore{repo: repository.NewMonitorRepository(pool)}
}

// ListActiveMonitorsDue translates idcdmain.Monitor rows into DueMonitor.
func (s *PGMonitorStore) ListActiveMonitorsDue(ctx context.Context) ([]DueMonitor, error) {
	ms, err := s.repo.ListActiveMonitorsDue(ctx)
	if err != nil {
		return nil, fmt.Errorf("monitorstore: %w", err)
	}
	out := make([]DueMonitor, 0, len(ms))
	for _, m := range ms {
		out = append(out, DueMonitor{
			ID:        m.ID,
			Type:      m.Type,
			Target:    m.Target,
			IntervalS: m.IntervalS,
			NodeCount: m.NodeCount,
			Config:    m.Config,
		})
	}
	return out, nil
}
