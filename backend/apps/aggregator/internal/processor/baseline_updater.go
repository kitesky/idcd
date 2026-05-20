package processor

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// BaselineUpdater recalculates monitor baselines periodically.
type BaselineUpdater struct {
	pool pgxQuerier
}

func newBaselineUpdater(pool *pgxpool.Pool) *BaselineUpdater {
	if pool == nil {
		return &BaselineUpdater{}
	}
	return &BaselineUpdater{pool: &poolQuerier{pool: pool}}
}

func newBaselineUpdaterWithQuerier(q pgxQuerier) *BaselineUpdater {
	return &BaselineUpdater{pool: q}
}

type baselineStats struct {
	p50         *float64
	p95         *float64
	p99         *float64
	successRate *float64
	sampleCount int
}

func (b *BaselineUpdater) computeBaseline(ctx context.Context, monitorID string) (baselineStats, error) {
	var stats baselineStats
	row := b.pool.QueryRow(ctx, `
		SELECT
		  percentile_cont(0.50) WITHIN GROUP (ORDER BY latency_ms) AS p50,
		  percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95,
		  percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms) AS p99,
		  AVG(CASE WHEN status = 'up' THEN 1.0 ELSE 0.0 END) AS success_rate,
		  COUNT(*) AS sample_count
		FROM monitor_checks
		WHERE monitor_id = $1 AND check_at > NOW() - INTERVAL '7 days'
	`, monitorID)

	err := row.Scan(&stats.p50, &stats.p95, &stats.p99, &stats.successRate, &stats.sampleCount)
	return stats, err
}

// UpdateBaseline recomputes the baseline for a single monitor using the last 7 days.
// Rate-limited: only runs when sample_count % 100 == 0, or baseline is older than 6h.
func (b *BaselineUpdater) UpdateBaseline(ctx context.Context, monitorID string) error {
	if b.pool == nil {
		return nil
	}

	var existingSampleCount int
	var computedAt *time.Time
	row := b.pool.QueryRow(ctx, `
		SELECT sample_count, computed_at FROM monitor_baselines WHERE monitor_id = $1
	`, monitorID)
	err := row.Scan(&existingSampleCount, &computedAt)
	if err == nil {
		// Baseline exists — rate-limit: skip if recent and not at 100-sample boundary.
		if computedAt != nil && time.Since(*computedAt) < 6*time.Hour {
			if existingSampleCount == 0 || existingSampleCount%100 != 0 {
				return nil
			}
		}
	}

	stats, err := b.computeBaseline(ctx, monitorID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	id := idgen.New("bln_")

	_, err = b.pool.Exec(ctx, `
		INSERT INTO monitor_baselines
		  (id, monitor_id, p50_latency, p95_latency, p99_latency, success_rate, sample_count, computed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (monitor_id) DO UPDATE SET
		  p50_latency   = EXCLUDED.p50_latency,
		  p95_latency   = EXCLUDED.p95_latency,
		  p99_latency   = EXCLUDED.p99_latency,
		  success_rate  = EXCLUDED.success_rate,
		  sample_count  = EXCLUDED.sample_count,
		  computed_at   = EXCLUDED.computed_at,
		  updated_at    = EXCLUDED.updated_at
	`, id, monitorID, stats.p50, stats.p95, stats.p99, stats.successRate, stats.sampleCount, now)
	return err
}
