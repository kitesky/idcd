package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
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

// ObservationPool is the narrow pgx surface fetchObservations exercises
// against the idcd_main schema (cross-schema READ, allowed by D1 — no
// FK is created across schemas; reads through Repository are fine).
//
// Both *pgxpool.Pool and pgxmock.PgxPoolIface satisfy it, so production
// and unit tests share one call site. The interface is exported so
// callers in cmd/generator and integration tests can pass their own
// implementations without touching internals.
type ObservationPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Compile-time guarantee: *pgxpool.Pool satisfies ObservationPool.
var _ ObservationPool = (*pgxpool.Pool)(nil)

// ErrObservationPoolNotConfigured signals that fetchObservations was
// invoked on a Service whose Config.Observations was nil. cmd/generator
// is expected to wire a real pool at process start; tests that never
// reach fetchObservations may legitimately leave it nil, but reaching
// the pipeline without one is a wiring bug we surface as a typed error
// rather than a nil-pointer panic.
var ErrObservationPoolNotConfigured = errors.New("attest/service: ObservationPool is not configured")

// NewObservationPoolFromDSN constructs a pgxpool.Pool against the
// idcd_main TimescaleDB using the provided DSN. The returned pool
// satisfies ObservationPool. Callers (cmd/generator) own the pool's
// lifecycle and MUST Close it on shutdown.
//
// Empty dsn returns ErrObservationPoolNotConfigured so the caller can
// decide whether to abort startup or run in a degraded mode.
func NewObservationPoolFromDSN(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, fmt.Errorf("%w: dsn is empty", ErrObservationPoolNotConfigured)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("attest/service: pgxpool.New: %w", err)
	}
	return pool, nil
}

// NewObservationPoolFromEnv constructs a pgxpool.Pool against the
// idcd_main TimescaleDB, picking the DSN from IDCD_MAIN_DB_DSN with
// fallback to ATTEST_DB_DSN (single-node dev). The returned pool
// satisfies ObservationPool. Callers (cmd/generator) own the pool's
// lifecycle and MUST Close it on shutdown — there is no longer a
// process-singleton.
//
// Prefer NewObservationPoolFromDSN when the DSN is already available
// from the loaded config (P1-8 migration).
//
// Both env vars unset returns ErrObservationPoolNotConfigured so the
// caller can decide whether to abort startup or run in a degraded mode
// that skips verdict generation.
func NewObservationPoolFromEnv(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("IDCD_MAIN_DB_DSN")
	if dsn == "" {
		dsn = os.Getenv("ATTEST_DB_DSN")
	}
	return NewObservationPoolFromDSN(ctx, dsn)
}

// fetchObservations pulls raw probe points for one Verdict order out of
// idcd_main.monitor_check. Per D1 this is a cross-schema READ — fine, no
// FK is created. Per docs/prd/18 §3.1 step 1, empty results signal a
// REJECT_REFUND and surface as a typed error; the orchestrator wraps it
// into failPipeline → SetFailed.
//
// Empty-result semantics: returns (nil, error). The error mentions the
// target + window so logs / refund emails can quote it back to the user.
//
// Receives the pool as a parameter rather than reading a package
// singleton; this lets the service.Service hold the pool as an injected
// dependency, enabling parallel tests + graceful Close() on shutdown.
func fetchObservations(ctx context.Context, pool ObservationPool, order *Order) ([]observation, error) {
	if order == nil {
		return nil, fmt.Errorf("fetchObservations: nil order")
	}
	if pool == nil {
		return nil, ErrObservationPoolNotConfigured
	}

	// Naming note: the v2 spec (D1) places monitors / monitor_checks under
	// the idcd_main schema with singular names + a nested node_results jsonb
	// column, but the live deployment uses default-schema plural names with
	// one row per (monitor, node, check_at). We query the live shape here.
	//
	// Per-node row → observation directly (no jsonb flatten). check_at /
	// latency_ms / status come straight off the row. OK is derived from
	// status='up' to match the legacy nodeResult.OK semantics.
	const q = `
		SELECT mc.check_at, mc.node_id, COALESCE(mc.latency_ms, 0), mc.status
		FROM monitor_checks mc
		JOIN monitors m ON m.id = mc.monitor_id
		WHERE m.user_id = $1
		  AND m.target = $2
		  AND mc.check_at >= $3
		  AND mc.check_at <= $4
	`
	rows, err := pool.Query(ctx, q, order.OwnerID, order.Target, order.TimeWindowStart, order.TimeWindowEnd)
	if err != nil {
		return nil, fmt.Errorf("fetchObservations: query monitor_checks: %w", err)
	}
	defer rows.Close()

	out := make([]observation, 0, 16)
	for rows.Next() {
		var (
			checkAt   time.Time
			nodeID    string
			latencyMS int64
			status    string
		)
		if err := rows.Scan(&checkAt, &nodeID, &latencyMS, &status); err != nil {
			return nil, fmt.Errorf("fetchObservations: scan monitor_checks row: %w", err)
		}
		out = append(out, observation{
			NodeID:    nodeID,
			Timestamp: checkAt.UTC(),
			Latency:   time.Duration(latencyMS) * time.Millisecond,
			OK:        status == "up",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fetchObservations: iterate monitor_checks rows: %w", err)
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
