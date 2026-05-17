// cert_consumer_test.go — miniredis-backed unit tests for the S2 W8
// cert:notifications consumer.  Each test stands up an independent in-process
// Redis (via miniredis.RunT) so they parallel cleanly and don't require a
// running Redis instance.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/lib/shared/i18n"
)

// loadHermeticRegistry wraps i18n.LoadFromBytesForTesting so the test reads
// like prose.  Returns the parsed *Registry or the underlying error so the
// caller can surface it via t.Fatalf with full context.
func loadHermeticRegistry(raw []byte) (*i18n.Registry, error) {
	return i18n.LoadFromBytesForTesting(raw)
}

// ---- stubs ----

// fakeSender captures every email handed to it and can be primed with a
// rolling error sequence (one entry per Send call). When the slice runs out,
// later sends succeed.  Thread-safe.
type fakeSender struct {
	mu       sync.Mutex
	sent     []email.Message
	errs     []error
	sendHook func(email.Message) // optional per-send observer
}

func (f *fakeSender) Send(_ context.Context, msg email.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendHook != nil {
		f.sendHook(msg)
	}
	var err error
	if len(f.errs) > 0 {
		err, f.errs = f.errs[0], f.errs[1:]
	}
	if err == nil {
		f.sent = append(f.sent, msg)
	}
	return err
}

func (f *fakeSender) Sent() []email.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]email.Message, len(f.sent))
	copy(out, f.sent)
	return out
}

// ---- helpers ----

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

func newTestConsumer(t *testing.T, rdb *redis.Client, sender email.Sender, lookup EmailLookup, opts ...CertConsumerOption) *CertConsumer {
	t.Helper()
	tpls, err := template.New()
	if err != nil {
		t.Fatalf("template.New: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	// Crank the block timeout / retry wait down so tests don't pay real wall
	// clock for the in-process retry sleeps.
	allOpts := append([]CertConsumerOption{
		WithCertBlockTimeout(10 * time.Millisecond),
		WithCertRetryBaseWait(time.Millisecond),
	}, opts...)
	c, err := NewCertConsumer(rdb, sender, tpls, lookup, log,
		"cert:notifications", "cert-notifier", allOpts...)
	if err != nil {
		t.Fatalf("NewCertConsumer: %v", err)
	}
	return c
}

// addEvent xADDs a synthetic cert notification matching the producer's
// schema. Returns the assigned stream ID.
func addEvent(t *testing.T, rdb *redis.Client, stream string, eventType string, accountID, certID, orderID int64, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal payload: %v", err)
	}
	id, err := rdb.XAdd(context.Background(), &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{
			"event":      eventType,
			"account_id": "42",
			"cert_id":    "0",
			"order_id":   "0",
			"payload":    string(raw),
			"emitted_at": time.Now().UTC().Format(time.RFC3339),
		},
	}).Result()
	if err != nil {
		t.Fatalf("XADD: %v", err)
	}
	// Override the redundant top-level fields when caller passes non-default
	// values; this keeps the happy-path call sites short.
	_ = accountID
	_ = certID
	_ = orderID
	return id
}

// drainUntil polls the predicate every 10ms until it returns true or the
// deadline elapses. Used to wait for the consumer goroutine to finish
// processing an event without sleeping arbitrary amounts.
func drainUntil(t *testing.T, deadline time.Duration, predicate func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("drainUntil: predicate never became true within %s", deadline)
}

// constLookup is the most common EmailLookup stub: always returns the same
// recipient + locale, never errors.
func constLookup(addr, locale string) EmailLookup {
	return func(_ context.Context, _ int64) (string, string, error) {
		return addr, locale, nil
	}
}

// ---- tests ----

