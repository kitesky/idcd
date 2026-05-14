package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/session"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newSessionTestEnv(t *testing.T) (*SessionHandler, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	svc := session.NewService(rdb)
	h := NewSessionHandler(svc, rdb)
	return h, rdb, mr
}

// withSessionCtx injects userID and sessionID into the request context
// (simulates what authn middleware does).
func withSessionCtx(r *http.Request, userID, sessionID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	ctx = context.WithValue(ctx, middleware.SessionIDContextKey(), sessionID)
	return r.WithContext(ctx)
}

// storeTestSession writes a session directly to miniredis.
func storeTestSession(t *testing.T, rdb *redis.Client, sid, userID string, ttl time.Duration) {
	t.Helper()
	data := session.SessionData{
		UserID:     userID,
		CreatedAt:  time.Now().Add(-time.Hour),
		LastSeenAt: time.Now(),
	}
	b, err := json.Marshal(data)
	require.NoError(t, err)
	require.NoError(t, rdb.Set(context.Background(), "session:"+sid, string(b), ttl).Err())
}

// ── List sessions ─────────────────────────────────────────────────────────────

func TestSessionHandler_ListSessions_Success(t *testing.T) {
	h, rdb, _ := newSessionTestEnv(t)

	const userID = "usr_alice"
	const currentSID = "sess_current"
	const otherSID = "sess_other"

	storeTestSession(t, rdb, currentSID, userID, time.Hour)
	storeTestSession(t, rdb, otherSID, userID, time.Hour)
	// A session belonging to a different user — must NOT appear.
	storeTestSession(t, rdb, "sess_bob", "usr_bob", time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/sessions", nil)
	req = withSessionCtx(req, userID, currentSID)
	req = withRequestID(req, "req-list-1")
	rr := httptest.NewRecorder()

	h.ListSessions(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data listSessionsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	assert.Len(t, resp.Data.Sessions, 2)

	var foundCurrent bool
	for _, s := range resp.Data.Sessions {
		if s.ID == currentSID {
			foundCurrent = true
			assert.True(t, s.IsCurrent)
		}
		if s.ID == otherSID {
			assert.False(t, s.IsCurrent)
		}
	}
	assert.True(t, foundCurrent, "current session should appear in the list")
}

// ── Revoke — success ──────────────────────────────────────────────────────────

func TestSessionHandler_RevokeSession_Success(t *testing.T) {
	h, rdb, _ := newSessionTestEnv(t)

	const userID = "usr_alice"
	const currentSID = "sess_current"
	const targetSID = "sess_old"

	storeTestSession(t, rdb, currentSID, userID, time.Hour)
	storeTestSession(t, rdb, targetSID, userID, time.Hour)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/sessions/"+targetSID, nil)
	req = withSessionCtx(req, userID, currentSID)
	req = withRequestID(req, "req-revoke-1")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", targetSID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.RevokeSession(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)

	// Verify the session is gone from Redis.
	exists, err := rdb.Exists(context.Background(), "session:"+targetSID).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

// ── Revoke current session → 400 ─────────────────────────────────────────────

func TestSessionHandler_RevokeSession_CurrentSession(t *testing.T) {
	h, rdb, _ := newSessionTestEnv(t)

	const userID = "usr_alice"
	const currentSID = "sess_current"

	storeTestSession(t, rdb, currentSID, userID, time.Hour)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/sessions/"+currentSID, nil)
	req = withSessionCtx(req, userID, currentSID)
	req = withRequestID(req, "req-revoke-current")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", currentSID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.RevokeSession(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &errResp))
	assert.Equal(t, "cannot revoke current session", errResp.Error.Message)
}

// ── Revoke another user's session → 403 ──────────────────────────────────────

func TestSessionHandler_RevokeSession_OtherUser(t *testing.T) {
	h, rdb, _ := newSessionTestEnv(t)

	const aliceID = "usr_alice"
	const currentSID = "sess_alice_current"
	const bobSID = "sess_bob"

	storeTestSession(t, rdb, currentSID, aliceID, time.Hour)
	storeTestSession(t, rdb, bobSID, "usr_bob", time.Hour)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/sessions/"+bobSID, nil)
	req = withSessionCtx(req, aliceID, currentSID)
	req = withRequestID(req, "req-revoke-other")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("session_id", bobSID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.RevokeSession(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}
