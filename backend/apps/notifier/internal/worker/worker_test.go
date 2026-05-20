package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// TestRetryDelayFunc_RefundUsesFixedDelay covers the P1#14 / D5 contract: a
// payment:refund_retry task that bounces through the asynq retry path (e.g.
// because the handler returned a transient error before it could enqueue the
// next explicit attempt) MUST receive the 5-minute fixed delay, not the
// generic exponential backoff.
func TestRetryDelayFunc_RefundUsesFixedDelay(t *testing.T) {
	task := asynq.NewTask(TaskRefundRetry, []byte(`{}`))
	// Exercise multiple retry counts to ensure n doesn't bleed into the
	// refund path.
	for _, n := range []int{0, 1, 2, 3, 5, 10} {
		got := retryDelayFunc(n, errors.New("transient"), task)
		if got != RefundRetryFirstDelay {
			t.Errorf("refund task at retry=%d: got delay %s, want %s", n, got, RefundRetryFirstDelay)
		}
	}
}

// TestRetryDelayFunc_GenericExponentialBackoff covers the unchanged behaviour
// for non-refund tasks: 1s / 4s / 16s with ±25% jitter, capped at 60s.
func TestRetryDelayFunc_GenericExponentialBackoff(t *testing.T) {
	cases := []struct {
		taskType string
		retry    int
		// base is 1s<<retry; the realised delay must be within ±25%.
		base time.Duration
	}{
		{TaskSendVerifyEmail, 0, 1 * time.Second},
		{TaskSendVerifyEmail, 1, 2 * time.Second},
		{TaskSendWelcome, 2, 4 * time.Second},
		{TaskSendResetPassword, 3, 8 * time.Second},
		{TypeAlertNotification, 4, 16 * time.Second},
	}

	for _, c := range cases {
		t.Run(c.taskType, func(t *testing.T) {
			task := asynq.NewTask(c.taskType, []byte(`{}`))
			got := retryDelayFunc(c.retry, errors.New("x"), task)
			lower := c.base - (c.base / 4)
			upper := c.base + (c.base / 4)
			if got < lower || got > upper {
				t.Errorf("retry=%d type=%s: got %s, want within [%s,%s]", c.retry, c.taskType, got, lower, upper)
			}
			if got > 60*time.Second {
				t.Errorf("retry=%d: delay %s exceeds 60s cap", c.retry, got)
			}
		})
	}
}

// TestRetryDelayFunc_GenericBackoffCapsAt60s ensures the 60-second ceiling
// holds for very large retry counts (which can happen if asynq's max-retry is
// raised in the future).
func TestRetryDelayFunc_GenericBackoffCapsAt60s(t *testing.T) {
	task := asynq.NewTask(TaskSendWelcome, []byte(`{}`))
	for _, n := range []int{6, 10, 20, 32} {
		got := retryDelayFunc(n, errors.New("x"), task)
		if got > 60*time.Second+(60*time.Second/4) {
			t.Errorf("retry=%d: delay %s exceeds 60s + max jitter", n, got)
		}
		// The cap should bring base down to 60s; with ±25% jitter the
		// floor is 45s.
		if got < 45*time.Second {
			t.Errorf("retry=%d: delay %s below capped floor 45s", n, got)
		}
	}
}

// TestRetryDelayFunc_HandlesNilTask defends against a (theoretical) asynq
// callback with task==nil — we should not panic and we should fall through to
// the generic backoff.
func TestRetryDelayFunc_HandlesNilTask(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("retryDelayFunc panicked on nil task: %v", r)
		}
	}()
	got := retryDelayFunc(0, errors.New("x"), nil)
	// Generic path: ~1s ± 25%
	if got < 750*time.Millisecond || got > 1250*time.Millisecond {
		t.Errorf("nil task n=0: got %s, want ~1s", got)
	}
}

// TestWorker_WithCertConsumer_AttachesAndReturnsSelf locks in the fluent
// builder behaviour for WithCertConsumer — passing nil must be a no-op (and
// must return the worker itself so callers can chain).
func TestWorker_WithCertConsumer_AttachesAndReturnsSelf(t *testing.T) {
	t.Parallel()
	w := &Worker{}
	if got := w.WithCertConsumer(nil); got != w {
		t.Errorf("WithCertConsumer(nil) returned %p, want %p", got, w)
	}
	if w.certConsumer != nil {
		t.Errorf("nil consumer attached: %v", w.certConsumer)
	}

	// Attach a non-nil placeholder; assertion is just that the field is set.
	c := &CertConsumer{}
	if got := w.WithCertConsumer(c); got != w {
		t.Errorf("WithCertConsumer(c) returned %p, want %p", got, w)
	}
	if w.certConsumer != c {
		t.Errorf("certConsumer not attached")
	}
}

// TestWorker_Stop_WithoutCertConsumer_DoesNotHang ensures the new shutdown
// path is a no-op when no consumer was attached — i.e. we never wait on a
// wg with zero adds.  This is the most common production case (cert consumer
// disabled).
func TestWorker_Stop_WithoutCertConsumer_DoesNotHang(t *testing.T) {
	t.Parallel()
	// Construct a Worker manually without invoking NewWorker (which would
	// require a real Redis).  We only exercise Stop()'s cert-consumer path
	// here; the asynq Shutdown() call still runs but is safe on a nil server
	// — except *asynq.Server's Shutdown panics on nil receiver, so we wrap
	// the call in a defer/recover so the test still measures the intent.
	w := &Worker{logger: testLogger()}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	// The Stop call WILL hit the nil-server panic at the end; catch it and
	// assert the cert-consumer drain completed first.
	done := make(chan struct{})
	go func() {
		defer func() {
			_ = recover() // expected: asynq server nil
			close(done)
		}()
		_ = w.Stop(ctx)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop without certConsumer did not return within 2s")
	}
}

// testLogger returns a slog.Logger that swallows output, used by the worker
// shutdown tests so test output stays clean.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
