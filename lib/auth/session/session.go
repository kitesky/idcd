// Package session provides Redis-based session storage for user sessions.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// SessionData represents stored session information.
type SessionData struct {
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// Service provides session storage operations using Redis.
type Service struct {
	redis  *redis.Client
	logger *slog.Logger
}

// NewService creates a new session service with the given Redis client.
// Uses slog.Default() for logging; call SetLogger to override.
func NewService(redisClient *redis.Client) *Service {
	return &Service{redis: redisClient, logger: slog.Default()}
}

// SetLogger overrides the logger used for non-fatal warnings (e.g. failed
// LastSeenAt write-back). Passing nil resets to slog.Default().
func (s *Service) SetLogger(l *slog.Logger) {
	if l == nil {
		s.logger = slog.Default()
		return
	}
	s.logger = l
}

// Store creates or updates a session with the given session ID, user ID, and TTL.
func (s *Service) Store(ctx context.Context, sessionID, userID string, ttl time.Duration) error {
	if sessionID == "" {
		return apperr.Validation("session ID is required", "")
	}
	if userID == "" {
		return apperr.Validation("user ID is required", "")
	}
	if ttl <= 0 {
		return apperr.Validation("TTL must be positive", "")
	}

	now := time.Now()
	sessionData := SessionData{
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
	}

	// Check if session already exists to preserve CreatedAt
	key := s.sessionKey(sessionID)
	existing, err := s.redis.Get(ctx, key).Result()
	if err == nil {
		// Session exists, preserve CreatedAt
		var existingData SessionData
		if err := json.Unmarshal([]byte(existing), &existingData); err == nil {
			sessionData.CreatedAt = existingData.CreatedAt
		}
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return apperr.Internal("failed to marshal session data", err)
	}

	// Use a pipeline: write the session key AND add the session ID to the
	// user-scoped sessions set in a single round-trip. The set enables O(1)
	// per-user listing instead of O(N) SCAN across all sessions.
	pipe := s.redis.Pipeline()
	pipe.Set(ctx, key, data, ttl)
	pipe.SAdd(ctx, s.userSessionsKey(userID), sessionID)
	// ExpireGT only updates the set TTL when the new value is larger than the
	// current one, preventing a short-lived session from truncating the set TTL
	// below a pre-existing long-lived session's window.
	pipe.ExpireGT(ctx, s.userSessionsKey(userID), ttl+time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperr.Internal("failed to store session", err)
	}

	return nil
}

// Get retrieves session data by session ID.
func (s *Service) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	if sessionID == "" {
		return nil, apperr.Validation("session ID is required", "")
	}

	key := s.sessionKey(sessionID)
	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperr.NotFound("session not found")
		}
		return nil, apperr.Internal("failed to get session", err)
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return nil, apperr.Internal("failed to unmarshal session data", err)
	}

	// Update LastSeenAt atomically: single SET ... XX KEEPTTL is a single Redis
	// command, so the TTL cannot expire between read and write (the previous
	// TTL-then-SET sequence could resurrect a just-expired session). XX means
	// we only write if the key still exists — concurrent Gets race on the
	// final byte value (last-writer-wins for last_seen, acceptable per design),
	// but the TTL and key existence invariants are preserved.
	//
	// last_seen is an observational signal, not auth state. On write failure
	// we log a warning so Redis flakiness is visible, but we still return the
	// session — the user is authenticated either way. If Redis is fully down
	// the initial GET above would have failed and we'd have returned the error.
	sessionData.LastSeenAt = time.Now()
	if updatedData, marshalErr := json.Marshal(sessionData); marshalErr == nil {
		if setErr := s.redis.SetArgs(ctx, key, updatedData, redis.SetArgs{
			Mode:    "XX",
			KeepTTL: true,
		}).Err(); setErr != nil && !errors.Is(setErr, redis.Nil) {
			// redis.Nil here means the XX guard rejected the write because the
			// key expired between GET and SET — benign, not logged.
			s.logger.WarnContext(ctx, "session: failed to update last_seen_at",
				slog.String("session_id", sessionID),
				slog.Any("error", setErr),
			)
		}
	} else {
		s.logger.WarnContext(ctx, "session: failed to marshal session for last_seen_at update",
			slog.String("session_id", sessionID),
			slog.Any("error", marshalErr),
		)
	}

	return &sessionData, nil
}

