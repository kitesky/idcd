package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// observation is the minimal probe data point the orchestrator's
// cross-validation and PDF rendering consume. The shape is package
// private and may be expanded without touching callers.
type observation struct {
	NodeID    string
	Timestamp time.Time
	Latency   time.Duration
	OK        bool
}

// observationPool is the narrow pgx surface fetchObservations exercises.
// Both *pgxpool.Pool and pgxmock.PgxPoolIface satisfy it, so the
// production path and unit tests share one call site.
type observationPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Compile-time guarantee: *pgxpool.Pool satisfies observationPool.
var _ observationPool = (*pgxpool.Pool)(nil)

// observationPoolState holds the lazily initialised pool plus an error
// captured from the first init attempt. A successful init wins; if any
// init returns an error it is returned to every caller until the
// singleton is replaced (e.g. by setObservationPool in tests).
var (
	observationPoolOnce sync.Once
	observationPoolVar  observationPool
	observationPoolErr  error
	observationPoolMu   sync.RWMutex
)

// getObservationPool returns the lazily initialised pool, or the test
// override if one was installed via setObservationPool. The DSN comes
// from IDCD_MAIN_DB_DSN (the main / TimescaleDB DB) and falls back to
// ATTEST_DB_DSN for single-node dev where both schemas live in the same
// physical DB. Both unset → clear error.
func getObservationPool(ctx context.Context) (observationPool, error) {
	observationPoolMu.RLock()
	if observationPoolVar != nil || observationPoolErr != nil {
		p, e := observationPoolVar, observationPoolErr
		observationPoolMu.RUnlock()
		if p != nil {
			return p, nil
		}
		return nil, e
	}
	observationPoolMu.RUnlock()

	observationPoolOnce.Do(func() {
		dsn := os.Getenv("IDCD_MAIN_DB_DSN")
		if dsn == "" {
			dsn = os.Getenv("ATTEST_DB_DSN")
		}
		if dsn == "" {
			observationPoolMu.Lock()
			observationPoolErr = errors.New("fetchObservations: neither IDCD_MAIN_DB_DSN nor ATTEST_DB_DSN is set")
			observationPoolMu.Unlock()
			return
		}
		pool, err := pgxpool.New(ctx, dsn)
		observationPoolMu.Lock()
		if err != nil {
			observationPoolErr = fmt.Errorf("fetchObservations: pgxpool.New: %w", err)
		} else {
			observationPoolVar = pool
		}
		observationPoolMu.Unlock()
	})

	observationPoolMu.RLock()
	defer observationPoolMu.RUnlock()
	if observationPoolVar != nil {
		return observationPoolVar, nil
	}
	return nil, observationPoolErr
}

// setObservationPool installs an explicit pool, bypassing the lazy DSN
// init. Package-private; intended for tests in the same package.
func setObservationPool(p observationPool) {
	observationPoolMu.Lock()
	defer observationPoolMu.Unlock()
	observationPoolVar = p
	observationPoolErr = nil
	// Mark the sync.Once as fired so a later real init does not
	// overwrite the test pool mid-test.
	observationPoolOnce.Do(func() {})
}

// resetObservationPoolForTest restores the package-level singleton to a
// pristine state so a subsequent test can exercise the lazy init.
func resetObservationPoolForTest() {
	observationPoolMu.Lock()
	defer observationPoolMu.Unlock()
	observationPoolVar = nil
	observationPoolErr = nil
	observationPoolOnce = sync.Once{}
}

// nodeResult is the per-node payload nested inside monitor_check.node_results.
// Only the fields fetchObservations consumes are decoded; extra fields
// in the jsonb (RTT distribution, headers, etc.) are ignored.
type nodeResult struct {
	NodeID    string `json:"node_id"`
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
}

// fetchObservations pulls raw probe points for one Verdict order out of
// idcd_main.monitor_check. Per D1 this is a cross-schema READ — fine, no
// FK is created. Per docs/prd/18 §3.1 step 1, empty results signal a
// REJECT_REFUND and surface as a typed error; the orchestrator wraps it
// into failPipeline → SetFailed.
//
// Empty-result semantics: returns (nil, error). The error mentions the
// target + window so logs / refund emails can quote it back to the user.
func fetchObservations(ctx context.Context, order *Order) ([]observation, error) {
	if order == nil {
		return nil, fmt.Errorf("fetchObservations: nil order")
	}

	pool, err := getObservationPool(ctx)
	if err != nil {
		return nil, err
	}

	// Single statement: monitor_check joined to monitor on monitor_id,
	// scoped by owner_id + target + time window. idx_monitor_check_monitor_time
	// + idx_monitor_target keep this cheap even for active accounts.
	//
	// We project only node_results and started_at because the orchestrator
	// flattens node_results into per-node observations below. Hypertable
	// reads honour the started_at bound; ORDER BY is delegated to Go since
	// we still have to flatten + re-sort by per-node timestamp anyway.
	const q = `
		SELECT mc.started_at, mc.node_results
		FROM idcd_main.monitor_check mc
		JOIN idcd_main.monitor m ON m.id = mc.monitor_id
		WHERE m.owner_id = $1
		  AND m.target = $2
		  AND mc.started_at >= $3
		  AND mc.started_at <= $4
	`
	rows, err := pool.Query(ctx, q, order.OwnerID, order.Target, order.TimeWindowStart, order.TimeWindowEnd)
	if err != nil {
		return nil, fmt.Errorf("fetchObservations: query monitor_check: %w", err)
	}
	defer rows.Close()

	out := make([]observation, 0, 16)
	for rows.Next() {
		var startedAt time.Time
		var raw []byte
		if err := rows.Scan(&startedAt, &raw); err != nil {
			return nil, fmt.Errorf("fetchObservations: scan monitor_check row: %w", err)
		}
		var nodes []nodeResult
		if err := json.Unmarshal(raw, &nodes); err != nil {
			return nil, fmt.Errorf("fetchObservations: decode node_results json (started_at=%s): %w",
				startedAt.Format(time.RFC3339Nano), err)
		}
		for _, n := range nodes {
			out = append(out, observation{
				NodeID:    n.NodeID,
				Timestamp: startedAt.UTC(),
				Latency:   time.Duration(n.LatencyMS) * time.Millisecond,
				OK:        n.OK,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetchObservations: iterate monitor_check rows: %w", err)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("fetchObservations: no data for target %s in window %s..%s",
			order.Target,
			order.TimeWindowStart.UTC().Format(time.RFC3339Nano),
			order.TimeWindowEnd.UTC().Format(time.RFC3339Nano))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Timestamp.Equal(out[j].Timestamp) {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}
