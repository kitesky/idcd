package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMiniredisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestVerdictPublisher_EnqueueVerdict_AddsStreamEntry(t *testing.T) {
	mr, rdb := newMiniredisClient(t)
	pub := newVerdictPublisherWithClient(rdb)

	err := pub.EnqueueVerdict(context.Background(), "v_abc123", "u_owner1")
	require.NoError(t, err)

	// One entry should be on the canonical stream.
	require.True(t, mr.Exists(VerdictStreamName), "stream should exist after XAdd")
	entries, err := rdb.XRange(context.Background(), VerdictStreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)

	values := entries[0].Values
	assert.Equal(t, "v_abc123", values["order_id"])
	assert.Equal(t, "u_owner1", values["owner_id"])
	assert.NotEmpty(t, values["enqueued_at"], "enqueued_at must be populated")
}

func TestVerdictPublisher_EnqueueVerdict_MultipleEntriesAppend(t *testing.T) {
	_, rdb := newMiniredisClient(t)
	pub := newVerdictPublisherWithClient(rdb)

	require.NoError(t, pub.EnqueueVerdict(context.Background(), "v_1", "u_1"))
	require.NoError(t, pub.EnqueueVerdict(context.Background(), "v_2", "u_2"))
	require.NoError(t, pub.EnqueueVerdict(context.Background(), "v_3", "u_3"))

	entries, err := rdb.XRange(context.Background(), VerdictStreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "v_1", entries[0].Values["order_id"])
	assert.Equal(t, "v_2", entries[1].Values["order_id"])
	assert.Equal(t, "v_3", entries[2].Values["order_id"])
}

func TestVerdictPublisher_EnqueueVerdict_EmptyOwnerIDAllowed(t *testing.T) {
	// Owner ID may be empty for service-account flows / synthetic stub events;
	// the consumer treats it as best-effort metadata only.
	_, rdb := newMiniredisClient(t)
	pub := newVerdictPublisherWithClient(rdb)

	err := pub.EnqueueVerdict(context.Background(), "v_solo", "")
	require.NoError(t, err)

	entries, err := rdb.XRange(context.Background(), VerdictStreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "", entries[0].Values["owner_id"])
}

func TestVerdictPublisher_EnqueueVerdict_EmptyOrderIDRejected(t *testing.T) {
	_, rdb := newMiniredisClient(t)
	pub := newVerdictPublisherWithClient(rdb)

	err := pub.EnqueueVerdict(context.Background(), "", "u_owner1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "order_id")
}

// stubFailingClient simulates a Redis error from XAdd so we can assert the
// error-wrap path without needing a flaky network.
type stubFailingClient struct {
	err error
}

func (s *stubFailingClient) XAdd(ctx context.Context, _ *redis.XAddArgs) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	cmd.SetErr(s.err)
	return cmd
}

func TestVerdictPublisher_EnqueueVerdict_ClientError(t *testing.T) {
	sentinel := errors.New("redis down")
	pub := newVerdictPublisherWithClient(&stubFailingClient{err: sentinel})

	err := pub.EnqueueVerdict(context.Background(), "v_x", "u_x")
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictPublisher_EnqueueVerdict_NilClient(t *testing.T) {
	pub := &VerdictPublisher{client: nil, stream: VerdictStreamName}
	err := pub.EnqueueVerdict(context.Background(), "v_x", "u_x")
	require.Error(t, err)
}

func TestVerdictPublisher_EnqueueVerdict_NilReceiver(t *testing.T) {
	var pub *VerdictPublisher
	err := pub.EnqueueVerdict(context.Background(), "v_x", "u_x")
	require.Error(t, err)
}

func TestNewVerdictPublisher_ConcreteClient(t *testing.T) {
	// Smoke-test the public constructor. We don't XAdd anything (would require
	// a real Redis); just verify the stream is wired correctly.
	c := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer func() { _ = c.Close() }()
	pub := NewVerdictPublisher(c)
	require.NotNil(t, pub)
	assert.Equal(t, VerdictStreamName, pub.stream)
}
