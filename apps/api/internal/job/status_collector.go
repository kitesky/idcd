// Package job hosts background workers attached to the api process.
package job

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status codes mirror the SMALLINT values in status_uptime_5min/daily
// (see lib/db/migrations/idcd_main/00050_status_uptime.sql).
const (
	StatusOperational int16 = 1
	StatusDegraded    int16 = 2
	StatusOutage      int16 = 3
	StatusMaintenance int16 = 4
)

const (
	defaultProbeInterval  = 5 * time.Minute
	defaultProbeTimeout   = 3 * time.Second
	defaultDegradedMs     = 1000
	nodeDegradedThreshold = 5 * time.Minute  // last_seen older than this → degraded
	nodeOutageThreshold   = 15 * time.Minute // last_seen older than this → outage
)

// StatusCollector probes idcd's own services + nodes every Interval and
// persists one row per service to status_uptime_5min. The daily aggregator
// (separate cron) rolls 5min buckets up to status_uptime_daily for the
// GitHub-style 90-day uptime bars on idcd.com/status.
type StatusCollector struct {
	pool       *pgxpool.Pool
	httpClient *http.Client
	logger     *slog.Logger

	interval   time.Duration
	timeout    time.Duration
	degradedMs int
	services   map[string]string // service_key → probe URL

	now func() time.Time
}

// Options tunes the collector. Zero-value fields fall back to documented
// defaults; an empty Services map means probeServices() becomes a no-op and
// only node-level rows are written.
type Options struct {
	Interval   time.Duration
	Timeout    time.Duration
	DegradedMs int
	Services   map[string]string
	Logger     *slog.Logger
	Clock      func() time.Time
}

// New builds a collector. The collector does not register itself with any
// scheduler — call Run(ctx) in a goroutine from main.
func New(pool *pgxpool.Pool, opts Options) *StatusCollector {
	c := &StatusCollector{
		pool:       pool,
		interval:   opts.Interval,
		timeout:    opts.Timeout,
		degradedMs: opts.DegradedMs,
		services:   opts.Services,
		logger:     opts.Logger,
		now:        opts.Clock,
	}
	if c.interval <= 0 {
		c.interval = defaultProbeInterval
	}
	if c.timeout <= 0 {
		c.timeout = defaultProbeTimeout
	}
	if c.degradedMs <= 0 {
		c.degradedMs = defaultDegradedMs
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	if c.now == nil {
		c.now = time.Now
	}
	c.httpClient = &http.Client{Timeout: c.timeout}
	return c
}

// Run probes once immediately then ticks at Interval until ctx is cancelled.
// Per-tick errors are logged but never returned — a single transient failure
// (DB hiccup, network blip) must not stop the loop.
func (c *StatusCollector) Run(ctx context.Context) error {
	if c.pool == nil {
		return fmt.Errorf("status collector: pool is nil")
	}

	c.logger.Info("status collector starting",
		"interval", c.interval.String(),
		"services", len(c.services),
	)

	if err := c.collectOnce(ctx); err != nil {
		c.logger.Warn("status collect failed", "err", err)
	}

	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("status collector stopping")
			return nil
		case <-t.C:
			if err := c.collectOnce(ctx); err != nil {
				c.logger.Warn("status collect failed", "err", err)
			}
		}
	}
}

// sample captures one (service_key, bucket_at, status, detail) row before
// it's bulk-inserted. Kept private — the public surface is just Run.
type sample struct {
	serviceKey string
	status     int16
	detail     map[string]any
}

// collectOnce runs a single probe cycle. Service probes and the node scan run
// in parallel; both feed into a single bulk INSERT. The bucket_at is computed
// once (truncated to 5min) so all rows for one tick share the same key.
func (c *StatusCollector) collectOnce(ctx context.Context) error {
	bucket := c.now().Truncate(5 * time.Minute)

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex // guards samples + serviceErr + nodeErr (single-lock keeps the rules obvious)
		samples    []sample
		serviceErr error
		nodeErr    error
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer recoverPanic(c.logger, "probeServices")
		ss, err := c.probeServices(ctx)
		mu.Lock()
		samples = append(samples, ss...)
		serviceErr = err
		mu.Unlock()
	}()
	go func() {
		defer wg.Done()
		defer recoverPanic(c.logger, "probeNodes")
		ss, err := c.probeNodes(ctx)
		mu.Lock()
		samples = append(samples, ss...)
		nodeErr = err
		mu.Unlock()
	}()
	wg.Wait()

	if len(samples) == 0 {
		// Nothing to write — log the underlying errors so dev sees what's
		// happening (no services configured + 0 nodes is a valid empty state).
		if serviceErr != nil || nodeErr != nil {
			return fmt.Errorf("no samples: serviceErr=%v nodeErr=%v", serviceErr, nodeErr)
		}
		return nil
	}

	if err := c.writeSamples(ctx, bucket, samples); err != nil {
		return fmt.Errorf("write samples: %w", err)
	}

	c.logger.Debug("status collect ok",
		"bucket", bucket.Format(time.RFC3339),
		"samples", len(samples),
		"service_err", serviceErr,
		"node_err", nodeErr,
	)
	return nil
}

