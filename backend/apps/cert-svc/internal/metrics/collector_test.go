package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSampler implements QuotaSampler — we record every call so the test
// can assert the collector polled the configured CA list.
type fakeSampler struct {
	mu       sync.Mutex
	usage    map[string]UsageRatio
	err      map[string]error
	calls    []string
}

func (f *fakeSampler) Usage(_ context.Context, ca string) (UsageRatio, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, ca)
	if e := f.err[ca]; e != nil {
		return UsageRatio{}, e
	}
	return f.usage[ca], nil
}

func newQuietCollectorLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestUsageRatio_Max(t *testing.T) {
	assert.InDelta(t, 0.8, UsageRatio{0.8, 0.5}.Max(), 1e-9)
	assert.InDelta(t, 0.9, UsageRatio{0.3, 0.9}.Max(), 1e-9)
	assert.InDelta(t, 0.0, UsageRatio{}.Max(), 1e-9)
}

func TestNewCollector_Defaults(t *testing.T) {
	c := NewCollector(nil, nil)
	assert.Equal(t, []string{"cert:order_events"}, c.streams)
	assert.Equal(t, []string{"lets-encrypt"}, c.cas)
	assert.Equal(t, DefaultInterval, c.interval)
}

func TestNewCollector_Options(t *testing.T) {
	logger := newQuietCollectorLogger()
	c := NewCollector(nil, nil,
		WithInterval(2*time.Second),
		WithLogger(logger),
		WithStreams("a", "b"),
		WithCAs("zerossl"),
	)
	assert.Equal(t, 2*time.Second, c.interval)
	assert.Same(t, logger, c.logger)
	assert.Equal(t, []string{"a", "b"}, c.streams)
	assert.Equal(t, []string{"zerossl"}, c.cas)

	// Zero / empty overrides are ignored.
	WithInterval(0)(c)
	WithLogger(nil)(c)
	WithStreams()(c)
	WithCAs()(c)
	assert.Equal(t, 2*time.Second, c.interval)
	assert.Same(t, logger, c.logger)
	assert.Equal(t, []string{"a", "b"}, c.streams)
	assert.Equal(t, []string{"zerossl"}, c.cas)
}

func TestCollectorScrape_UpdatesGauges(t *testing.T) {
	QueueDepth.Reset()
	CAQuotaUsed.Reset()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	// Seed a stream with 3 entries via XAdd.
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.NoError(t, rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: "cert:order_events",
			Values: map[string]any{"order_id": "1"},
		}).Err())
	}

	sampler := &fakeSampler{
		usage: map[string]UsageRatio{
			"lets-encrypt": {PerRegisteredDomain: 0.42, PerAccount3h: 0.1},
		},
	}

	c := NewCollector(rdb, sampler,
		WithLogger(newQuietCollectorLogger()),
	)

	c.scrape(ctx)

	assert.Equal(t, float64(3),
		testutil.ToFloat64(QueueDepth.WithLabelValues("cert:order_events")))
	assert.InDelta(t, 0.42,
		testutil.ToFloat64(CAQuotaUsed.WithLabelValues("lets-encrypt")), 1e-9)
	assert.Equal(t, []string{"lets-encrypt"}, sampler.calls)
}

func TestCollectorScrape_ContinuesOnError(t *testing.T) {
	QueueDepth.Reset()
	CAQuotaUsed.Reset()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	require.NoError(t, rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: "good",
		Values: map[string]any{"k": "v"},
	}).Err())

	sampler := &fakeSampler{
		usage: map[string]UsageRatio{"ok-ca": {PerAccount3h: 0.5}},
		err:   map[string]error{"bad-ca": errors.New("boom")},
	}

	c := NewCollector(rdb, sampler,
		WithLogger(newQuietCollectorLogger()),
		WithStreams("good", "does-not-exist-stream"),
		WithCAs("bad-ca", "ok-ca"),
	)

	c.scrape(context.Background())

	// Good stream observed.
	assert.Equal(t, float64(1),
		testutil.ToFloat64(QueueDepth.WithLabelValues("good")))
	// Missing stream is XLEN=0 in miniredis (not an error); we accept either path.

	// Sampler error did not abort — ok-ca was still polled.
	assert.InDelta(t, 0.5,
		testutil.ToFloat64(CAQuotaUsed.WithLabelValues("ok-ca")), 1e-9)
	assert.Contains(t, sampler.calls, "bad-ca")
	assert.Contains(t, sampler.calls, "ok-ca")
}

func TestCollectorScrape_NilDependenciesSkip(t *testing.T) {
	// Must not panic; gauges simply stay untouched.
	c := NewCollector(nil, nil, WithLogger(newQuietCollectorLogger()))
	c.scrape(context.Background())
}

func TestCollectorRun_StopsOnContext(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	c := NewCollector(rdb, &fakeSampler{usage: map[string]UsageRatio{"lets-encrypt": {}}},
		WithInterval(10*time.Millisecond),
		WithLogger(newQuietCollectorLogger()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Let it tick at least once.
	time.Sleep(40 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("collector did not stop on cancel")
	}
}
