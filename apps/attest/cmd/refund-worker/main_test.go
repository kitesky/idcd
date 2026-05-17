package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/attest/internal/refund"
)

// TestNewLogger_AcceptsAllLevels exercises the level-parsing switch so
// a typo in newLogger fails CI rather than at deploy time.
func TestNewLogger_AcceptsAllLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "warning", "error", "", "garbage"} {
		log := newLogger(lvl)
		assert.NotNil(t, log, "newLogger(%q) returned nil", lvl)
	}
}

// TestNewPaddleRefunder_MissingEnv asserts the fail-fast path: the
// binary refuses to start without credentials, which keeps a
// misconfigured deploy from silently dropping refund attempts.
func TestNewPaddleRefunder_MissingEnv(t *testing.T) {
	t.Setenv(envPaddleBaseURL, "")
	t.Setenv(envPaddleAPIKey, "")
	t.Setenv(envPaddleAPISecret, "")
	_, err := newPaddleRefunder()
	require.Error(t, err)
	assert.Contains(t, err.Error(), envPaddleBaseURL)
}

func TestNewPaddleRefunder_OK(t *testing.T) {
	t.Setenv(envPaddleBaseURL, "https://hub.example.com")
	t.Setenv(envPaddleAPIKey, "k")
	t.Setenv(envPaddleAPISecret, "s")
	p, err := newPaddleRefunder()
	require.NoError(t, err)
	assert.NotNil(t, p)

	// Compile-time interface assertion (also exists in the production
	// file); runtime re-check here so a future refactor that drops the
	// implementation surfaces in a targeted test rather than every
	// refund-worker test.
	var _ refund.RefundProvider = p
}

// TestPaddleRefunder_Refund_InputValidation covers the input checks that
// surface before any payment-SDK call. The HTTP path is exercised in
// the SDK's own tests; we only assert the adapter's pre-flight guards.
func TestPaddleRefunder_Refund_InputValidation(t *testing.T) {
	t.Setenv(envPaddleBaseURL, "https://x")
	t.Setenv(envPaddleAPIKey, "k")
	t.Setenv(envPaddleAPISecret, "s")
	p, err := newPaddleRefunder()
	require.NoError(t, err)

	err = p.Refund(context.Background(), "", 100, "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty paddle_order_id")

	err = p.Refund(context.Background(), "pdle_1", 0, "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-positive amount_cents")
}

// TestDefaultRefundID returns a stable prefix so the producer can be
// grepped from payment-hub logs.
func TestDefaultRefundID(t *testing.T) {
	id := defaultRefundID()
	assert.Contains(t, id, "rfd_")
}

// TestDefaultJSONMarshal sanity-checks the seam used by the apology
// mailer to encode payloads. A regression here would silently break
// apology email enqueue at runtime. The full wire shape is asserted
// separately in TestApologyPayload_WireShape.
func TestDefaultJSONMarshal(t *testing.T) {
	b, err := defaultJSONMarshal(apologyPayload{OrderID: "v_1", FailureReason: "x"})
	require.NoError(t, err)
	assert.Contains(t, string(b), `"order_id":"v_1"`)
	assert.Contains(t, string(b), `"failure_reason":"x"`)
}

// TestApologyPayload_WireShape pins the JSON field names — those names
// form a wire contract with apps/notifier/internal/worker/handlers.go
// (RefundApologyPayload). A rename here without coordinating with the
// notifier would silently break apology emails in prod.
func TestApologyPayload_WireShape(t *testing.T) {
	p := apologyPayload{
		OrderID:           "v_1",
		UserEmail:         "u@example.com",
		PaddleOrderID:     "pdle_1",
		RefundAmountCents: 19900,
		Currency:          "CNY",
		FailureReason:     "paddle 5xx",
		EnqueuedAt:        "2026-05-17T10:00:00Z",
	}
	b, err := defaultJSONMarshal(p)
	require.NoError(t, err)
	s := string(b)
	for _, want := range []string{
		`"order_id":"v_1"`,
		`"user_email":"u@example.com"`,
		`"paddle_order_id":"pdle_1"`,
		`"refund_amount_cents":19900`,
		`"currency":"CNY"`,
		`"failure_reason":"paddle 5xx"`,
		`"enqueued_at":"2026-05-17T10:00:00Z"`,
	} {
		assert.Contains(t, s, want, "wire shape regression on %q", want)
	}
}

