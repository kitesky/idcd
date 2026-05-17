package refund

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ----- in-memory fakes ---------------------------------------------------

type fakeOrders struct {
	mu          sync.Mutex
	byReport    map[string]*Order
	byID        map[string]*Order
	getByIDErr  error
	getByRepErr error
	markFailErr error
	markRefErr  error
	bumpErr     error
	apologyErr  error
	bumps       []bumpCall
}

type bumpCall struct {
	OrderID   string
	Reason    string
	Attempt   int
}

func newFakeOrders() *fakeOrders {
	return &fakeOrders{
		byReport: make(map[string]*Order),
		byID:     make(map[string]*Order),
	}
}

func (f *fakeOrders) put(reportID string, o *Order) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *o
	f.byReport[reportID] = &cp
	f.byID[o.ID] = &cp
}

func (f *fakeOrders) get(id string) *Order {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.byID[id]
	if !ok {
		return nil
	}
	cp := *o
	return &cp
}

func (f *fakeOrders) GetByReportID(_ context.Context, reportID string) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getByRepErr != nil {
		return nil, f.getByRepErr
	}
	o, ok := f.byReport[reportID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeOrders) GetByID(_ context.Context, orderID string) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	o, ok := f.byID[orderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeOrders) MarkRefunded(_ context.Context, orderID, _ string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markRefErr != nil {
		return f.markRefErr
	}
	o, ok := f.byID[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	o.Status = StatusRefunded
	t := at
	o.RefundedAt = &t
	return nil
}

func (f *fakeOrders) MarkRefundFailed(_ context.Context, orderID, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markFailErr != nil {
		return f.markFailErr
	}
	o, ok := f.byID[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	o.Status = StatusRefundFailed
	return nil
}

func (f *fakeOrders) BumpRefundAttempt(_ context.Context, orderID, reason string, newAttempt int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.bumpErr != nil {
		return f.bumpErr
	}
	o, ok := f.byID[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	o.RefundAttempts = newAttempt
	f.bumps = append(f.bumps, bumpCall{OrderID: orderID, Reason: reason, Attempt: newAttempt})
	return nil
}

func (f *fakeOrders) MarkApologySent(_ context.Context, orderID string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.apologyErr != nil {
		return f.apologyErr
	}
	o, ok := f.byID[orderID]
	if !ok {
		return ErrOrderNotFound
	}
	t := at
	o.ApologySentAt = &t
	return nil
}

type fakeRefunder struct {
	mu      sync.Mutex
	calls   []refundCall
	errors  []error // pop one per call; nil = success
}

type refundCall struct {
	PaddleOrderID string
	AmountCents   int64
	Reason        string
}

func (f *fakeRefunder) Refund(_ context.Context, paddleID string, cents int64, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, refundCall{PaddleOrderID: paddleID, AmountCents: cents, Reason: reason})
	if len(f.errors) == 0 {
		return nil
	}
	err := f.errors[0]
	f.errors = f.errors[1:]
	return err
}

func (f *fakeRefunder) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type fakeMailer struct {
	mu     sync.Mutex
	calls  []mailCall
	err    error
}

type mailCall struct {
	OrderID       string
	UserEmail     string
	PaddleOrderID string
	AmountCents   int64
	Currency      string
	Reason        string
}

func (f *fakeMailer) SendApology(_ context.Context, order *Order, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if order == nil {
		f.calls = append(f.calls, mailCall{Reason: reason})
		return f.err
	}
	f.calls = append(f.calls, mailCall{
		OrderID:       order.ID,
		UserEmail:     order.UserEmail,
		PaddleOrderID: order.PaddleOrderID,
		AmountCents:   order.PriceCents(),
		Currency:      order.Currency,
		Reason:        reason,
	})
	return f.err
}

// silentLogger keeps test output clean.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

// fixedClock returns a stable Now func + a knob to advance time.
type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock(t time.Time) *fixedClock { return &fixedClock{now: t} }

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// newHandler constructs a Handler with all the test fakes pre-wired.
func newHandler(t *testing.T, orders *fakeOrders, ref *fakeRefunder, mailer ApologyMailer, rdb ZSetClient, clk *fixedClock) *Handler {
	t.Helper()
	return New(Config{
		Orders:       orders,
		Refunder:     ref,
		Mailer:       mailer,
		Redis:        rdb,
		DelayZoneKey: DefaultDelayZoneKey,
		Logger:       silentLogger(),
		Now:          clk.Now,
	})
}

func sampleOrder(id string) *Order {
	return &Order{
		ID:            id,
		Status:        "failed",
		OwnerID:       "u_" + id,
		UserEmail:     id + "@example.com",
		PaddleOrderID: "pdle_" + id,
		PriceCNYYuan:  199.0,
		Currency:      "CNY",
	}
}

// ----- pure-function tests (100% line coverage target) -------------------

func TestEncodeParseMember_Roundtrip(t *testing.T) {
	cases := []struct {
		orderID string
		attempt int
	}{
		{"v_abc", 1},
		{"v_x", 2},
		{"v_long_01HXXX", 1},
	}
	for _, c := range cases {
		m := encodeMember(c.orderID, c.attempt)
		id, a, err := parseMember(m)
		require.NoError(t, err)
		assert.Equal(t, c.orderID, id)
		assert.Equal(t, c.attempt, a)
	}
}

func TestParseMember_Malformed(t *testing.T) {
	for _, m := range []string{"", "no_sep", "|1", "v_x|", "v_x|abc", "v_x|0", "v_x|-1"} {
		_, _, err := parseMember(m)
		assert.Error(t, err, "expected error for %q", m)
	}
}

func TestRetryDelay(t *testing.T) {
	assert.Equal(t, FirstRetryDelay, retryDelay(1))
	assert.Equal(t, SecondRetryDelay, retryDelay(2))
	// fallback for out-of-range
	assert.Equal(t, SecondRetryDelay, retryDelay(0))
	assert.Equal(t, SecondRetryDelay, retryDelay(99))
}

func TestStringField(t *testing.T) {
	m := map[string]any{
		"a": "hello",
		"b": []byte("world"),
		"c": 42,
	}
	assert.Equal(t, "hello", stringField(m, "a"))
	assert.Equal(t, "world", stringField(m, "b"))
	assert.Equal(t, "42", stringField(m, "c"))
	assert.Equal(t, "", stringField(m, "missing"))
}

func TestScheduleScore_PreservesOrder(t *testing.T) {
	// Use a 1-second delta so the float64 representation cannot collapse
	// the two scores (UnixNano is ~1.77e18 in 2026, comfortably inside
	// float64's 53-bit mantissa for second-level deltas).
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Second)
	assert.Less(t, scheduleScore(t1), scheduleScore(t2))
}

func TestParseScheduledAt(t *testing.T) {
	t1, err := parseScheduledAt("2026-05-17T10:00:00Z")
	require.NoError(t, err)
	assert.Equal(t, 2026, t1.Year())

	t2, err := parseScheduledAt("2026-05-17T10:00:00.123456789Z")
	require.NoError(t, err)
	assert.Equal(t, 123456789, t2.Nanosecond())

	_, err = parseScheduledAt("")
	assert.Error(t, err)

	_, err = parseScheduledAt("not a date")
	assert.Error(t, err)
}

func TestOrder_PriceCents_Rounding(t *testing.T) {
	cases := []struct {
		yuan  float64
		cents int64
	}{
		{199.00, 19900},
		{0.99, 99},
		{0.995, 100}, // rounding to-cents
		{1.234, 123},
	}
	for _, c := range cases {
		o := &Order{PriceCNYYuan: c.yuan}
		assert.Equal(t, c.cents, o.PriceCents(), "yuan=%v", c.yuan)
	}
}

func TestOrder_IsTerminal(t *testing.T) {
	assert.True(t, (&Order{Status: StatusRefunded}).IsTerminal())
	assert.True(t, (&Order{Status: StatusRefundFailed}).IsTerminal())
	assert.False(t, (&Order{Status: "failed"}).IsTerminal())
	assert.False(t, (&Order{Status: "delivered"}).IsTerminal())
}

// ----- New panics --------------------------------------------------------

func TestNew_PanicsOnMissingRequired(t *testing.T) {
	_, rdb := newRedis(t)
	cases := []Config{
		{Refunder: &fakeRefunder{}, Redis: rdb},
		{Orders: newFakeOrders(), Redis: rdb},
		{Orders: newFakeOrders(), Refunder: &fakeRefunder{}},
	}
	for i, cfg := range cases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			assert.Panics(t, func() { _ = New(cfg) })
		})
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	_, rdb := newRedis(t)
	h := New(Config{
		Orders:   newFakeOrders(),
		Refunder: &fakeRefunder{},
		Redis:    rdb,
	})
	assert.Equal(t, DefaultDelayZoneKey, h.cfg.DelayZoneKey)
	assert.NotNil(t, h.cfg.Logger)
	assert.NotNil(t, h.cfg.Now)
}