func TestNewCertConsumer_RequiresAllDeps(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	tpls, err := template.New()
	if err != nil {
		t.Fatalf("template.New: %v", err)
	}
	sender := &fakeSender{}
	lookup := constLookup("u@example.com", "cn")
	log := slog.Default()

	tests := []struct {
		name string
		call func() (*CertConsumer, error)
		want string
	}{
		{"nil rdb", func() (*CertConsumer, error) {
			return NewCertConsumer(nil, sender, tpls, lookup, log, "s", "g")
		}, "redis client required"},
		{"nil sender", func() (*CertConsumer, error) {
			return NewCertConsumer(rdb, nil, tpls, lookup, log, "s", "g")
		}, "email sender required"},
		{"nil templates", func() (*CertConsumer, error) {
			return NewCertConsumer(rdb, sender, nil, lookup, log, "s", "g")
		}, "templates required"},
		{"nil lookup", func() (*CertConsumer, error) {
			return NewCertConsumer(rdb, sender, tpls, nil, log, "s", "g")
		}, "email lookup required"},
		{"empty stream", func() (*CertConsumer, error) {
			return NewCertConsumer(rdb, sender, tpls, lookup, log, "", "g")
		}, "stream name required"},
		{"empty group", func() (*CertConsumer, error) {
			return NewCertConsumer(rdb, sender, tpls, lookup, log, "s", "")
		}, "consumer group required"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestNewCertConsumer_NilLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	tpls, _ := template.New()
	c, err := NewCertConsumer(rdb, &fakeSender{}, tpls, constLookup("u@e.com", "cn"),
		nil, "cert:notifications", "cert-notifier")
	if err != nil {
		t.Fatalf("NewCertConsumer: %v", err)
	}
	if c.logger == nil {
		t.Fatal("logger should default to slog.Default()")
	}
}

func TestCertConsumer_AccessorsAndOptions(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	tpls, _ := template.New()
	c, err := NewCertConsumer(rdb, &fakeSender{}, tpls, constLookup("u@e.com", "cn"),
		slog.Default(), "cert:notifications", "cert-notifier",
		WithCertConsumerName("custom-name"),
		WithCertDashboardBase("https://staging.idcd.com/"),
		WithCertMaxAttempts(5),
		WithCertBlockTimeout(2*time.Second),
		WithCertBatchSize(50),
		WithCertRetryBaseWait(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewCertConsumer: %v", err)
	}
	if c.ConsumerName() != "custom-name" {
		t.Errorf("ConsumerName = %q", c.ConsumerName())
	}
	if c.Stream() != "cert:notifications" {
		t.Errorf("Stream = %q", c.Stream())
	}
	if c.DeadStream() != "cert:notifications:dead" {
		t.Errorf("DeadStream = %q", c.DeadStream())
	}
	if c.dashboardBase != "https://staging.idcd.com" {
		t.Errorf("dashboardBase = %q (trailing slash should be trimmed)", c.dashboardBase)
	}
	if c.maxAttempts != 5 {
		t.Errorf("maxAttempts = %d", c.maxAttempts)
	}
	if c.blockTimeout != 2*time.Second {
		t.Errorf("blockTimeout = %s", c.blockTimeout)
	}
	if c.batchSize != 50 {
		t.Errorf("batchSize = %d", c.batchSize)
	}
	if c.retryBaseWait != 100*time.Millisecond {
		t.Errorf("retryBaseWait = %s", c.retryBaseWait)
	}

	// Zero / negative values must be rejected by the WithCert* options.
	WithCertConsumerName("")(c)
	if c.ConsumerName() != "custom-name" {
		t.Error("empty ConsumerName option should not override")
	}
	WithCertDashboardBase("")(c)
	if c.dashboardBase != "https://staging.idcd.com" {
		t.Error("empty DashboardBase option should not override")
	}
	WithCertMaxAttempts(0)(c)
	if c.maxAttempts != 5 {
		t.Error("zero MaxAttempts option should not override")
	}
	WithCertBlockTimeout(0)(c)
	if c.blockTimeout != 2*time.Second {
		t.Error("zero BlockTimeout option should not override")
	}
	WithCertBatchSize(0)(c)
	if c.batchSize != 50 {
		t.Error("zero BatchSize option should not override")
	}
	withCertClock(nil)(c)
	if c.clock == nil {
		t.Error("nil clock option should not override")
	}
}

func TestCertConsumer_EnsureGroup_Idempotent(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	c := newTestConsumer(t, rdb, &fakeSender{}, constLookup("u@e.com", "cn"))

	ctx := context.Background()
	if err := c.EnsureGroup(ctx); err != nil {
		t.Fatalf("first EnsureGroup: %v", err)
	}
	// Second call must succeed because BUSYGROUP is swallowed.
	if err := c.EnsureGroup(ctx); err != nil {
		t.Fatalf("second EnsureGroup: %v", err)
	}
}

func TestCertConsumer_ProcessIssued_EndToEnd(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "cn"))

	// XADD before the consumer starts; the consumer creates the group at "$"
	// so it would normally skip this entry. Override by creating the group
	// at 0 here.
	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}

	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 99, 7, map[string]any{
		"account_id":     42,
		"cert_id":        99,
		"order_id":       7,
		"sans":           []string{"api.example.com", "www.example.com"},
		"ca":             "lets-encrypt",
		"not_after":      time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		"days_to_expire": 0,
		"error_message":  "",
		"subject":        "已签发",
		"body":           "body text",
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	drainUntil(t, 2*time.Second, func() bool { return len(sender.Sent()) == 1 })
	cancel()
	<-done

	got := sender.Sent()[0]
	if got.To != "u@example.com" {
		t.Errorf("To = %q", got.To)
	}
	if !strings.Contains(got.Subject, "您的 SSL 证书已签发") || !strings.Contains(got.Subject, "api.example.com") {
		t.Errorf("Subject = %q", got.Subject)
	}
	if !strings.Contains(got.HTML, "api.example.com") {
		t.Errorf("HTML missing primary domain; got %s", got.HTML[:min(200, len(got.HTML))])
	}
	if !strings.Contains(got.HTML, "lets-encrypt") {
		t.Errorf("HTML missing CA; got %s", got.HTML[:min(200, len(got.HTML))])
	}
}

func TestCertConsumer_ProcessAllEventTypes(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "en"))

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}

	events := []struct {
		eventType   string
		payload     map[string]any
		mustSubject string
	}{
		{
			EventCertIssued,
			map[string]any{
				"sans": []string{"a.example.com"},
				"ca":   "lets-encrypt",
			},
			"[idcd] Your SSL certificate is ready: a.example.com",
		},
		{
			EventCertFailed,
			map[string]any{
				"sans":          []string{"b.example.com"},
				"ca":            "lets-encrypt",
				"error_message": "dns-01 NXDOMAIN",
			},
			"[idcd] Certificate request failed: b.example.com",
		},
		{
			EventCertExpiring,
			map[string]any{
				"sans":           []string{"c.example.com"},
				"ca":             "lets-encrypt",
				"days_to_expire": 7,
				"not_after":      time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
			"[idcd] Certificate expiring soon: c.example.com (7 days left)",
		},
		{
			EventCertRenewalFailed,
			map[string]any{
				"sans":          []string{"d.example.com"},
				"ca":            "lets-encrypt",
				"error_message": "dns provider 401",
			},
			"[idcd] Certificate auto-renewal failed: d.example.com",
		},
	}
	for _, e := range events {
		addEvent(t, rdb, c.Stream(), e.eventType, 42, 0, 0, e.payload)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	drainUntil(t, 3*time.Second, func() bool { return len(sender.Sent()) == len(events) })
	cancel()
	<-done

	sent := sender.Sent()
	for i, e := range events {
		if sent[i].Subject != e.mustSubject {
			t.Errorf("event[%d] (%s): Subject = %q, want %q",
				i, e.eventType, sent[i].Subject, e.mustSubject)
		}
	}

	// Group should have ACKed all four — XPENDING summary count zero.
	pending, err := rdb.XPending(context.Background(), c.Stream(), c.group).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("XPending count = %d, want 0 (all acked); details: %+v", pending.Count, pending)
	}
}

