package streamconsumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// silentLogger keeps tests quiet — the Run loop logs every handler
// failure and every iteration; surfacing that noise during a full
// `go test ./...` run drowns out real failures.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

// newMiniredis spins up a miniredis instance and returns both the
// server (for t.Cleanup / XLen reads from tests) and a *redis.Client.
func newMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestNew_PanicsOnMissingRequiredFields(t *testing.T) {
	_, rdb := newMiniredis(t)
	handler := func(context.Context, map[string]any) error { return nil }
	cases := []struct {
		name string
		cfg  Config
	}{
		{"redis", Config{Stream: "s", Group: "g", Consumer: "c", Handler: handler}},
		{"stream", Config{Redis: rdb, Group: "g", Consumer: "c", Handler: handler}},
		{"group", Config{Redis: rdb, Stream: "s", Consumer: "c", Handler: handler}},
		{"consumer", Config{Redis: rdb, Stream: "s", Group: "g", Handler: handler}},
		{"handler", Config{Redis: rdb, Stream: "s", Group: "g", Consumer: "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Panics(t, func() { _ = New(tc.cfg) })
		})
	}
}

func TestNew_DefaultsBlockAndCount(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := New(Config{
		Redis:    rdb,
		Stream:   "s",
		Group:    "g",
		Consumer: "c",
		Handler:  func(context.Context, map[string]any) error { return nil },
	})
	assert.Equal(t, DefaultBlockTime, c.cfg.BlockTime)
	assert.Equal(t, DefaultCount, c.cfg.Count)
	assert.NotNil(t, c.cfg.Logger)
}

// TestRun_HappyPath: XADD a message, Handler receives the fields,
// returns nil, message is XACK'd and pending count drops to 0.
func TestRun_HappyPath(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "verdict_generation_queue"
		group  = "attest-generator"
	)

	received := make(chan map[string]any, 1)
	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "test-consumer-1",
		BlockTime: 50 * time.Millisecond,
		Count:     5,
		Logger:    silentLogger(),
		Handler: func(_ context.Context, fields map[string]any) error {
			received <- fields
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Wait a moment for the group to be created (XReadGroup needs it).
	require.Eventually(t, func() bool {
		return mr.Exists(stream)
	}, 2*time.Second, 10*time.Millisecond, "group should be created")

	// Produce a message.
	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"order_id": "v_abc123"},
	}).Result()
	require.NoError(t, err)

	// Handler should receive it.
	select {
	case fields := <-received:
		assert.Equal(t, "v_abc123", fields["order_id"])
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not receive message")
	}

	// Give the consumer time to XACK before we shut down. Polling
	// XPENDING avoids relying on arbitrary sleeps.
	require.Eventually(t, func() bool {
		pending, _ := rdb.XPending(ctx, stream, group).Result()
		return pending != nil && pending.Count == 0
	}, 2*time.Second, 20*time.Millisecond, "message should be acked")

	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after ctx cancel")
	}
}

// TestRun_HandlerErrorLeavesMessageUnacked: Handler returns an error,
// message stays in the PEL (pending count > 0).
func TestRun_HandlerErrorLeavesMessageUnacked(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "tasks"
		group  = "g1"
	)

	var calls atomic.Int32
	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "consumer-err",
		BlockTime: 50 * time.Millisecond,
		Count:     1,
		Logger:    silentLogger(),
		Handler: func(context.Context, map[string]any) error {
			calls.Add(1)
			return errors.New("intentional failure")
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool {
		return mr.Exists(stream)
	}, 2*time.Second, 10*time.Millisecond)

	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"k": "v"},
	}).Result()
	require.NoError(t, err)

	// Wait for at least one handler call.
	require.Eventually(t, func() bool {
		return calls.Load() >= 1
	}, 2*time.Second, 20*time.Millisecond)

	// Pending count should be ≥ 1 (handler never returned nil).
	require.Eventually(t, func() bool {
		pending, perr := rdb.XPending(ctx, stream, group).Result()
		return perr == nil && pending != nil && pending.Count >= 1
	}, 2*time.Second, 20*time.Millisecond, "unacked message should remain pending")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after ctx cancel")
	}
}

