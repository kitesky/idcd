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

// SessionService is the interface required by SessionHandler.
type SessionService interface {
	Get(ctx context.Context, sessionID string) (*session.SessionData, error)
	Delete(ctx context.Context, sessionID string) error
}

// redisSessionLister uses a Redis SCAN to enumerate sessions belonging to a user.
type redisSessionLister struct {
	redis *redis.Client
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
// It scans Redis for all session:{id} keys, loads each, and returns only the
// sessions that belong to the authenticated user.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}
	currentSessionID := middleware.SessionIDFromContext(r.Context())

	var cursor uint64
	var entries []sessionEntry

	for {
		keys, nextCursor, err := h.redis.Scan(r.Context(), cursor, "session:*", 100).Result()
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to scan sessions", err))
			return
		}

		for _, key := range keys {
			// key is "session:<id>" — extract the session ID suffix
			if len(key) <= len("session:") {
				continue
			}
			sid := key[len("session:"):]

			raw, err := h.redis.Get(r.Context(), key).Result()
			if err != nil {
				// Session may have expired between SCAN and GET; skip silently.
				continue
			}

			var data session.SessionData
			if err := json.Unmarshal([]byte(raw), &data); err != nil {
				continue
			}

			if data.UserID != userID {
				continue
			}

			entries = append(entries, sessionEntry{
				ID:        sid,
				CreatedAt: data.CreatedAt,
				IsCurrent: sid == currentSessionID,
			})
		}

		cursor = nextCursor
		if cursor == 0 {
			break
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