func TestCertConsumer_RevokedEvent_SoftSkip(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "cn"))

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertRevoked, 42, 0, 0, map[string]any{
		"sans": []string{"x.example.com"},
		"ca":   "lets-encrypt",
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// We can't directly observe "skip", so wait for the message to drain
	// from PEL (consumer ACKs after skip).
	drainUntil(t, 2*time.Second, func() bool {
		p, _ := rdb.XPending(context.Background(), c.Stream(), c.group).Result()
		return p.Count == 0
	})
	cancel()
	<-done

	if got := len(sender.Sent()); got != 0 {
		t.Errorf("revoked event should not send email, got %d sends", got)
	}
}

func TestCertConsumer_LookupSoftMiss_ACKsWithoutSending(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	// EmailLookup returns "", "", nil — soft miss (user has no email).
	lookup := func(_ context.Context, _ int64) (string, string, error) {
		return "", "", nil
	}
	c := newTestConsumer(t, rdb, sender, lookup)

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 0, 0, map[string]any{
		"sans": []string{"a.example.com"},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 2*time.Second, func() bool {
		p, _ := rdb.XPending(context.Background(), c.Stream(), c.group).Result()
		return p.Count == 0
	})
	cancel()
	<-done

	if got := len(sender.Sent()); got != 0 {
		t.Errorf("expected zero sends for soft-miss lookup, got %d", got)
	}
}