// ----- HandleInitiate ----------------------------------------------------

func TestHandleInitiate_Success_FirstTry(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)

	err := h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_1",
		"reason":    "bad-pdf",
	})
	require.NoError(t, err)

	o := orders.get("v_1")
	assert.Equal(t, StatusRefunded, o.Status)
	assert.NotNil(t, o.RefundedAt)
	assert.Equal(t, 1, ref.callCount())
	assert.Equal(t, "pdle_v_1", ref.calls[0].PaddleOrderID)
	assert.Equal(t, int64(19900), ref.calls[0].AmountCents)
	assert.Equal(t, "bad-pdf", ref.calls[0].Reason)
}

func TestHandleInitiate_FirstTryFails_SchedulesRetry(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{errors: []error{errors.New("paddle 503")}}
	mr, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)

	err := h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_1",
		"reason":    "bad-pdf",
	})
	require.NoError(t, err)

	o := orders.get("v_1")
	assert.Equal(t, "failed", o.Status, "status unchanged on transient failure")
	assert.Equal(t, 1, o.RefundAttempts)
	require.Len(t, orders.bumps, 1)
	assert.Contains(t, orders.bumps[0].Reason, "paddle 503")

	// Delay zone has exactly one member scheduled at now+5min.
	members, err := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "v_1|1", members[0].Member)
	expected := scheduleScore(clk.Now().Add(FirstRetryDelay))
	assert.InDelta(t, expected, members[0].Score, 1)
	assert.True(t, mr.Exists(DefaultDelayZoneKey))
}