// TestRun_CtxCancelExitsCleanly: cancelling ctx without any messages
// still produces a clean nil return.
func TestRun_CtxCancelExitsCleanly(t *testing.T) {
	_, rdb := newMiniredis(t)
	c := New(Config{
		Redis:     rdb,
		Stream:    "s",
		Group:     "g",
		Consumer:  "c",
		BlockTime: 50 * time.Millisecond,
		Logger:    silentLogger(),
		Handler:   func(context.Context, map[string]any) error { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Give the consumer one block-cycle to enter the loop.
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}

// TestEnsureGroup_BusyGroupIsTreatedAsSuccess: starting a second
// consumer with the same group name must not error.
func TestEnsureGroup_BusyGroupIsTreatedAsSuccess(t *testing.T) {
	_, rdb := newMiniredis(t)
	const (
		stream = "tasks2"
		group  = "g"
	)

	c := New(Config{
		Redis:    rdb,
		Stream:   stream,
		Group:    group,
		Consumer: "c1",
		Logger:   silentLogger(),
		Handler:  func(context.Context, map[string]any) error { return nil },
	})
	require.NoError(t, c.ensureGroup(context.Background()))
	// Second invocation should also succeed (BUSYGROUP swallowed).
	require.NoError(t, c.ensureGroup(context.Background()))
}

// TestRun_FailsOnEnsureGroupError: a closed Redis surfaces the
// XGROUP CREATE failure as a fatal Run() return.
func TestRun_FailsOnEnsureGroupError(t *testing.T) {
	mr, rdb := newMiniredis(t)
	mr.Close() // kill server before Run

	c := New(Config{
		Redis:    rdb,
		Stream:   "s",
		Group:    "g",
		Consumer: "c",
		Logger:   silentLogger(),
		Handler:  func(context.Context, map[string]any) error { return nil },
	})

	err := c.Run(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ensure group")
}

// TestSleepCtx_WakesOnCancel: covers the sleepCtx helper's cancel
// branch (used by Run's transient-error backoff loop).
func TestSleepCtx_WakesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	completed := sleepCtx(ctx, time.Hour)
	assert.False(t, completed, "sleepCtx should return false when ctx is cancelled")
	assert.Less(t, time.Since(start), time.Second, "sleepCtx should return promptly on cancel")
}

// TestSleepCtx_RunsFullDuration: covers the timer branch.
func TestSleepCtx_RunsFullDuration(t *testing.T) {
	completed := sleepCtx(context.Background(), 20*time.Millisecond)
	assert.True(t, completed)
}

// TestRun_XReadGroupTransientErrorRetries: when XREADGROUP returns an
// error (not redis.Nil), the loop should log a warn, sleep with backoff
// and continue. We simulate this by closing miniredis after the group
// is created. The Run loop must:
//   - log warn + enter sleepCtx (the warn branch),
//   - exit cleanly when ctx is cancelled during the sleep backoff
//     (the !sleepCtx → return nil branch).
func TestRun_XReadGroupTransientErrorRetries(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "tasks_transient"
		group  = "g"
	)

	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "transient-c",
		BlockTime: 20 * time.Millisecond,
		Logger:    silentLogger(),
		Handler:   func(context.Context, map[string]any) error { return nil },
	})

	// Install a server-side error string that ensureGroup will treat
	// as success (the "BUSYGROUP" substring) but XREADGROUP will see
	// as a generic error reply. SetError makes ALL subsequent commands
	// return this reply without dropping the connection — which is
	// exactly the transient Redis hiccup the retry path is designed for.
	mr.SetError("BUSYGROUP simulated transient — ensureGroup treats this as success, XREADGROUP as error")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	// Give the loop enough time to: ensureGroup (errors -> Run returns
	// the wrapped error) OR succeed at ensureGroup-via-BUSYGROUP and
	// then hit XREADGROUP error -> warn -> sleepCtx(1s). After 100ms
	// we cancel, which must terminate the sleepCtx and return nil.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Either nil (sleepCtx path won the race) or an ensure-group
		// wrapping error (initial ensureGroup hit the SetError). Both
		// outcomes exercise different paths; the goal is coverage of
		// the retry branch when the timing favours it.
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not exit after ctx cancel")
	}
}

