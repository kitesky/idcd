package job

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 5min retention window — anything older than this can be safely deleted
// because it's already been rolled up into status_uptime_daily.
const fiveMinRetention = 7 * 24 * time.Hour

// DailyAggregator rolls yesterday's 5min buckets into one status_uptime_daily
// row per service_key. Designed to run once a day (00:05 local) but is safe
// to run multiple times — uses ON CONFLICT DO UPDATE.
//
// Schedule strategy: a simple sleep-until-next-window loop instead of a real
// cron lib. Keeps zero new deps; the only knob is RunAtMinute (default 5).
type DailyAggregator struct {
	pool        *pgxpool.Pool
	logger      *slog.Logger
	runAtMinute int // minute past midnight UTC to fire; default 5
	now         func() time.Time
}

// DailyOptions tunes the aggregator. Zero-value RunAtMinute uses 5 (i.e.
// 00:05 UTC) — late enough that the final 5min bucket of the day (23:55)
// is definitely persisted, early enough that morning operators see fresh data.
type DailyOptions struct {
	RunAtMinute int
	Logger      *slog.Logger
	Clock       func() time.Time
}

// NewDailyAggregator builds the aggregator. Run() expects to be invoked from
// a goroutine; it blocks until ctx is cancelled.
func NewDailyAggregator(pool *pgxpool.Pool, opts DailyOptions) *DailyAggregator {
	a := &DailyAggregator{
		pool:        pool,
		logger:      opts.Logger,
		runAtMinute: opts.RunAtMinute,
		now:         opts.Clock,
	}
	if a.runAtMinute < 0 || a.runAtMinute >= 60 {
		a.runAtMinute = 5
	}
	if a.runAtMinute == 0 {
		a.runAtMinute = 5
	}
	if a.logger == nil {
		a.logger = slog.Default()
	}
	if a.now == nil {
		a.now = time.Now
	}
	return a
}

// Run sleeps until the next 00:<runAtMinute> UTC, fires AggregateYesterday +
// pruneOldFiveMin, then loops. On startup it ALSO fires once immediately so
// a fresh deploy backfills the previous day without waiting up to 24h.
func (a *DailyAggregator) Run(ctx context.Context) error {
	if a.pool == nil {
		return fmt.Errorf("daily aggregator: pool is nil")
	}

	a.logger.Info("daily aggregator starting", "run_at_minute_utc", a.runAtMinute)

	// Backfill on startup so we don't lose the most recent rollup when
	// the api restarts during the day.
	if err := a.runOnce(ctx); err != nil {
		a.logger.Warn("startup aggregation failed", "err", err)
	}

	for {
		delay := a.untilNextRun()
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			a.logger.Info("daily aggregator stopping")
			return nil
		case <-timer.C:
			if err := a.runOnce(ctx); err != nil {
				a.logger.Warn("daily aggregation failed", "err", err)
			}
		}
	}
}

// runOnce executes one full daily cycle: aggregate yesterday + prune.
func (a *DailyAggregator) runOnce(ctx context.Context) error {
	yesterday := a.now().UTC().AddDate(0, 0, -1)
	day := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)

	if err := a.AggregateDay(ctx, day); err != nil {
		return fmt.Errorf("aggregate %s: %w", day.Format("2006-01-02"), err)
	}
	if err := a.pruneOldFiveMin(ctx); err != nil {
		// Log but don't fail the cycle — pruning is housekeeping.
		a.logger.Warn("prune old 5min rows failed", "err", err)
	}
	return nil
}

// AggregateDay reduces all status_uptime_5min rows for the given UTC day into
// one status_uptime_daily row per service_key. Exported so admin can re-run
// for a specific day if data was backfilled or corrected.
//
// uptime_pct = 100 * count(status=1) / count(*)
// worst_status = MAX(status)  — relies on 1<2<3<4 ordering matching severity
// incident_ids = ARRAY(SELECT id FROM status_incidents
//                       WHERE service_key=$svc
//                         AND started_at < day+1d
//                         AND (ended_at IS NULL OR ended_at >= day))
func (a *DailyAggregator) AggregateDay(ctx context.Context, day time.Time) error {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)

	// $1 = dayStart (UTC midnight), $2 = dayEnd (next-day midnight).
	// dayStart drives both the bucket window lower bound AND the
	// incident-overlap "ended_at >= dayStart" check.
	_, err := a.pool.Exec(ctx, `
		INSERT INTO status_uptime_daily (service_key, day, uptime_pct, worst_status, incident_ids)
		SELECT
		  service_key,
		  $1::date                                              AS day,
		  ROUND(
		    100.0 * COUNT(*) FILTER (WHERE status = 1) / NULLIF(COUNT(*), 0),
		    2
		  )                                                     AS uptime_pct,
		  MAX(status)                                           AS worst_status,
		  COALESCE(
		    (SELECT array_agg(i.id ORDER BY i.started_at)
		       FROM status_incidents i
		      WHERE i.service_key = m.service_key
		        AND i.started_at <  $2
		        AND (i.ended_at IS NULL OR i.ended_at >= $1)),
		    ARRAY[]::BIGINT[]
		  )                                                     AS incident_ids
		FROM status_uptime_5min m
		WHERE bucket_at >= $1 AND bucket_at < $2
		GROUP BY service_key
		ON CONFLICT (service_key, day)
		DO UPDATE SET
		  uptime_pct   = EXCLUDED.uptime_pct,
		  worst_status = EXCLUDED.worst_status,
		  incident_ids = EXCLUDED.incident_ids
	`, dayStart, dayEnd)
	if err != nil {
		return err
	}

	a.logger.Info("aggregated day",
		"day", dayStart.Format("2006-01-02"),
	)
	return nil
}

// pruneOldFiveMin deletes status_uptime_5min rows older than the retention
// window. Cheap with idx_status_uptime_5min_recent (bucket_at DESC).
func (a *DailyAggregator) pruneOldFiveMin(ctx context.Context) error {
	cutoff := a.now().Add(-fiveMinRetention)
	tag, err := a.pool.Exec(ctx, `DELETE FROM status_uptime_5min WHERE bucket_at < $1`, cutoff)
	if err != nil {
		return err
	}
	if n := tag.RowsAffected(); n > 0 {
		a.logger.Info("pruned old 5min rows", "deleted", n, "cutoff", cutoff.Format(time.RFC3339))
	}
	return nil
}

// untilNextRun returns the duration from now until the next runAtMinute UTC
// boundary. Always returns > 0 — if the boundary today already passed, target
// tomorrow.
func (a *DailyAggregator) untilNextRun() time.Duration {
	now := a.now().UTC()
	target := time.Date(now.Year(), now.Month(), now.Day(), 0, a.runAtMinute, 0, 0, time.UTC)
	if !target.After(now) {
		target = target.AddDate(0, 0, 1)
	}
	return target.Sub(now)
}