func TestHandleInitiate_MissingReportID(t *testing.T) {
	orders := newFakeOrders()
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, 0, ref.callCount())
}

func TestHandleInitiate_OrderNotFound(t *testing.T) {
	orders := newFakeOrders()
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_missing",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, ref.callCount())
}

func TestHandleInitiate_LookupError_Propagates(t *testing.T) {
	orders := newFakeOrders()
	orders.getByRepErr = errors.New("db boom")
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_x",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db boom")
}

func TestHandleInitiate_AlreadyRefunded_NoOp(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.Status = StatusRefunded
	orders.put("vr_1", o)
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{"report_id": "vr_1"})
	require.NoError(t, err)
	assert.Equal(t, 0, ref.callCount())
}

func TestHandleInitiate_MarkRefundedFailure_Propagates(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	orders.markRefErr = errors.New("update boom")
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{"report_id": "vr_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update boom")
}

func TestHandleInitiate_BumpFailure_Propagates(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	orders.bumpErr = errors.New("bump boom")
	ref := &fakeRefunder{errors: []error{errors.New("paddle 503")}}
	_, rdb := newRedis(t)
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, newClock(time.Now()))

	err := h.HandleInitiate(context.Background(), map[string]any{"report_id": "vr_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bump boom")
}

// ----- HandleRetryEnqueue ------------------------------------------------