// probeServices fires concurrent HTTP GETs against every configured service
// URL and classifies each response. A non-2xx, timeout, or transport error
// becomes outage; a slow 2xx (≥ degradedMs) becomes degraded.
func (c *StatusCollector) probeServices(ctx context.Context) ([]sample, error) {
	if len(c.services) == 0 {
		return nil, nil
	}

	out := make([]sample, 0, len(c.services))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for key, url := range c.services {
		wg.Add(1)
		go func(serviceKey, probeURL string) {
			defer wg.Done()
			status, latencyMs, detail := c.probeOne(ctx, probeURL)
			mu.Lock()
			out = append(out, sample{
				serviceKey: serviceKey,
				status:     status,
				detail: map[string]any{
					"probe_url":  probeURL,
					"latency_ms": latencyMs,
					"detail":     detail,
				},
			})
			mu.Unlock()
		}(key, url)
	}
	wg.Wait()
	return out, nil
}

// probeOne performs a single HTTP probe and returns the classified status,
// elapsed milliseconds, and a short human-readable detail string used in
// logs / admin debugging (not surfaced on the public page).
func (c *StatusCollector) probeOne(ctx context.Context, url string) (int16, int64, string) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := c.now()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return StatusOutage, 0, "build_request: " + err.Error()
	}
	resp, err := c.httpClient.Do(req)
	elapsed := c.now().Sub(start).Milliseconds()
	if err != nil {
		return StatusOutage, elapsed, "transport: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StatusOutage, elapsed, fmt.Sprintf("http_%d", resp.StatusCode)
	}
	if elapsed >= int64(c.degradedMs) {
		return StatusDegraded, elapsed, fmt.Sprintf("slow_%dms", elapsed)
	}
	return StatusOperational, elapsed, "ok"
}

// probeNodes reads enrolled_nodes once and classifies each active row by
// last_seen_at staleness. Each node produces a row keyed 'node:<node_id>'
// with country_code / ip / last_seen_age_s in detail JSONB so the frontend
// can group by country without an extra query.
func (c *StatusCollector) probeNodes(ctx context.Context) ([]sample, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT node_id,
		       ip_address,
		       last_seen_at,
		       COALESCE(metadata->>'country_code', '') AS country_code,
		       COALESCE(metadata->>'city', '')         AS city
		  FROM enrolled_nodes
		 WHERE status = 'active'
	`)
	if err != nil {
		return nil, fmt.Errorf("query enrolled_nodes: %w", err)
	}
	defer rows.Close()

	now := c.now()
	out := []sample{}
	for rows.Next() {
		var (
			nodeID      string
			ipAddr      *string
			lastSeen    *time.Time
			countryCode string
			city        string
		)
		if err := rows.Scan(&nodeID, &ipAddr, &lastSeen, &countryCode, &city); err != nil {
			return out, fmt.Errorf("scan node row: %w", err)
		}

		status, ageSec := classifyNodeAge(lastSeen, now)

		detail := map[string]any{
			"country_code":     countryCode,
			"city":             city,
			"last_seen_age_s":  ageSec,
		}
		if ipAddr != nil {
			detail["ip"] = *ipAddr
		}

		out = append(out, sample{
			serviceKey: "node:" + nodeID,
			status:     status,
			detail:     detail,
		})
	}
	return out, rows.Err()
}

// writeSamples bulk-upserts all samples for one bucket. Uses ON CONFLICT
// DO UPDATE so a re-run of the same bucket overwrites with the latest probe
// result instead of failing.
func (c *StatusCollector) writeSamples(ctx context.Context, bucket time.Time, samples []sample) error {
	// pgx Batch keeps it one round-trip for small N (typical: 6 services +
	// a few dozen nodes — never enough to need COPY).
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, s := range samples {
		_, err := tx.Exec(ctx, `
			INSERT INTO status_uptime_5min (service_key, bucket_at, status, detail)
			VALUES ($1, $2, $3, $4::jsonb)
			ON CONFLICT (service_key, bucket_at)
			DO UPDATE SET status = EXCLUDED.status, detail = EXCLUDED.detail
		`, s.serviceKey, bucket, s.status, mustMarshal(s.detail))
		if err != nil {
			return fmt.Errorf("insert %s: %w", s.serviceKey, err)
		}
	}
	return tx.Commit(ctx)
}