func TestCertConsumer_TransientErrorRetriedThenSucceeds(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	// First two Sends fail; third succeeds.
	sender := &fakeSender{errs: []error{errors.New("smtp 4xx"), errors.New("smtp 4xx")}}
	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "cn"))

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 0, 0, map[string]any{
		"sans": []string{"a.example.com"},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 2*time.Second, func() bool { return len(sender.Sent()) == 1 })
	cancel()
	<-done

	if got := len(sender.Sent()); got != 1 {
		t.Errorf("expected 1 successful send after retries, got %d", got)
	}
}

func TestCertConsumer_MaxRetries_DeadLetters(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	// All three attempts fail → entry should land in <stream>:dead.
	sender := &fakeSender{errs: []error{
		errors.New("smtp boom"),
		errors.New("smtp boom"),
		errors.New("smtp boom"),
	}}
	var sendCount atomic.Int32
	sender.sendHook = func(email.Message) { sendCount.Add(1) }

	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "cn"))

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 0, 0, map[string]any{
		"sans": []string{"a.example.com"},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 3*time.Second, func() bool {
		n, _ := rdb.XLen(context.Background(), c.DeadStream()).Result()
		return n == 1
	})
	cancel()
	<-done

	if got := sendCount.Load(); got != int32(c.maxAttempts) {
		t.Errorf("send hook invocations = %d, want %d", got, c.maxAttempts)
	}

	// PEL should be empty (we ACK after dead-lettering).
	p, err := rdb.XPending(context.Background(), c.Stream(), c.group).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if p.Count != 0 {
		t.Errorf("PEL count = %d, want 0 after dead-letter ACK", p.Count)
	}

	// Dead-letter row should carry the reason + original ID + timestamp.
	entries, err := rdb.XRange(context.Background(), c.DeadStream(), "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 dead-letter entry, got %d", len(entries))
	}
	values := entries[0].Values
	if reason, _ := values["_dead_reason"].(string); !strings.Contains(reason, "max_retries") {
		t.Errorf("_dead_reason = %v, want substring max_retries", values["_dead_reason"])
	}
	if orig, _ := values["_dead_original_id"].(string); orig == "" {
		t.Errorf("_dead_original_id missing")
	}
	if at, _ := values["_dead_at"].(string); at == "" {
		t.Errorf("_dead_at missing")
	}
}

