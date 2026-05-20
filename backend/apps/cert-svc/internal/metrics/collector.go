package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default scrape interval. The collector goroutine ticks at this rate and
// refreshes every gauge in one pass. 30s is the PRD §13 spec — fast enough
// for fall-over decisions, slow enough not to add noticeable DB load.
const DefaultInterval = 30 * time.Second

// QuotaSampler is the narrow interface the collector needs to refresh the
// per-CA quota gauge. *service.RepoQuotaChecker satisfies this — we
// declare a local interface to avoid an import cycle and to keep the
// collector trivially testable with an in-memory fake.
type QuotaSampler interface {
	Usage(ctx context.Context, caName string) (UsageRatio, error)
}

// UsageRatio mirrors service.QuotaUsage so the collector does not depend
// on the service package. The collector picks the max of the two fields
// as the headline "how close to fallover" signal.
type UsageRatio struct {
	PerRegisteredDomain float64
	PerAccount3h        float64
}

// Max returns the worst-case (largest) of the two ratios. Router uses
// the same notion to decide fallover, so the gauge tracks the same value.
func (u UsageRatio) Max() float64 {
	if u.PerRegisteredDomain > u.PerAccount3h {
		return u.PerRegisteredDomain
	}
	return u.PerAccount3h
}

// QueueLister is satisfied by *redis.Client (and by miniredis in tests via
// its *redis.Client adapter). Kept narrow so unit tests can fake it.
type QueueLister interface {
	XLen(ctx context.Context, stream string) *redis.IntCmd
}

// Collector periodically refreshes gauges that cannot be incremented
// in-line on the hot path: CA quota usage (DB query) + Redis stream queue
// depth (XLEN). Counters and the order-duration histogram are recorded by
// the orchestrator directly — the collector only owns gauges.
type Collector struct {
	queues   QueueLister
	streams  []string
	sampler  QuotaSampler
	cas      []string
	interval time.Duration
	logger   *slog.Logger
	now      func() time.Time
}

// Option tunes a Collector at construction time.
type Option func(*Collector)

// WithInterval overrides the default 30s scrape tick.
func WithInterval(d time.Duration) Option {
	return func(c *Collector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithLogger swaps the structured logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Collector) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithStreams overrides the list of Redis streams the collector polls
// for queue depth. Defaults to ["cert:order_events"].
func WithStreams(streams ...string) Option {
	return func(c *Collector) {
		if len(streams) > 0 {
			c.streams = append([]string(nil), streams...)
		}
	}
}

// WithCAs overrides the list of CA names polled for quota usage.
// Defaults to ["lets-encrypt"] — the only CA with published quotas as of
// S2 (ZeroSSL / Buypass do not surface ACME-side caps).
func WithCAs(cas ...string) Option {
	return func(c *Collector) {
		if len(cas) > 0 {
			c.cas = append([]string(nil), cas...)
		}
	}
}

// NewCollector wires the gauge collector. queues and sampler are both
// optional — pass nil to disable that particular gauge family (useful for
// tests / preview deploys where one dependency is not yet ready).
func NewCollector(queues QueueLister, sampler QuotaSampler, opts ...Option) *Collector {
	c := &Collector{
		queues:   queues,
		streams:  []string{"cert:order_events"},
		sampler:  sampler,
		cas:      []string{"lets-encrypt"},
		interval: DefaultInterval,
		logger:   slog.Default(),
		now:      time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Run blocks until ctx is cancelled, refreshing every gauge once per
// interval (plus one immediate scrape on start). Per-tick errors are
// logged at warn level and never abort the loop.
func (c *Collector) Run(ctx context.Context) error {
	c.logger.Info("metrics collector starting",
		"interval", c.interval.String(),
		"streams", c.streams,
		"cas", c.cas)

	c.scrape(ctx)

	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("metrics collector stopping")
			return nil
		case <-t.C:
			c.scrape(ctx)
		}
	}
}

// scrape runs one refresh pass. Exposed (lower-case) so the test file in
// the same package can drive it deterministically without a ticker.
func (c *Collector) scrape(ctx context.Context) {
	if c.queues != nil {
		for _, stream := range c.streams {
			n, err := c.queues.XLen(ctx, stream).Result()
			if err != nil {
				c.logger.Warn("xlen failed", "stream", stream, "err", err)
				CollectorScrapeFailures.WithLabelValues("queue_depth").Inc()
				continue
			}
			SetQueueDepth(stream, n)
		}
	}

	if c.sampler != nil {
		for _, ca := range c.cas {
			u, err := c.sampler.Usage(ctx, ca)
			if err != nil {
				c.logger.Warn("quota sample failed", "ca", ca, "err", err)
				CollectorScrapeFailures.WithLabelValues("ca_quota").Inc()
				continue
			}
			SetCAQuotaUsed(ca, u.Max())
		}
	}
}