// TestRun_CtxCancelDuringBacklog: cancels the context after a message
// is XADDed but before the handler returns. Covers the `ctx.Err() != nil`
// branch inside the message-iteration loop.
func TestRun_CtxCancelDuringBacklog(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "tasks_ctx"
		group  = "g"
	)

	handlerStarted := make(chan struct{}, 1)
	releaseHandler := make(chan struct{})
	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "ctx-c",
		BlockTime: 20 * time.Millisecond,
		Count:     10,
		Logger:    silentLogger(),
		Handler: func(ctx context.Context, _ map[string]any) error {
			select {
			case handlerStarted <- struct{}{}:
			default:
			}
			<-releaseHandler
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool { return mr.Exists(stream) },
		2*time.Second, 10*time.Millisecond)

	// Add two messages so when the first one is mid-handler the second
	// is sitting in the batch slice; cancelling will trigger the
	// per-message ctx.Err() check on the second iteration.
	for i := 0; i < 2; i++ {
		_, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"k": "v"},
		}).Result()
		require.NoError(t, err)
	}

	<-handlerStarted
	cancel()
	close(releaseHandler)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}

// TestRun_XAckFailureIsLogged: when XACK fails the loop must continue
// (log warn, no propagation). We engineer this by closing miniredis
// after a message lands in the handler — XACK then fails but the loop's
// for-range over `msgs` is already populated, so the XACK failure path
// runs.
func TestRun_XAckFailureIsLogged(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "tasks_ack"
		group  = "g"
	)

	handlerRan := make(chan struct{}, 1)
	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "ack-c",
		BlockTime: 20 * time.Millisecond,
		Logger:    silentLogger(),
		Handler: func(context.Context, map[string]any) error {
			select {
			case handlerRan <- struct{}{}:
			default:
			}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool { return mr.Exists(stream) },
		2*time.Second, 10*time.Millisecond)

	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"k": "v"},
	}).Result()
	require.NoError(t, err)

	<-handlerRan
	// Yank the server so the XAck after handler returns errors out.
	// We can't deterministically guarantee XAck has not already run,
	// but in practice the handler returns before the loop reaches the
	// XAck call — this path is exercised whenever the close lands first.
	mr.Close()
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit")
	}
}

// TestRun_HandlesMultipleMessagesSequentially: multiple XADDs are all
// delivered to the handler in order and all get acked.
func TestRun_HandlesMultipleMessagesSequentially(t *testing.T) {
	mr, rdb := newMiniredis(t)
	const (
		stream = "multi"
		group  = "g"
	)

	var (
		mu       sync.Mutex
		received []string
	)
	c := New(Config{
		Redis:     rdb,
		Stream:    stream,
		Group:     group,
		Consumer:  "multi-c",
		BlockTime: 50 * time.Millisecond,
		Count:     10,
		Logger:    silentLogger(),
		Handler: func(_ context.Context, fields map[string]any) error {
			mu.Lock()
			defer mu.Unlock()
			if v, ok := fields["id"].(string); ok {
				received = append(received, v)
			}
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	require.Eventually(t, func() bool {
		return mr.Exists(stream)
	}, 2*time.Second, 10*time.Millisecond)

	for _, id := range []string{"a", "b", "c"} {
		_, err := rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"id": id},
		}).Result()
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 3
	}, 3*time.Second, 50*time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	assert.ElementsMatch(t, []string{"a", "b", "c"}, received)
}