func TestCertConsumer_MalformedPayload_DeadLetters(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	c := newTestConsumer(t, rdb, sender, constLookup("u@example.com", "cn"))

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}

	// Missing payload field — parseCertEvent must reject.
	if _, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: c.Stream(),
		Values: map[string]any{
			"event":      EventCertIssued,
			"account_id": "42",
		},
	}).Result(); err != nil {
		t.Fatalf("XADD: %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 2*time.Second, func() bool {
		n, _ := rdb.XLen(context.Background(), c.DeadStream()).Result()
		return n == 1
	})
	cancel()
	<-done

	if got := len(sender.Sent()); got != 0 {
		t.Errorf("malformed entry should not send, got %d", got)
	}
	entries, _ := rdb.XRange(context.Background(), c.DeadStream(), "-", "+").Result()
	if len(entries) != 1 {
		t.Fatalf("dead stream length = %d, want 1", len(entries))
	}
	if reason, _ := entries[0].Values["_dead_reason"].(string); !strings.Contains(reason, "parse") {
		t.Errorf("_dead_reason = %v, want substring parse", entries[0].Values["_dead_reason"])
	}
}

func TestCertConsumer_LookupErrorRetriedThenDeadLetters(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	var calls atomic.Int32
	lookup := func(_ context.Context, _ int64) (string, string, error) {
		calls.Add(1)
		return "", "", errors.New("db timeout")
	}
	c := newTestConsumer(t, rdb, sender, lookup)

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 0, 0, map[string]any{
		"sans": []string{"a.example.com"},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 3*time.Second, func() bool {
		n, _ := rdb.XLen(context.Background(), c.DeadStream()).Result()
		return n == 1
	})
	cancel()
	<-done

	if got := calls.Load(); got != int32(c.maxAttempts) {
		t.Errorf("lookup call count = %d, want %d", got, c.maxAttempts)
	}
}