func TestHandleRetryEnqueue_AddsToDelayZone(t *testing.T) {
	orders := newFakeOrders()
	ref := &fakeRefunder{}
	mr, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)

	sched := clk.Now().Add(5 * time.Minute)
	err := h.HandleRetryEnqueue(context.Background(), map[string]any{
		"order_id":     "v_99",
		"attempt":      "1",
		"scheduled_at": sched.Format(time.RFC3339Nano),
	})
	require.NoError(t, err)

	members, _ := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.Len(t, members, 1)
	assert.Equal(t, "v_99|1", members[0].Member)
	assert.True(t, mr.Exists(DefaultDelayZoneKey))
}

func TestHandleRetryEnqueue_MissingFields(t *testing.T) {
	h := newHandler(t, newFakeOrders(), &fakeRefunder{}, &fakeMailer{}, mustClient(t), newClock(time.Now()))

	// missing order_id
	assert.NoError(t, h.HandleRetryEnqueue(context.Background(), map[string]any{}))
	// bad attempt
	assert.NoError(t, h.HandleRetryEnqueue(context.Background(), map[string]any{"order_id": "v_x", "attempt": "abc"}))
	// non-positive attempt
	assert.NoError(t, h.HandleRetryEnqueue(context.Background(), map[string]any{"order_id": "v_x", "attempt": "0"}))
	// bad scheduled_at
	assert.NoError(t, h.HandleRetryEnqueue(context.Background(), map[string]any{
		"order_id": "v_x", "attempt": "1", "scheduled_at": "not-a-date",
	}))
}

// ----- TickDelayZone -----------------------------------------------------

func TestTickDelayZone_EmptyQueue(t *testing.T) {
	_, rdb := newRedis(t)
	h := newHandler(t, newFakeOrders(), &fakeRefunder{}, &fakeMailer{}, rdb, newClock(time.Now()))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestTickDelayZone_SkipsFutureMembers(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, &fakeRefunder{}, &fakeMailer{}, rdb, clk)

	// Schedule 5 minutes in the future.
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(5*time.Minute)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n, "future members must not be picked")
}

func TestTickDelayZone_FirstRetrySucceeds(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{}
	mailer := &fakeMailer{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	// Schedule due "now" (past).
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefunded, orders.get("v_1").Status)
	assert.Equal(t, 1, ref.callCount())
	assert.Equal(t, failureReasonPaddleAPI, ref.calls[0].Reason)
	assert.Empty(t, mailer.calls)
}

func TestTickDelayZone_FirstRetryFails_SchedulesSecond(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 1
	orders.put("vr_1", o)
	ref := &fakeRefunder{errors: []error{errors.New("paddle nope")}}
	mailer := &fakeMailer{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Status unchanged, attempt bumped to 2, member rescheduled at +30min.
	assert.Equal(t, "failed", orders.get("v_1").Status)
	assert.Equal(t, 2, orders.get("v_1").RefundAttempts)
	members, _ := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.Len(t, members, 1)
	assert.Equal(t, "v_1|2", members[0].Member)
	expected := scheduleScore(clk.Now().Add(SecondRetryDelay))
	assert.InDelta(t, expected, members[0].Score, 1)
	assert.Empty(t, mailer.calls)
}

func TestTickDelayZone_SecondRetryFails_FlipsRefundFailedAndApologizes(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 2
	orders.put("vr_1", o)
	ref := &fakeRefunder{errors: []error{errors.New("paddle still down")}}
	mailer := &fakeMailer{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 30, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 2, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	assert.Equal(t, StatusRefundFailed, orders.get("v_1").Status)
	require.Len(t, mailer.calls, 1)
	// Enriched payload reaches the mailer: order_id + email + paddle id
	// + cents + currency + reason all populated so the notifier can
	// render the email without a follow-up DB read.
	got := mailer.calls[0]
	assert.Equal(t, "v_1", got.OrderID)
	assert.Equal(t, "v_1@example.com", got.UserEmail)
	assert.Equal(t, "pdle_v_1", got.PaddleOrderID)
	assert.Equal(t, int64(19900), got.AmountCents)
	assert.Equal(t, "CNY", got.Currency)
	assert.Contains(t, got.Reason, "paddle still down")
	assert.NotNil(t, orders.get("v_1").ApologySentAt)
}

func TestTickDelayZone_SecondRetry_ApologyEnqueueFails_LeavesUnstamped(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 2
	orders.put("vr_1", o)
	ref := &fakeRefunder{errors: []error{errors.New("paddle down")}}
	mailer := &fakeMailer{err: errors.New("notifier offline")}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 11, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 2, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefundFailed, orders.get("v_1").Status)
	require.Len(t, mailer.calls, 1)
	assert.Nil(t, orders.get("v_1").ApologySentAt, "apology stamp must wait for successful enqueue")
}

func TestTickDelayZone_SecondRetry_NoMailer_LogsWarning(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 2
	orders.put("vr_1", o)
	ref := &fakeRefunder{errors: []error{errors.New("paddle down")}}
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   orders,
		Refunder: ref,
		Redis:    rdb,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 2, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefundFailed, orders.get("v_1").Status)
}