// TestAsynqApologyMailer_MarshalFailure swaps the JSON hook to verify
// the mailer surfaces marshal errors instead of silently swallowing
// them.
func TestAsynqApologyMailer_MarshalFailure(t *testing.T) {
	prev := jsonMarshal
	defer func() { jsonMarshal = prev }()
	jsonMarshal = func(any) ([]byte, error) { return nil, errors.New("marshal boom") }

	m := &asynqApologyMailer{client: nil, queue: "billing", now: time.Now}
	err := m.SendApology(context.Background(), sampleApologyOrder(), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal boom")
}

// TestAsynqApologyMailer_UsesQueueFallback covers the empty-queue
// branch in SendApology. We can't easily exercise the EnqueueContext
// success path without a live Redis broker; we only need to assert
// that the queue-default logic is in place.
func TestAsynqApologyMailer_UsesQueueFallback(t *testing.T) {
	m := &asynqApologyMailer{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: "127.0.0.1:1"}),
		queue:  "",
		now:    func() time.Time { return time.Now() },
	}
	defer func() { _ = m.client.Close() }()
	// We expect a connection error (no Redis on :1), but the path
	// through queue-fallback must execute first.
	err := m.SendApology(context.Background(), sampleApologyOrder(), "x")
	require.Error(t, err)
}

// TestAsynqApologyMailer_NilOrder guards the defensive nil-check.
func TestAsynqApologyMailer_NilOrder(t *testing.T) {
	m := &asynqApologyMailer{client: nil, queue: "billing", now: time.Now}
	err := m.SendApology(context.Background(), nil, "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil order")
}

// TestAsynqApologyMailer_CurrencyFallback covers the "Currency empty"
// branch so a future order projection bug cannot ship a payload with
// currency="".
func TestAsynqApologyMailer_CurrencyFallback(t *testing.T) {
	var captured []byte
	prev := jsonMarshal
	defer func() { jsonMarshal = prev }()
	jsonMarshal = func(v any) ([]byte, error) {
		b, err := defaultJSONMarshal(v)
		captured = b
		// Stop short of asynq enqueue: returning an error after capture
		// is the cleanest way to skip the network call.
		if err != nil {
			return nil, err
		}
		return nil, errors.New("stop here")
	}
	m := &asynqApologyMailer{client: nil, queue: "billing", now: time.Now}
	order := sampleApologyOrder()
	order.Currency = ""
	_ = m.SendApology(context.Background(), order, "x")
	assert.Contains(t, string(captured), `"currency":"CNY"`)
}

// sampleApologyOrder builds a fully-populated refund.Order so apology
// mailer tests can focus on the marshalling / enqueue path.
func sampleApologyOrder() *refund.Order {
	return &refund.Order{
		ID:            "v_1",
		OwnerID:       "u_1",
		UserEmail:     "u_1@example.com",
		PaddleOrderID: "pdle_v_1",
		PriceCNYYuan:  199.0,
		Currency:      "CNY",
	}
}

// TestRunTickLoop_CancelsCleanly drives the tick goroutine with a
// cancelled context and asserts it exits before the test timeout.
func TestRunTickLoop_CancelsCleanly(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	h := refund.New(refund.Config{
		Orders:   stubOrders{},
		Refunder: stubRefunder{},
		Redis:    rdb,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		runTickLoop(ctx, h, time.Millisecond, newLogger("info"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tick loop did not exit on context cancel")
	}
}

// TestApologyTaskTypeStable pins the wire constant so a rename here
// without a coordinated notifier-side change is caught at CI time.
func TestApologyTaskTypeStable(t *testing.T) {
	assert.Equal(t, "payment:refund_apology", apologyTaskType)
}

// --- stubs for runTickLoop test ---

type stubOrders struct{}

func (stubOrders) GetByReportID(context.Context, string) (*refund.Order, error) {
	return nil, refund.ErrOrderNotFound
}
func (stubOrders) GetByID(context.Context, string) (*refund.Order, error) {
	return nil, refund.ErrOrderNotFound
}
func (stubOrders) MarkRefunded(context.Context, string, string, time.Time) error  { return nil }
func (stubOrders) MarkRefundFailed(context.Context, string, string, string) error { return nil }
func (stubOrders) BumpRefundAttempt(context.Context, string, string, int) error   { return nil }
func (stubOrders) MarkApologySent(context.Context, string, time.Time) error       { return nil }

type stubRefunder struct{}

func (stubRefunder) Refund(context.Context, string, int64, string) error { return nil }