func TestCertConsumer_UnsupportedLocaleFallsBackToDefault(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	sender := &fakeSender{}
	// Lookup returns a locale that the registry doesn't know.
	lookup := constLookup("u@example.com", "ja")
	c := newTestConsumer(t, rdb, sender, lookup)

	ctx := context.Background()
	if err := rdb.XGroupCreateMkStream(ctx, c.Stream(), c.group, "0").Err(); err != nil {
		t.Fatalf("XGroupCreateMkStream: %v", err)
	}
	addEvent(t, rdb, c.Stream(), EventCertIssued, 42, 0, 0, map[string]any{
		"sans": []string{"a.example.com"},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	drainUntil(t, 2*time.Second, func() bool { return len(sender.Sent()) == 1 })
	cancel()
	<-done

	got := sender.Sent()[0]
	// Default locale is "cn" → Chinese subject.
	if !strings.Contains(got.Subject, "已签发") {
		t.Errorf("expected fallback to cn subject, got %q", got.Subject)
	}
}

func TestCertConsumer_DashboardURLs(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	c := newTestConsumer(t, rdb, &fakeSender{}, constLookup("u@e.com", "cn"))

	if got := c.certDashboardURL(0); got != "https://idcd.com/app/cert" {
		t.Errorf("certDashboardURL(0) = %q", got)
	}
	if got := c.certDashboardURL(42); got != "https://idcd.com/app/cert/42" {
		t.Errorf("certDashboardURL(42) = %q", got)
	}
	if got := c.orderDashboardURL(0); got != "https://idcd.com/app/cert/orders" {
		t.Errorf("orderDashboardURL(0) = %q", got)
	}
	if got := c.orderDashboardURL(99); got != "https://idcd.com/app/cert/orders/99" {
		t.Errorf("orderDashboardURL(99) = %q", got)
	}
}

func TestParseCertEvent_HappyPath(t *testing.T) {
	t.Parallel()
	rawPayload, _ := json.Marshal(map[string]any{
		"account_id":     42,
		"cert_id":        99,
		"order_id":       7,
		"sans":           []string{"a.example.com", "b.example.com"},
		"ca":             "lets-encrypt",
		"days_to_expire": 14,
		"error_message":  "boom",
		"not_after":      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	})
	evt, err := parseCertEvent(map[string]any{
		"event":      EventCertExpiring,
		"account_id": "42",
		"cert_id":    "99",
		"order_id":   "7",
		"payload":    string(rawPayload),
		"emitted_at": "2026-05-17T08:00:00Z",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if evt.EventType != EventCertExpiring {
		t.Errorf("EventType = %q", evt.EventType)
	}
	if evt.AccountID != 42 || evt.CertID != 99 || evt.OrderID != 7 {
		t.Errorf("ids = %d/%d/%d", evt.AccountID, evt.CertID, evt.OrderID)
	}
	if evt.primaryDomain() != "a.example.com" {
		t.Errorf("primaryDomain = %q", evt.primaryDomain())
	}
	if evt.DaysToExpire != 14 {
		t.Errorf("DaysToExpire = %d", evt.DaysToExpire)
	}
	if evt.ErrorMessage != "boom" {
		t.Errorf("ErrorMessage = %q", evt.ErrorMessage)
	}
	if evt.NotAfter.IsZero() {
		t.Errorf("NotAfter zero")
	}
	if evt.EmittedAt.IsZero() {
		t.Errorf("EmittedAt zero")
	}
}

func TestParseCertEvent_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		values map[string]any
		want   string
	}{
		{
			"missing event",
			map[string]any{"account_id": "42", "payload": "{}"},
			"missing 'event'",
		},
		{
			"missing payload",
			map[string]any{"event": "cert.issued", "account_id": "42"},
			"missing 'payload'",
		},
		{
			"bad payload json",
			map[string]any{"event": "cert.issued", "account_id": "42", "payload": "{not json"},
			"decode payload",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCertEvent(tc.values)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestParseCertEvent_TopLevelIDsZeroFallsBackToPayload(t *testing.T) {
	t.Parallel()
	// Top-level account_id zero → parser must pick up payload.account_id.
	raw, _ := json.Marshal(map[string]any{
		"account_id": 7,
		"cert_id":    11,
		"order_id":   13,
		"sans":       []string{"x.example.com"},
	})
	evt, err := parseCertEvent(map[string]any{
		"event":      EventCertIssued,
		"account_id": "0",
		"cert_id":    "0",
		"order_id":   "0",
		"payload":    string(raw),
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if evt.AccountID != 7 || evt.CertID != 11 || evt.OrderID != 13 {
		t.Errorf("ids = %d/%d/%d, want 7/11/13", evt.AccountID, evt.CertID, evt.OrderID)
	}
}

func TestStringField_VariantTypes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"string", "hello", "hello"},
		{"bytes", []byte("hi"), "hi"},
		{"missing", nil, ""}, // overridden below
		{"int", 42, "42"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]any{}
			if tc.name == "missing" {
				if got := stringField(values, "absent"); got != "" {
					t.Errorf("missing field = %q, want empty", got)
				}
				return
			}
			values["k"] = tc.val
			if got := stringField(values, "k"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInt64Field_BadValueReturnsZero(t *testing.T) {
	t.Parallel()
	if got := int64Field(map[string]any{"k": "not-a-number"}, "k"); got != 0 {
		t.Errorf("bad int = %d, want 0", got)
	}
	if got := int64Field(map[string]any{}, "absent"); got != 0 {
		t.Errorf("missing field int = %d, want 0", got)
	}
}

func TestFormatCertTime_Zero(t *testing.T) {
	t.Parallel()
	if got := formatCertTime(time.Time{}); got != "" {
		t.Errorf("zero time = %q, want empty", got)
	}
	got := formatCertTime(time.Date(2026, 8, 15, 12, 30, 0, 0, time.UTC))
	if got != "2026-08-15 12:30 UTC" {
		t.Errorf("formatCertTime = %q", got)
	}
}

func TestSleepOrDone(t *testing.T) {
	t.Parallel()
	t.Run("elapses", func(t *testing.T) {
		ctx := context.Background()
		start := time.Now()
		if !sleepOrDone(ctx, 5*time.Millisecond) {
			t.Error("sleepOrDone returned false when ctx alive")
		}
		if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
			t.Errorf("returned early, elapsed=%s", elapsed)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if sleepOrDone(ctx, 100*time.Millisecond) {
			t.Error("sleepOrDone returned true after ctx cancel")
		}
	})
	t.Run("zero duration", func(t *testing.T) {
		if !sleepOrDone(context.Background(), 0) {
			t.Error("zero duration should return true immediately on live ctx")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if sleepOrDone(ctx, 0) {
			t.Error("zero duration on cancelled ctx should return false")
		}
	})
}

func TestCertSubjects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fn       func(string, string) string
		locale   string
		domain   string
		contains string
	}{
		{"issued cn", certSubjectIssued, "cn", "a.example.com", "已签发"},
		{"issued en", certSubjectIssued, "en", "a.example.com", "is ready"},
		{"failed cn", certSubjectFailed, "cn", "a.example.com", "申请失败"},
		{"failed en", certSubjectFailed, "en", "a.example.com", "request failed"},
		{"renewal cn", certSubjectRenewalFailed, "cn", "a.example.com", "自动续期失败"},
		{"renewal en", certSubjectRenewalFailed, "en", "a.example.com", "auto-renewal failed"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.fn(tc.locale, tc.domain)
			if !strings.Contains(got, tc.contains) || !strings.Contains(got, tc.domain) {
				t.Errorf("subject = %q, want substring %q + %q", got, tc.contains, tc.domain)
			}
		})
	}

	// Expiring subject takes 3 args.
	if got := certSubjectExpiring("cn", "a.example.com", 7); !strings.Contains(got, "剩余 7 天") {
		t.Errorf("expiring cn subject = %q", got)
	}
	if got := certSubjectExpiring("en", "a.example.com", 7); !strings.Contains(got, "7 days") {
		t.Errorf("expiring en subject = %q", got)
	}
}

func TestDefaultConsumerName_NonEmpty(t *testing.T) {
	t.Parallel()
	got := defaultConsumerName("cert-notifier")
	if !strings.HasPrefix(got, "cert-notifier-") {
		t.Errorf("defaultConsumerName = %q, want prefix cert-notifier-", got)
	}
}

func TestCertConsumer_WithCertRegistry_Overrides(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	// Build a hermetic registry with only "fr" so we can verify it overrode
	// the default.  We don't exercise rendering here — just the field set.
	regYAML := []byte(`{
		"default": "fr",
		"locales": [
			{"code": "fr", "bcp47": "fr-FR", "label": "Français", "nativeLabel": "Français", "baseLanguage": "fr", "acceptLanguageAliases": ["fr"], "dir": "ltr", "fontStack": "latin", "fallback": []}
		]
	}`)
	// The real i18n package exposes LoadFromBytesForTesting; use it via the
	// pulled-in registry helper.
	reg, err := loadHermeticRegistry(regYAML)
	if err != nil {
		t.Fatalf("loadHermeticRegistry: %v", err)
	}
	tpls, _ := template.New()
	c, err := NewCertConsumer(rdb, &fakeSender{}, tpls, constLookup("u@e.com", "fr"),
		slog.Default(), "cert:notifications", "cert-notifier",
		WithCertRegistry(reg),
	)
	if err != nil {
		t.Fatalf("NewCertConsumer: %v", err)
	}
	if c.registry.DefaultCode() != "fr" {
		t.Errorf("registry default = %q, want fr", c.registry.DefaultCode())
	}
	// nil registry option must be a no-op.
	WithCertRegistry(nil)(c)
	if c.registry.DefaultCode() != "fr" {
		t.Error("nil registry option should not override")
	}
}

func TestCertEvent_PrimaryDomainEmpty(t *testing.T) {
	t.Parallel()
	if got := (certEvent{}).primaryDomain(); got != "" {
		t.Errorf("empty SANs primary = %q, want empty", got)
	}
}

func TestCertConsumer_Run_CancelsCleanly(t *testing.T) {
	t.Parallel()
	rdb, _ := newTestRedis(t)
	c := newTestConsumer(t, rdb, &fakeSender{}, constLookup("u@e.com", "cn"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v on cancel, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return within 2s after cancel")
	}
}
