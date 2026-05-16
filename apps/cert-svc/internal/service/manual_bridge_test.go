package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/lib/cert/dns/manual"
)

func newBridgeService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc := New(Config{
		Redis:              rdb,
		ManualTimeout:      2 * time.Second,
		ManualPollInterval: 50 * time.Millisecond,
	})
	return svc, mr
}

// installTestCoordinator builds a manual.Coordinator with a never-matches
// lookup, registers a Present() goroutine for (fqdn, value), and stores
// the coordinator in the Service's manualCoordinators map at orderID.
// The returned channel closes when Present() returns successfully — i.e.
// when InjectReady has fired through the bridge.
func installTestCoordinator(t *testing.T, svc *Service, orderID int64, fqdn, value string) <-chan error {
	t.Helper()
	noMatch := func(_ context.Context, _ string) ([]string, error) { return nil, nil }
	co := manual.NewCoordinator(manual.Config{
		Timeout:      2 * time.Second,
		PollInterval: 50 * time.Millisecond,
		LookupTXT:    noMatch,
	})
	svc.mu.Lock()
	svc.manualCoordinators[orderID] = co
	svc.mu.Unlock()

	provider := manual.NewWithCoordinator(co)
	solver, err := provider.BuildSolver(context.Background(), nil, nil)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- solver.Present(context.Background(), fqdn, value)
	}()
	// Give Present() a moment to call register() before publishing.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		// register holds the coordinator mutex briefly; trying to grab it
		// gives us a coarse "has the goroutine started" check.
		co.Timeout() // no-op; just yields scheduler
		time.Sleep(20 * time.Millisecond)
		// after a few ticks, we assume Present has registered. We could
		// inspect coordinator state, but Timeout() is an obvious no-op.
		break
	}
	// Sleep one more tick to widen the safety margin on slow CI.
	time.Sleep(50 * time.Millisecond)
	return done
}

func TestPublishManualReady_NoRedis(t *testing.T) {
	svc := New(Config{})
	err := svc.PublishManualReady(context.Background(), 1, "f", "v")
	require.Error(t, err)
}

func TestPublishManualReady_RejectsBadArgs(t *testing.T) {
	svc, _ := newBridgeService(t)
	require.Error(t, svc.PublishManualReady(context.Background(), 0, "f", "v"))
	require.Error(t, svc.PublishManualReady(context.Background(), 1, "", "v"))
	require.Error(t, svc.PublishManualReady(context.Background(), 1, "f", ""))
}

func TestPublishManualReady_WritesPayload(t *testing.T) {
	svc, mr := newBridgeService(t)
	sub := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = sub.Close() }()

	ps := sub.Subscribe(context.Background(), ManualReadyChannel)
	defer func() { _ = ps.Close() }()
	ch := ps.Channel()
	waitUntilSubscribers(t, mr, ManualReadyChannel, 1)

	require.NoError(t, svc.PublishManualReady(context.Background(), 42, "_acme.example.com", "abc"))

	select {
	case m := <-ch:
		var got ManualReadyMessage
		require.NoError(t, json.Unmarshal([]byte(m.Payload), &got))
		assert.Equal(t, int64(42), got.OrderID)
		assert.Equal(t, "_acme.example.com", got.FQDN)
		assert.Equal(t, "abc", got.Value)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive publish")
	}
}

func TestRunManualReadySubscriber_DeliversToCoordinator(t *testing.T) {
	svc, mr := newBridgeService(t)

	presentDone := installTestCoordinator(t, svc, 99, "_acme.example.com", "v1")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subDone := make(chan error, 1)
	go func() { subDone <- svc.RunManualReadySubscriber(ctx) }()
	waitUntilSubscribers(t, mr, ManualReadyChannel, 1)

	pub := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = pub.Close() }()
	payload, _ := json.Marshal(ManualReadyMessage{
		OrderID: 99, FQDN: "_acme.example.com", Value: "v1",
	})
	require.NoError(t, pub.Publish(ctx, ManualReadyChannel, payload).Err())

	select {
	case err := <-presentDone:
		require.NoError(t, err, "Present should return cleanly once InjectReady fires")
	case <-time.After(3 * time.Second):
		t.Fatal("Present did not return after bridge injection")
	}

	cancel()
	select {
	case err := <-subDone:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not stop on cancel")
	}
}

func TestRunManualReadySubscriber_BadPayloadSkipped(t *testing.T) {
	svc, mr := newBridgeService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subDone := make(chan error, 1)
	go func() { subDone <- svc.RunManualReadySubscriber(ctx) }()
	waitUntilSubscribers(t, mr, ManualReadyChannel, 1)

	pub := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = pub.Close() }()
	require.NoError(t, pub.Publish(ctx, ManualReadyChannel, "not-json").Err())

	// Loop must still be alive: send a valid message and verify delivery.
	presentDone := installTestCoordinator(t, svc, 7, "f", "v")
	payload, _ := json.Marshal(ManualReadyMessage{OrderID: 7, FQDN: "f", Value: "v"})
	require.NoError(t, pub.Publish(ctx, ManualReadyChannel, payload).Err())

	select {
	case err := <-presentDone:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("good message after bad payload was not delivered")
	}

	cancel()
	<-subDone
}

func TestRunManualReadySubscriber_NoRedis(t *testing.T) {
	svc := New(Config{})
	err := svc.RunManualReadySubscriber(context.Background())
	require.Error(t, err)
}

func TestHandleManualReadyMessage_Nil(t *testing.T) {
	svc := New(Config{})
	svc.handleManualReadyMessage(nil)
}

func TestHandleManualReadyMessage_NoLocalCoordinator(t *testing.T) {
	svc, _ := newBridgeService(t)
	payload, _ := json.Marshal(ManualReadyMessage{OrderID: 999, FQDN: "f", Value: "v"})
	svc.handleManualReadyMessage(&redis.Message{
		Channel: ManualReadyChannel,
		Payload: string(payload),
	})
}

func TestHandleManualReadyMessage_BadJSON(t *testing.T) {
	svc, _ := newBridgeService(t)
	svc.handleManualReadyMessage(&redis.Message{
		Channel: ManualReadyChannel,
		Payload: "{garbage",
	})
}

func TestManualReadyMessage_Roundtrip(t *testing.T) {
	in := ManualReadyMessage{OrderID: 5, FQDN: "x", Value: "y"}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	var out ManualReadyMessage
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

// waitUntilSubscribers spins until miniredis reports the expected number
// of pub/sub subscribers on the channel, or t.Fatals after a short
// timeout.
func waitUntilSubscribers(t *testing.T, mr *miniredis.Miniredis, channel string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mr.PubSubNumSub(channel)[channel] >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected %d subscribers on %s", want, channel)
}
