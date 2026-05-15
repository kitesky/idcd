package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/session"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// SessionService is the interface required by SessionHandler.
type SessionService interface {
	Get(ctx context.Context, sessionID string) (*session.SessionData, error)
	Delete(ctx context.Context, sessionID string) error
	// SessionIDsForUser lists all session IDs for a user via the user-scoped set.
	SessionIDsForUser(ctx context.Context, userID string) ([]string, error)
	// RemoveFromUserSet cleans up a stale entry from the user-sessions set.
	RemoveFromUserSet(ctx context.Context, userID, sessionID string) error
}

// sessionEntry is a single session as returned by the list endpoint.
type sessionEntry struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	IsCurrent bool      `json:"is_current"`
}

// listSessionsResponse is the response body for GET /v1/account/sessions.
type listSessionsResponse struct {
	Sessions []sessionEntry `json:"sessions"`
}

// SessionHandler implements the session management endpoints.
type SessionHandler struct {
	svc   SessionService
	redis *redis.Client
}

// NewSessionHandler creates a SessionHandler.
func NewSessionHandler(svc SessionService, rdb *redis.Client) *SessionHandler {
	return &SessionHandler{svc: svc, redis: rdb}
}

// ListSessions handles GET /v1/account/sessions.
// Uses the user-scoped sessions set (user_sessions:{userID}) for O(N_user_sessions)
// lookup instead of O(N_total_sessions) SCAN.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}
	currentSessionID := middleware.SessionIDFromContext(r.Context())

	// Fetch all session IDs for this user in one SMEMBERS call.
	sessionIDs, err := h.svc.SessionIDsForUser(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list sessions", err))
		return
	}

	var entries []sessionEntry
	var staleIDs []string

	if len(sessionIDs) > 0 {
		// Batch-fetch all session values with a pipelined MGet — O(1) round-trip.
		keys := make([]string, len(sessionIDs))
		for i, sid := range sessionIDs {
			keys[i] = sessionKeyPrefix + sid
		}
		vals, err := h.redis.MGet(r.Context(), keys...).Result()
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to load sessions", err))
			return
		}

		for i, val := range vals {
			if val == nil {
				// Session expired without an explicit Delete — prune stale set entry.
				staleIDs = append(staleIDs, sessionIDs[i])
				continue
			}
			raw, ok := val.(string)
			if !ok {
				continue
			}
			var data session.SessionData
			if err := json.Unmarshal([]byte(raw), &data); err != nil {
				continue
			}
			entries = append(entries, sessionEntry{
				ID:        sessionIDs[i],
				CreatedAt: data.CreatedAt,
				IsCurrent: sessionIDs[i] == currentSessionID,
			})
		}

		// Lazy cleanup of stale set entries (best-effort, non-blocking).
		for _, sid := range staleIDs {
			_ = h.svc.RemoveFromUserSet(r.Context(), userID, sid)
		}
	}

	if entries == nil {
		entries = []sessionEntry{}
	}

	response.JSON(w, r, http.StatusOK, listSessionsResponse{Sessions: entries})
}

// RevokeSession handles DELETE /v1/account/sessions/{session_id}.
func (h *SessionHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}
	currentSessionID := middleware.SessionIDFromContext(r.Context())

	targetID := chi.URLParam(r, "session_id")
	if targetID == "" {
		response.Error(w, r, apperr.Validation("session_id is required", ""))
		return
	}

	// Cannot revoke the current session.
	if targetID == currentSessionID {
		response.Error(w, r, apperr.Validation("cannot revoke current session", ""))
		return
	}

	// Ownership check: load the target session and verify it belongs to this user.
	data, err := h.svc.Get(r.Context(), targetID)
	if err != nil {
		if appErr := apperr.AsError(err); appErr != nil && appErr.Code == apperr.CodeNotFound {
			response.Error(w, r, apperr.NotFound("session not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to load session", err))
		return
	}

	if data.UserID != userID {
		response.Error(w, r, apperr.Forbidden("not your session"))
		return
	}

	if err := h.svc.Delete(r.Context(), targetID); err != nil {
		if appErr := apperr.AsError(err); appErr != nil && appErr.Code == apperr.CodeNotFound {
			response.Error(w, r, apperr.NotFound("session not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to revoke session", err))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}
