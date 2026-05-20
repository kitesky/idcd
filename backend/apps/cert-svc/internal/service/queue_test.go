package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

func newPgxMock() (pgxmock.PgxPoolIface, error) { return pgxmock.NewPool() }

func repoNewWithPool(p pgxmock.PgxPoolIface) *repo.Repos { return repo.NewWithPool(p) }

func newQueueService(t *testing.T) (*Service, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := New(Config{
		Redis:        rdb,
		Stream:       "cert:order_events",
		Group:        "cert-worker",
		ConsumerName: "test-consumer",
		BlockTimeout: 50 * time.Millisecond,
	})
	return svc, mr, rdb
}

func TestEnqueueOrder_WritesToStream(t *testing.T) {
	svc, mr, _ := newQueueService(t)
	ctx := context.Background()

	require.NoError(t, svc.EnqueueOrder(ctx, 42))

	entries, err := mr.Stream("cert:order_events")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	// Values is a flat [key, value, key, value, ...] slice.
	require.GreaterOrEqual(t, len(entries[0].Values), 2)
	assert.Equal(t, "order_id", entries[0].Values[0])
	assert.Equal(t, "42", entries[0].Values[1])
}

func TestEnqueueOrder_NoRedis(t *testing.T) {
	svc := New(Config{})
	err := svc.EnqueueOrder(context.Background(), 1)
	require.Error(t, err)
}

func TestRunConsumer_HandlesAndACKs(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	// Wire a pgxmock so DriveOrder has a Repos that returns ErrNotFound
	// quickly — handleMessage will log the error but still ACK.
	pool, err := newPgxMock()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	pool.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(7)).
		WillReturnError(redis.Nil) // any error works

	svc := New(Config{
		Redis:        rdb,
		Repos:        repoNewWithPool(pool),
		Stream:       "cert:order_events",
		Group:        "cert-worker",
		ConsumerName: "test-consumer",
		BlockTimeout: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, svc.ensureGroup(ctx))

	_, err = rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "cert:order_events",
		Values: map[string]any{"order_id": "7"},
	}).Result()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		_ = svc.RunConsumer(ctx)
		close(done)
	}()

	// Wait long enough for the consumer to pick the message, drive it,
	// and ACK. miniredis is synchronous so a single 200ms tick is
	// plenty in practice.
	acked := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// Give the goroutine a turn.
		time.Sleep(50 * time.Millisecond)
		groups, gerr := rdb.XInfoGroups(ctx, "cert:order_events").Result()
		if gerr == nil && len(groups) > 0 {
			g := groups[0]
			if g.LastDeliveredID != "0-0" && g.Pending == 0 {
				acked = true
				break
			}
		}
	}
	cancel()
	<-done
	assert.True(t, acked, "expected message to be delivered and then ACKed")
}

func TestRunConsumer_InvalidMessageACKed(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := New(Config{
		Redis:        rdb,
		Stream:       "cert:order_events",
		Group:        "cert-worker",
		ConsumerName: "test-consumer",
		BlockTimeout: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, svc.ensureGroup(ctx))

	// Message missing order_id field.
	_, err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: "cert:order_events",
		Values: map[string]any{"bogus": "1"},
	}).Result()
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		_ = svc.RunConsumer(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	acked := false
	for time.Now().Before(deadline) {
		info, perr := rdb.XPending(ctx, "cert:order_events", "cert-worker").Result()
		if perr == nil && info.Count == 0 {
			acked = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	assert.True(t, acked)
}

func TestRunConsumer_StopsOnContextCancel(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := New(Config{
		Redis:        rdb,
		Stream:       "cert:order_events",
		Group:        "cert-worker",
		ConsumerName: "test-consumer",
		BlockTimeout: 30 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.RunConsumer(ctx) }()

	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("RunConsumer did not return after cancel")
	}
}

func TestParseOrderID(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
		want int64
		ok   bool
	}{
		{"string", map[string]any{"order_id": "42"}, 42, true},
		{"int", map[string]any{"order_id": 7}, 7, true},
		{"int64", map[string]any{"order_id": int64(99)}, 99, true},
		{"missing", map[string]any{}, 0, false},
		{"bad-string", map[string]any{"order_id": "abc"}, 0, false},
		{"wrong-type", map[string]any{"order_id": 1.5}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOrderID(tc.in)
			if tc.ok {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestRunConsumer_NoRedis(t *testing.T) {
	svc := New(Config{})
	err := svc.RunConsumer(context.Background())
	require.Error(t, err)
}

func TestReclaimAbandoned_NoOpOnEmpty(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := New(Config{
		Redis:        rdb,
		Stream:       "cert:order_events",
		Group:        "cert-worker",
		ConsumerName: "rc",
	})
	ctx := context.Background()
	require.NoError(t, svc.ensureGroup(ctx))

	// No pending messages — XAUTOCLAIM returns empty result. The
	// function should swallow that silently.
	svc.reclaimAbandoned(ctx, "rc")
}
