package service

import (
	"context"
	"fmt"
	"time"
)

// observation is the minimal probe data point the orchestrator's
// cross-validation and PDF rendering consume. The production
// implementation (S2 W6+) will replace fetchObservations with a real
// TimescaleDB query that returns a richer struct; everywhere
// observation is consumed in this package is internal, so we can expand
// the shape without touching callers.
type observation struct {
	NodeID    string
	Timestamp time.Time
	Latency   time.Duration
	OK        bool
}

// fetchObservations stubs the TimescaleDB read in step 1 of the
// pipeline. It returns three fixed probe points so downstream stages
// get realistic-looking input. The function takes the order so the
// future real implementation can scope the query by target / time
// window without a signature change.
func fetchObservations(_ context.Context, order *Order) ([]observation, error) {
	if order == nil {
		return nil, fmt.Errorf("fetchObservations: nil order")
	}
	now := time.Now().UTC().Truncate(time.Second)
	return []observation{
		{NodeID: "node-cn-bj", Timestamp: now.Add(-3 * time.Second), Latency: 42 * time.Millisecond, OK: true},
		{NodeID: "node-cn-sh", Timestamp: now.Add(-2 * time.Second), Latency: 51 * time.Millisecond, OK: true},
		{NodeID: "node-cn-gz", Timestamp: now.Add(-1 * time.Second), Latency: 47 * time.Millisecond, OK: true},
	}, nil
}