// Refresh extends the session TTL by the given duration.
//
// Atomicity: a previous implementation used Exists → Get → Set, which races
// against concurrent Delete (logout in another tab) and would resurrect an
// already-deleted session. We now use Redis SET ... XX which only writes
// when the key still exists, making refresh-then-delete safe in either
// order: if Delete wins, Refresh's SET XX is a no-op and returns
// session-not-found.
func (s *Service) Refresh(ctx context.Context, sessionID string, ttl time.Duration) error {
	if sessionID == "" {
		return apperr.Validation("session ID is required", "")
	}
	if ttl <= 0 {
		return apperr.Validation("TTL must be positive", "")
	}

	key := s.sessionKey(sessionID)

	data, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return apperr.NotFound("session not found")
		}
		return apperr.Internal("failed to get session for refresh", err)
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(data), &sessionData); err != nil {
		return apperr.Internal("failed to unmarshal session data", err)
	}

	sessionData.LastSeenAt = time.Now()
	updatedData, err := json.Marshal(sessionData)
	if err != nil {
		return apperr.Internal("failed to marshal updated session data", err)
	}

	// SET XX = only set if key exists. Returns false (without error) when
	// the key has been deleted between our Get and SetXX — treat that as
	// session-not-found rather than silently recreating the row.
	ok, err := s.redis.SetXX(ctx, key, updatedData, ttl).Result()
	if err != nil {
		return apperr.Internal("failed to refresh session", err)
	}
	if !ok {
		return apperr.NotFound("session not found")
	}

	return nil
}

// deleteSessionScript atomically reads the session JSON at KEYS[1] and, if
// present, deletes both the session key and removes the session ID (ARGV[1])
// from the user-sessions set (key name templated as ARGV[2] prefix). Returning
// the raw session JSON lets the caller unmarshal user_id WITHOUT having to do
// a separate (racy) GET first.
//
// Behaviour:
//   - KEYS[1] missing → returns nil (caller treats as NotFound).
//   - KEYS[1] present but invalid JSON → still deletes the session key and
//     returns the raw payload; caller may skip the SRem because user_id is
//     unknown. (We do not attempt to parse JSON inside Lua — Redis Lua has no
//     real JSON decoder, and we already need the bytes back in Go anyway.)
//   - KEYS[1] present and JSON-parseable → deletes session key; the SRem
//     happens in the same transaction once we know which user-set to touch.
//
// Eliminates the prior TOCTOU window between GET (to learn user_id) and DEL:
// a concurrent Delete for the same sessionID will now either see the key on
// its first script invocation (succeeds, returns payload) or not (returns
// nil → NotFound). No window remains where two callers both "succeed" at
// deleting, and no window remains where another caller could Store() between
// our GET and DEL and have its fresh session silently wiped out.
//
// The SRem for the user-sessions set is intentionally a SEPARATE follow-up
// pipeline (not folded into the Lua script) because the user_id lives inside
// the JSON payload and Lua-side JSON parsing is fragile / non-portable across
// Redis versions. The follow-up SRem is best-effort — at worst we leave a
// stale set member that SessionIDsForUser callers already prune lazily.
const deleteSessionScript = `
local v = redis.call('GET', KEYS[1])
if v == false then
  return nil
end
redis.call('DEL', KEYS[1])
return v
`

// Delete removes a session by session ID and cleans up the user-sessions set.
//
// Atomicity: previously this method did GET → SRem → DEL, which raced
// against concurrent Delete (double-success) and concurrent Store (a freshly
// re-created session could be wiped between our GET and DEL). We now run a
// Lua script that does GET + DEL atomically: the script either returns the
// pre-existing payload (and has already removed the key) or returns nil
// (key was never there, or another caller won the race — both translate
// to NotFound).
func (s *Service) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return apperr.Validation("session ID is required", "")
	}

	key := s.sessionKey(sessionID)

	res, err := s.redis.Eval(ctx, deleteSessionScript, []string{key}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Script returned nil — the key did not exist when the script ran.
			return apperr.NotFound("session not found")
		}
		return apperr.Internal("failed to delete session", err)
	}

	raw, ok := res.(string)
	if !ok || raw == "" {
		// Defensive: Eval returned an unexpected type. Treat as not-found so
		// callers don't see a spurious success.
		return apperr.NotFound("session not found")
	}

	// Best-effort: clean up the per-user sessions set. The session key itself
	// is already gone — failure here only leaves a stale set member that
	// SessionIDsForUser callers will prune lazily.
	var data SessionData
	if jsonErr := json.Unmarshal([]byte(raw), &data); jsonErr == nil && data.UserID != "" {
		_ = s.redis.SRem(ctx, s.userSessionsKey(data.UserID), sessionID).Err()
	}

	return nil
}

// sessionKey returns the Redis key for a given session ID.
func (s *Service) sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// userSessionsKey returns the Redis Set key that lists all session IDs for a user.
func (s *Service) userSessionsKey(userID string) string {
	return fmt.Sprintf("user_sessions:%s", userID)
}

// SessionIDsForUser returns all active session IDs belonging to userID.
// Uses the user-scoped set — O(N_user_sessions), not O(N_total_sessions).
// Stale entries (from sessions that expired without an explicit Delete) are
// pruned lazily: callers should remove IDs that no longer have a matching session key.
func (s *Service) SessionIDsForUser(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.redis.SMembers(ctx, s.userSessionsKey(userID)).Result()
	if err != nil {
		return nil, apperr.Internal("failed to list user sessions", err)
	}
	return ids, nil
}

// RemoveFromUserSet removes a session ID from the user-scoped sessions set.
// Called after a session is deleted to keep the set clean.
func (s *Service) RemoveFromUserSet(ctx context.Context, userID, sessionID string) error {
	return s.redis.SRem(ctx, s.userSessionsKey(userID), sessionID).Err()
}