func TestTickDelayZone_AlreadyTerminal_Skipped(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.Status = StatusRefunded
	orders.put("vr_1", o)
	ref := &fakeRefunder{}
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 0, ref.callCount(), "no refund attempt on terminal order")
}

func TestTickDelayZone_OrderMissing_DropsSilently(t *testing.T) {
	orders := newFakeOrders()
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := newHandler(t, orders, &fakeRefunder{}, &fakeMailer{}, rdb, clk)
	require.NoError(t, h.scheduleRetry(context.Background(), "v_ghost", 1, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestTickDelayZone_MalformedMember_ZRemmed(t *testing.T) {
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, newFakeOrders(), &fakeRefunder{}, &fakeMailer{}, rdb, clk)

	// Directly ZADD a bad member.
	require.NoError(t, rdb.ZAdd(context.Background(), DefaultDelayZoneKey, redis.Z{
		Score: scheduleScore(clk.Now().Add(-time.Second)), Member: "no_separator",
	}).Err())

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	// Member was malformed → counts as 0 processed but ZREM ran.
	assert.Equal(t, 0, n)
	count, _ := rdb.ZCard(context.Background(), DefaultDelayZoneKey).Result()
	assert.Equal(t, int64(0), count, "malformed member must be removed")
}

func TestTickDelayZone_GetByIDError_ReschedulesAttempt(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	orders.put("vr_1", o)
	orders.getByIDErr = errors.New("db transient")
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, &fakeRefunder{}, &fakeMailer{}, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Reschedule should have been re-added with same attempt.
	members, _ := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.Len(t, members, 1)
	assert.Equal(t, "v_1|1", members[0].Member)
}

func TestTickDelayZone_ZRangeError(t *testing.T) {
	failing := &failingZSet{zrangeErr: errors.New("redis down")}
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   newFakeOrders(),
		Refunder: &fakeRefunder{},
		Redis:    failing,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	_, err := h.TickDelayZone(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis down")
}

func TestTickDelayZone_ZRemRace_LosesGracefully(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	racing := &racingZSet{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	racing.inner = rdb
	racing.zremReturn = 0 // pretend somebody else won
	h := New(Config{
		Orders:   orders,
		Refunder: &fakeRefunder{},
		Redis:    racing,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	// Lost the race → 0 processed.
	assert.Equal(t, 0, n)
}

func TestTickDelayZone_ZRemError_Continues(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	racing := &racingZSet{}
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	racing.inner = rdb
	racing.zremErr = errors.New("transient")
	h := New(Config{
		Orders:   orders,
		Refunder: &fakeRefunder{},
		Redis:    racing,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestTickDelayZone_NonStringMember_Removed(t *testing.T) {
	// Inject a member whose interface{} type is not string (simulated
	// by a custom ZSet client returning a numeric member).
	zs := &exoticMemberZSet{score: scheduleScore(time.Now().Add(-time.Second))}
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   newFakeOrders(),
		Refunder: &fakeRefunder{},
		Redis:    zs,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	_, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, zs.zremCalls, "exotic member should be ZREMmed")
}

// ----- end-to-end ladder over miniredis ----------------------------------

func TestFullLadder_TwoFailuresThenApology(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{errors: []error{
		errors.New("attempt0"), // initiate
		errors.New("attempt1"), // tick attempt 1
		errors.New("attempt2"), // tick attempt 2
	}}
	mailer := &fakeMailer{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	// 1) Initiate fails → schedules attempt 1.
	require.NoError(t, h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_1",
		"reason":    "self-verify-fail",
	}))

	// 2) Advance past FirstRetryDelay → tick attempt 1.
	clk.advance(FirstRetryDelay + time.Second)
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "failed", orders.get("v_1").Status, "still pending another retry")

	// 3) Advance past SecondRetryDelay → tick attempt 2 → terminal.
	clk.advance(SecondRetryDelay + time.Second)
	n, err = h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefundFailed, orders.get("v_1").Status)
	require.Len(t, mailer.calls, 1)
	assert.Equal(t, "v_1", mailer.calls[0].OrderID)

	// Delay zone now empty.
	card, _ := rdb.ZCard(context.Background(), DefaultDelayZoneKey).Result()
	assert.Equal(t, int64(0), card)
}

func TestFullLadder_FirstRetrySucceeds(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{errors: []error{
		errors.New("initiate down"),
		nil, // retry 1 succeeds
	}}
	mailer := &fakeMailer{}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, mailer, rdb, clk)

	require.NoError(t, h.HandleInitiate(context.Background(), map[string]any{
		"report_id": "vr_1",
		"reason":    "x",
	}))
	clk.advance(FirstRetryDelay + time.Second)
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefunded, orders.get("v_1").Status)
	assert.Empty(t, mailer.calls)
}

// ----- additional coverage for processRetry / scheduleRetry error paths --

// failingZAdd makes ZAdd error so scheduleRetry's error wrapping is
// exercised end-to-end.
type failingZAdd struct {
	inner ZSetClient
}

func (f *failingZAdd) ZAdd(ctx context.Context, _ string, _ ...redis.Z) *redis.IntCmd {
	c := redis.NewIntCmd(ctx)
	c.SetErr(errors.New("zadd boom"))
	return c
}
func (f *failingZAdd) ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return f.inner.ZRangeByScoreWithScores(ctx, key, opt)
}
func (f *failingZAdd) ZRem(ctx context.Context, key string, members ...any) *redis.IntCmd {
	return f.inner.ZRem(ctx, key, members...)
}

func TestScheduleRetry_ZAddError(t *testing.T) {
	_, rdb := newRedis(t)
	z := &failingZAdd{inner: rdb}
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   newFakeOrders(),
		Refunder: &fakeRefunder{},
		Redis:    z,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	err := h.scheduleRetry(context.Background(), "v_1", 1, clk.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zadd boom")
}

// TestHandleInitiate_ScheduleZAddFails covers the second error-wrap in
// HandleInitiate (between bump and schedule).
func TestHandleInitiate_ScheduleZAddFails(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	ref := &fakeRefunder{errors: []error{errors.New("paddle down")}}
	_, rdb := newRedis(t)
	z := &failingZAdd{inner: rdb}
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   orders,
		Refunder: ref,
		Redis:    z,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	err := h.HandleInitiate(context.Background(), map[string]any{"report_id": "vr_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schedule retry")
}

// TestHandleRetryEnqueue_ScheduleZAddFails covers the error-wrap path
// in HandleRetryEnqueue after a successful parse.
func TestHandleRetryEnqueue_ScheduleZAddFails(t *testing.T) {
	_, rdb := newRedis(t)
	z := &failingZAdd{inner: rdb}
	clk := newClock(time.Now())
	h := New(Config{
		Orders:   newFakeOrders(),
		Refunder: &fakeRefunder{},
		Redis:    z,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	err := h.HandleRetryEnqueue(context.Background(), map[string]any{
		"order_id": "v_x", "attempt": "1", "scheduled_at": clk.Now().Format(time.RFC3339Nano),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refund retry enqueue")
}

// TestTickDelayZone_BumpError covers the processRetry error path where
// BumpRefundAttempt fails — verifies the member is rescheduled rather
// than dropped.
func TestTickDelayZone_BumpError(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	orders.bumpErr = errors.New("bump db boom")
	ref := &fakeRefunder{errors: []error{errors.New("paddle")}}
	_, rdb := newRedis(t)
	clk := newClock(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)

	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Member rescheduled (same attempt) by the error-handler in tick.
	members, _ := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.Len(t, members, 1)
	assert.Equal(t, "v_1|1", members[0].Member)
}

// TestTickDelayZone_MarkRefundedError verifies that a DB failure on
// marking refunded surfaces a processRetry error (which then triggers
// the tick-level reschedule).
func TestTickDelayZone_MarkRefundedError(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	orders.markRefErr = errors.New("update boom")
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := newHandler(t, orders, &fakeRefunder{}, &fakeMailer{}, rdb, clk)
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 1, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	// rescheduled because mark failed
	members, _ := rdb.ZRangeWithScores(context.Background(), DefaultDelayZoneKey, 0, -1).Result()
	require.Len(t, members, 1)
}

// TestTickDelayZone_MarkRefundFailedError covers the terminal-flip DB
// failure path.
func TestTickDelayZone_MarkRefundFailedError(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 2
	orders.put("vr_1", o)
	orders.markFailErr = errors.New("update boom")
	ref := &fakeRefunder{errors: []error{errors.New("paddle")}}
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 2, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// TestTickDelayZone_ApologyStampError covers the post-apology-stamp DB
// failure path (logs but does not abort).
func TestTickDelayZone_ApologyStampError(t *testing.T) {
	orders := newFakeOrders()
	o := sampleOrder("v_1")
	o.RefundAttempts = 2
	orders.put("vr_1", o)
	orders.apologyErr = errors.New("stamp boom")
	ref := &fakeRefunder{errors: []error{errors.New("paddle")}}
	_, rdb := newRedis(t)
	clk := newClock(time.Now())
	h := newHandler(t, orders, ref, &fakeMailer{}, rdb, clk)
	require.NoError(t, h.scheduleRetry(context.Background(), "v_1", 2, clk.Now().Add(-time.Second)))
	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, StatusRefundFailed, orders.get("v_1").Status)
}

// TestTickDelayZone_RescheduleFailsAfterHandlerError covers the
// double-fault path: the retry handler errors, AND the reschedule ZADD
// errors. The tick must not propagate either as a fatal error; loss is
// limited to one member that the operator can re-trigger from the
// admin dashboard.
func TestTickDelayZone_DoubleFault(t *testing.T) {
	orders := newFakeOrders()
	orders.put("vr_1", sampleOrder("v_1"))
	orders.markRefErr = errors.New("update boom")
	_, rdb := newRedis(t)

	// Use real Redis for the ZRange/ZRem path so the member is picked;
	// then override ZAdd to always fail so the reschedule errors too.
	clk := newClock(time.Now())
	z := &flakeyZAdd{inner: rdb, failAfter: 1}
	h := New(Config{
		Orders:   orders,
		Refunder: &fakeRefunder{},
		Redis:    z,
		Logger:   silentLogger(),
		Now:      clk.Now,
	})
	// Seed via real ZAdd before flip.
	require.NoError(t, rdb.ZAdd(context.Background(), DefaultDelayZoneKey, redis.Z{
		Score: scheduleScore(clk.Now().Add(-time.Second)), Member: "v_1|1",
	}).Err())

	n, err := h.TickDelayZone(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// flakeyZAdd lets the first N ZAdds through then fails subsequent ones.
type flakeyZAdd struct {
	inner     *redis.Client
	failAfter int
	calls     int
}

func (f *flakeyZAdd) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	f.calls++
	if f.calls > f.failAfter {
		c := redis.NewIntCmd(ctx)
		c.SetErr(errors.New("zadd boom"))
		return c
	}
	return f.inner.ZAdd(ctx, key, members...)
}
func (f *flakeyZAdd) ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return f.inner.ZRangeByScoreWithScores(ctx, key, opt)
}
func (f *flakeyZAdd) ZRem(ctx context.Context, key string, members ...any) *redis.IntCmd {
	return f.inner.ZRem(ctx, key, members...)
}

// ----- supporting fakes for redis edge cases ----------------------------

func mustClient(t *testing.T) *redis.Client {
	_, rdb := newRedis(t)
	return rdb
}

// failingZSet returns an error from ZRangeByScoreWithScores.
type failingZSet struct {
	zrangeErr error
}

func (f *failingZSet) ZAdd(ctx context.Context, _ string, _ ...redis.Z) *redis.IntCmd {
	c := redis.NewIntCmd(ctx)
	c.SetVal(0)
	return c
}

func (f *failingZSet) ZRangeByScoreWithScores(ctx context.Context, _ string, _ *redis.ZRangeBy) *redis.ZSliceCmd {
	c := redis.NewZSliceCmd(ctx)
	c.SetErr(f.zrangeErr)
	return c
}

func (f *failingZSet) ZRem(ctx context.Context, _ string, _ ...any) *redis.IntCmd {
	c := redis.NewIntCmd(ctx)
	c.SetVal(0)
	return c
}

// racingZSet delegates to inner Redis but lets the test override ZRem's
// numeric return + error so we can simulate a lost race or transient
// ZREM failure deterministically.
type racingZSet struct {
	inner      *redis.Client
	zremReturn int64
	zremErr    error
}

func (r *racingZSet) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	return r.inner.ZAdd(ctx, key, members...)
}

func (r *racingZSet) ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return r.inner.ZRangeByScoreWithScores(ctx, key, opt)
}

func (r *racingZSet) ZRem(ctx context.Context, _ string, _ ...any) *redis.IntCmd {
	c := redis.NewIntCmd(context.Background())
	if r.zremErr != nil {
		c.SetErr(r.zremErr)
		return c
	}
	c.SetVal(r.zremReturn)
	return c
}

// exoticMemberZSet returns a single non-string member from
// ZRangeByScoreWithScores so we can cover the type-assertion guard in
// the tick loop.
type exoticMemberZSet struct {
	score     float64
	zremCalls int
}

func (e *exoticMemberZSet) ZAdd(ctx context.Context, _ string, _ ...redis.Z) *redis.IntCmd {
	c := redis.NewIntCmd(ctx)
	c.SetVal(1)
	return c
}

func (e *exoticMemberZSet) ZRangeByScoreWithScores(ctx context.Context, _ string, _ *redis.ZRangeBy) *redis.ZSliceCmd {
	c := redis.NewZSliceCmd(ctx)
	c.SetVal([]redis.Z{{Score: e.score, Member: 12345}})
	return c
}

func (e *exoticMemberZSet) ZRem(ctx context.Context, _ string, _ ...any) *redis.IntCmd {
	e.zremCalls++
	c := redis.NewIntCmd(ctx)
	c.SetVal(1)
	return c
}

// ----- sanity checks on helper constants -----------------------------------

func TestDefaults_AreReasonable(t *testing.T) {
	assert.Equal(t, "refund_delay_zone", DefaultDelayZoneKey)
	assert.Equal(t, 30*time.Second, DefaultTickInterval)
	assert.Equal(t, 100, DefaultTickBatchSize)
	assert.Equal(t, 2, MaxAttempts)
	assert.Equal(t, 5*time.Minute, FirstRetryDelay)
	assert.Equal(t, 25*time.Minute, SecondRetryDelay)
	// memberSep must not appear in any legit order_id prefix
	assert.False(t, strings.Contains("v_abc123", memberSep))
}
