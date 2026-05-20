package processor

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// AnchorChecker compares check results against the monitor baseline.
type AnchorChecker struct {
	pool pgxQuerier
}

func newAnchorChecker(pool *pgxpool.Pool) *AnchorChecker {
	if pool == nil {
		return &AnchorChecker{}
	}
	return &AnchorChecker{pool: &poolQuerier{pool: pool}}
}

func newAnchorCheckerWithQuerier(q pgxQuerier) *AnchorChecker {
	return &AnchorChecker{pool: q}
}

type anchorBaseline struct {
	id          string
	p95Latency  *float64
	successRate *float64
	sampleCount int
}

func (a *AnchorChecker) loadBaseline(ctx context.Context, monitorID string) (*anchorBaseline, error) {
	var bl anchorBaseline
	row := a.pool.QueryRow(ctx, `
		SELECT id, p95_latency, success_rate, sample_count
		FROM monitor_baselines
		WHERE monitor_id = $1
	`, monitorID)
	err := row.Scan(&bl.id, &bl.p95Latency, &bl.successRate, &bl.sampleCount)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bl, nil
}

func (a *AnchorChecker) hasOpenDeviation(ctx context.Context, monitorID, devType string) (bool, error) {
	var count int
	err := a.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM anchor_deviations
		WHERE monitor_id = $1 AND deviation_type = $2 AND status = 'open'
	`, monitorID, devType).Scan(&count)
	return count > 0, err
}

func (a *AnchorChecker) insertDeviation(ctx context.Context, monitorID, baselineID, devType string, current, baseline, pct float64, severity string) error {
	id := idgen.New("dev_")
	now := time.Now().UTC()
	_, err := a.pool.Exec(ctx, `
		INSERT INTO anchor_deviations
		  (id, monitor_id, baseline_id, deviation_type, current_value, baseline_value, deviation_pct, severity, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'open', $9)
	`, id, monitorID, baselineID, devType, current, baseline, pct, severity, now)
	return err
}

func (a *AnchorChecker) resolveDeviations(ctx context.Context, monitorID, devType string) error {
	now := time.Now().UTC()
	_, err := a.pool.Exec(ctx, `
		UPDATE anchor_deviations
		SET status = 'resolved', resolved_at = $1
		WHERE monitor_id = $2 AND deviation_type = $3 AND status = 'open'
	`, now, monitorID, devType)
	return err
}

// CheckDeviation compares current check result against baseline.
// Called in processor.Process() after baseline update.
func (a *AnchorChecker) CheckDeviation(ctx context.Context, monitorID string, latencyMs float64, isSuccess bool) error {
	if a.pool == nil {
		return nil
	}

	bl, err := a.loadBaseline(ctx, monitorID)
	if err != nil {
		return err
	}
	if bl == nil || bl.sampleCount < 10 {
		return nil
	}

	// Latency deviation check.
	if bl.p95Latency != nil && *bl.p95Latency > 0 {
		ratio := latencyMs / *bl.p95Latency
		if ratio > 3.0 {
			open, err := a.hasOpenDeviation(ctx, monitorID, "latency_spike")
			if err != nil {
				return err
			}
			if !open {
				pct := (latencyMs - *bl.p95Latency) / *bl.p95Latency * 100
				if err := a.insertDeviation(ctx, monitorID, bl.id, "latency_spike", latencyMs, *bl.p95Latency, pct, "critical"); err != nil {
					return err
				}
			}
		} else if ratio > 2.0 {
			open, err := a.hasOpenDeviation(ctx, monitorID, "latency_spike")
			if err != nil {
				return err
			}
			if !open {
				pct := (latencyMs - *bl.p95Latency) / *bl.p95Latency * 100
				if err := a.insertDeviation(ctx, monitorID, bl.id, "latency_spike", latencyMs, *bl.p95Latency, pct, "warning"); err != nil {
					return err
				}
			}
		} else {
			if err := a.resolveDeviations(ctx, monitorID, "latency_spike"); err != nil {
				return err
			}
		}
	}

	// Success rate deviation check.
	if bl.successRate != nil {
		successVal := 0.0
		if isSuccess {
			successVal = 1.0
		}
		threshold := *bl.successRate * 0.95
		if successVal < threshold {
			open, err := a.hasOpenDeviation(ctx, monitorID, "success_rate_drop")
			if err != nil {
				return err
			}
			if !open {
				pct := (*bl.successRate - successVal) / *bl.successRate * 100
				if err := a.insertDeviation(ctx, monitorID, bl.id, "success_rate_drop", successVal, *bl.successRate, pct, "warning"); err != nil {
					return err
				}
			}
		} else {
			if err := a.resolveDeviations(ctx, monitorID, "success_rate_drop"); err != nil {
				return err
			}
		}
	}

	return nil
}
