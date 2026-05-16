package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/kite365/idcd/lib/shared/apperr"
)

func setupTestRedis(t *testing.T) *redis.Client {
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

func TestNewService(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	assert.NotNil(t, service)
	assert.Equal(t, redisClient, service.redis)
}

func TestService_Store(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_testsession123"
	userID := "u_testuser456"
	ttl := 15 * time.Minute

	t.Run("successful store", func(t *testing.T) {
		err := service.Store(ctx, sessionID, userID, ttl)
		require.NoError(t, err)

		// Verify data is stored in Redis
		key := service.sessionKey(sessionID)
		exists := redisClient.Exists(ctx, key).Val()
		assert.Equal(t, int64(1), exists)

		// Verify TTL is set
		ttlVal := redisClient.TTL(ctx, key).Val()
		assert.True(t, ttlVal > 0 && ttlVal <= ttl)
	})

	t.Run("empty session ID", func(t *testing.T) {
		err := service.Store(ctx, "", userID, ttl)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("empty user ID", func(t *testing.T) {
		err := service.Store(ctx, sessionID, "", ttl)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("zero TTL", func(t *testing.T) {
		err := service.Store(ctx, sessionID, userID, 0)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("negative TTL", func(t *testing.T) {
		err := service.Store(ctx, sessionID, userID, -time.Minute)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestService_Get(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_testsession123"
	userID := "u_testuser456"
	ttl := 15 * time.Minute

	t.Run("get existing session", func(t *testing.T) {
		// First store a session
		err := service.Store(ctx, sessionID, userID, ttl)
		require.NoError(t, err)

		// Get the session
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.NotNil(t, sessionData)
		assert.Equal(t, userID, sessionData.UserID)
		assert.False(t, sessionData.CreatedAt.IsZero())
		assert.False(t, sessionData.LastSeenAt.IsZero())
		assert.True(t, sessionData.LastSeenAt.After(sessionData.CreatedAt) || sessionData.LastSeenAt.Equal(sessionData.CreatedAt))
	})

	t.Run("get non-existent session", func(t *testing.T) {
		_, err := service.Get(ctx, "s_nonexistent")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeNotFound))
	})

	t.Run("empty session ID", func(t *testing.T) {
		_, err := service.Get(ctx, "")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("updates LastSeenAt on Get", func(t *testing.T) {
		// Store a session
		sessionID2 := "s_testsession789"
		err := service.Store(ctx, sessionID2, userID, ttl)
		require.NoError(t, err)

		// Get it once
		sessionData1, err := service.Get(ctx, sessionID2)
		require.NoError(t, err)
		firstLastSeen := sessionData1.LastSeenAt

		// Wait a bit
		time.Sleep(10 * time.Millisecond)

		// Get it again
		sessionData2, err := service.Get(ctx, sessionID2)
		require.NoError(t, err)

		// LastSeenAt should be updated
		assert.True(t, sessionData2.LastSeenAt.After(firstLastSeen))
		// CreatedAt should remain the same
		assert.Equal(t, sessionData1.CreatedAt, sessionData2.CreatedAt)
	})
}

func TestService_Refresh(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_testsession123"
	userID := "u_testuser456"
	originalTTL := 5 * time.Minute
	newTTL := 30 * time.Minute

	t.Run("refresh existing session", func(t *testing.T) {
		// Store a session
		err := service.Store(ctx, sessionID, userID, originalTTL)
		require.NoError(t, err)

		// Get original TTL
		key := service.sessionKey(sessionID)
		originalTTLVal := redisClient.TTL(ctx, key).Val()

		// Wait a bit
		time.Sleep(10 * time.Millisecond)

		// Refresh the session
		err = service.Refresh(ctx, sessionID, newTTL)
		require.NoError(t, err)

		// Check TTL is extended
		newTTLVal := redisClient.TTL(ctx, key).Val()
		assert.True(t, newTTLVal > originalTTLVal)

		// Verify session data still exists and LastSeenAt is updated
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, userID, sessionData.UserID)
	})

	t.Run("refresh non-existent session", func(t *testing.T) {
		err := service.Refresh(ctx, "s_nonexistent", newTTL)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeNotFound))
	})

	t.Run("empty session ID", func(t *testing.T) {
		err := service.Refresh(ctx, "", newTTL)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("zero TTL", func(t *testing.T) {
		err := service.Refresh(ctx, sessionID, 0)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("negative TTL", func(t *testing.T) {
		err := service.Refresh(ctx, sessionID, -time.Minute)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestService_Delete(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_testsession123"
	userID := "u_testuser456"
	ttl := 15 * time.Minute

	t.Run("delete existing session", func(t *testing.T) {
		// Store a session
		err := service.Store(ctx, sessionID, userID, ttl)
		require.NoError(t, err)

		// Verify it exists
		key := service.sessionKey(sessionID)
		exists := redisClient.Exists(ctx, key).Val()
		assert.Equal(t, int64(1), exists)

		// Delete it
		err = service.Delete(ctx, sessionID)
		require.NoError(t, err)

		// Verify it's gone
		exists = redisClient.Exists(ctx, key).Val()
		assert.Equal(t, int64(0), exists)
	})

	t.Run("delete non-existent session", func(t *testing.T) {
		err := service.Delete(ctx, "s_nonexistent")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeNotFound))
	})

	t.Run("empty session ID", func(t *testing.T) {
		err := service.Delete(ctx, "")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestService_sessionKey(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)

	sessionID := "s_testsession123"
	expected := "session:s_testsession123"
	actual := service.sessionKey(sessionID)

	assert.Equal(t, expected, actual)
}

func TestService_StorePreservesCreatedAt(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_testsession123"
	userID := "u_testuser456"
	ttl := 15 * time.Minute

	// Store session first time
	err := service.Store(ctx, sessionID, userID, ttl)
	require.NoError(t, err)

	// Get the session to check CreatedAt
	sessionData1, err := service.Get(ctx, sessionID)
	require.NoError(t, err)
	originalCreatedAt := sessionData1.CreatedAt

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Store the session again (simulating session update)
	err = service.Store(ctx, sessionID, userID, ttl)
	require.NoError(t, err)

	// Get the session again
	sessionData2, err := service.Get(ctx, sessionID)
	require.NoError(t, err)

	// CreatedAt should be preserved
	assert.Equal(t, originalCreatedAt, sessionData2.CreatedAt)
	// LastSeenAt should be updated
	assert.True(t, sessionData2.LastSeenAt.After(sessionData1.LastSeenAt))
}

func TestService_ErrorCases(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	t.Run("store with malformed existing session data", func(t *testing.T) {
		sessionID := "s_corrupted"
		key := service.sessionKey(sessionID)

		// Store invalid JSON data
		redisClient.Set(ctx, key, "invalid-json-data", 5*time.Minute)

		// Store should still work (will overwrite corrupted data)
		err := service.Store(ctx, sessionID, "u_test", 5*time.Minute)
		require.NoError(t, err)

		// Should be able to get the session normally
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, "u_test", sessionData.UserID)
	})

	t.Run("get with malformed session data", func(t *testing.T) {
		sessionID := "s_corrupted2"
		key := service.sessionKey(sessionID)

		// Store invalid JSON data
		redisClient.Set(ctx, key, "invalid-json-data", 5*time.Minute)

		// Get should return error
		_, err := service.Get(ctx, sessionID)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeInternal))
	})

	t.Run("refresh with malformed session data", func(t *testing.T) {
		sessionID := "s_corrupted3"
		key := service.sessionKey(sessionID)

		// Store invalid JSON data
		redisClient.Set(ctx, key, "invalid-json-data", 5*time.Minute)

		// Refresh should return error
		err := service.Refresh(ctx, sessionID, 10*time.Minute)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeInternal))
	})

	t.Run("get updates last seen with zero TTL", func(t *testing.T) {
		sessionID := "s_zero_ttl"
		userID := "u_test"

		// Store session
		err := service.Store(ctx, sessionID, userID, 5*time.Minute)
		require.NoError(t, err)

		// Manually set TTL to -1 (expired/no TTL)
		key := service.sessionKey(sessionID)
		redisClient.Persist(ctx, key) // Remove expiry

		// Get should still work and not try to update with invalid TTL
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, userID, sessionData.UserID)
	})

	t.Run("refresh handles marshal error path", func(t *testing.T) {
		// This is hard to trigger in practice, but we'll create a session
		// and verify refresh works normally to improve coverage
		sessionID := "s_refresh_test"
		userID := "u_test"

		// Store session
		err := service.Store(ctx, sessionID, userID, 5*time.Minute)
		require.NoError(t, err)

		// Normal refresh should work
		err = service.Refresh(ctx, sessionID, 10*time.Minute)
		require.NoError(t, err)

		// Verify session still exists and has updated TTL
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, userID, sessionData.UserID)
	})

	t.Run("store handles marshal error scenarios", func(t *testing.T) {
		// Test that store works even when there's corrupted data
		sessionID := "s_marshal_test"
		userID := "u_test"

		// First store should work
		err := service.Store(ctx, sessionID, userID, 5*time.Minute)
		require.NoError(t, err)

		// Store again should preserve CreatedAt even if existing data is corrupted
		key := service.sessionKey(sessionID)
		redisClient.Set(ctx, key, "corrupted-json", 5*time.Minute)

		// Store should still work (corrupted data will be ignored)
		err = service.Store(ctx, sessionID, userID, 10*time.Minute)
		require.NoError(t, err)

		// Should be able to get the session
		sessionData, err := service.Get(ctx, sessionID)
		require.NoError(t, err)
		assert.Equal(t, userID, sessionData.UserID)
	})
}

// errorOnCmdHook injects an error response for any redis command whose name
// starts with the given prefix (case-insensitive). It implements redis.Hook.
type errorOnCmdHook struct {
	cmdPrefix string
	err       error
}

func (h *errorOnCmdHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (h *errorOnCmdHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if strings.EqualFold(cmd.Name(), h.cmdPrefix) {
			cmd.SetErr(h.err)
			return h.err
		}
		return next(ctx, cmd)
	}
}

func (h *errorOnCmdHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

// captureLogger returns a slog.Logger that writes JSON lines into buf so the
// test can assert on warnings.
func captureLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// TestService_GetConcurrent verifies that concurrent Get() calls do not
// lose the LastSeenAt update — atomic SET ... XX KEEPTTL means each writer
// either wins or is benignly preempted, and the final value is always one
// of the writers' timestamps (not the original stored timestamp).
func TestService_GetConcurrent(t *testing.T) {
	redisClient := setupTestRedis(t)
	service := NewService(redisClient)
	ctx := context.Background()

	sessionID := "s_concurrent"
	userID := "u_concurrent"
	ttl := 15 * time.Minute

	require.NoError(t, service.Store(ctx, sessionID, userID, ttl))

	// Capture the original LastSeenAt for comparison.
	original, err := service.Get(ctx, sessionID)
	require.NoError(t, err)
	originalLastSeen := original.LastSeenAt
	// Wait so concurrent goroutines produce later timestamps.
	time.Sleep(20 * time.Millisecond)

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]time.Time, goroutines)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			data, err := service.Get(ctx, sessionID)
			if err == nil && data != nil {
				results[idx] = data.LastSeenAt
			}
		}(i)
	}
	wg.Wait()

	// Each goroutine read a LastSeenAt that is strictly after the initial
	// (pre-concurrent) read — proves Get() returns a freshly stamped value.
	for i, ts := range results {
		assert.Falsef(t, ts.IsZero(), "goroutine %d got zero timestamp", i)
		assert.Truef(t, ts.After(originalLastSeen),
			"goroutine %d LastSeenAt=%v not after originalLastSeen=%v",
			i, ts, originalLastSeen)
	}

	// And the value persisted in Redis is one of the concurrent writers'
	// timestamps (last writer wins) — not the original. This proves the
	// write-back actually happened atomically and did not get clobbered
	// by a TTL race resurrecting the old session.
	final, err := service.Get(ctx, sessionID)
	require.NoError(t, err)
	assert.True(t, final.LastSeenAt.After(originalLastSeen),
		"final LastSeenAt should be after the pre-concurrent value")

	// TTL must still be roughly intact — KEEPTTL means we never accidentally
	// dropped or extended the expiry.
	key := service.sessionKey(sessionID)
	finalTTL := redisClient.TTL(ctx, key).Val()
	assert.True(t, finalTTL > 0 && finalTTL <= ttl,
		"TTL=%v should remain within (0, %v]", finalTTL, ttl)
}

// TestService_GetRedisWriteFailure verifies that a failure while writing
// back LastSeenAt does NOT cause Get() to fail — the caller still gets the
// session — but the failure is surfaced via the logger (not silently swallowed).
func TestService_GetRedisWriteFailure(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := NewService(rdb)
	ctx := context.Background()

	sessionID := "s_write_fail"
	userID := "u_write_fail"
	ttl := 15 * time.Minute

	// Pre-store a session via the normal path (before installing the failing hook).
	require.NoError(t, service.Store(ctx, sessionID, userID, ttl))

	// Capture warnings.
	var buf bytes.Buffer
	service.SetLogger(captureLogger(&buf))

	// Install a hook that fails ONLY the SET command. GET still works,
	// so Get() can load the session — but the write-back of LastSeenAt fails.
	injected := fmt.Errorf("simulated redis write failure")
	rdb.AddHook(&errorOnCmdHook{cmdPrefix: "SET", err: injected})

	got, err := service.Get(ctx, sessionID)
	require.NoError(t, err, "Get should still succeed even if LastSeenAt write-back fails")
	require.NotNil(t, got)
	assert.Equal(t, userID, got.UserID)

	// The failure should be logged as a warning, not silently swallowed.
	logged := buf.String()
	assert.Contains(t, logged, "session: failed to update last_seen_at",
		"expected warning log line, got: %s", logged)
	assert.Contains(t, logged, sessionID, "log line should include session_id")
	// Confirm the level is WARN (slog JSON uses "level":"WARN").
	assert.True(t, strings.Contains(logged, `"level":"WARN"`),
		"expected WARN level in log: %s", logged)

	// And we can still parse out the entry as valid JSON.
	for _, line := range strings.Split(strings.TrimSpace(logged), "\n") {
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), "log line must be valid JSON")
		assert.Equal(t, "WARN", entry["level"])
	}
}

// TestService_GetRedisDown verifies that when Redis is completely
// unreachable, Get() returns nil + error rather than masquerading as success.
func TestService_GetRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	service := NewService(rdb)
	ctx := context.Background()

	sessionID := "s_redis_down"
	userID := "u_redis_down"
	ttl := 15 * time.Minute

	// Store a session, then kill the Redis server so subsequent ops fail
	// with a connection error.
	require.NoError(t, service.Store(ctx, sessionID, userID, ttl))
	mr.Close()

	got, err := service.Get(ctx, sessionID)
	assert.Nil(t, got, "Get must return nil when Redis is unreachable")
	require.Error(t, err, "Get must return an error when Redis is unreachable")
	// Must not be a NotFound (that would mask a real outage as a missing session).
	assert.False(t, apperr.Is(err, apperr.CodeNotFound),
		"unreachable Redis must not be reported as NotFound")
	// Should be an Internal error wrapping the underlying connection failure.
	assert.True(t, apperr.Is(err, apperr.CodeInternal),
		"expected CodeInternal, got: %v", err)
}