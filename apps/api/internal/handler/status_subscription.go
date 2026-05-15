package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type StatusSubPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type StatusSubscriptionHandler struct {
	pool StatusSubPool
}

func NewStatusSubscriptionHandler(pool StatusSubPool) *StatusSubscriptionHandler {
	return &StatusSubscriptionHandler{pool: pool}
}

type statusSubRequest struct {
	ChannelType string   `json:"channel_type"`
	Endpoint    string   `json:"endpoint"`
	Events      []string `json:"events"`
}

type statusSubResponse struct {
	ID          string    `json:"id"`
	ChannelType string    `json:"channel_type"`
	Endpoint    string    `json:"endpoint"`
	Verified    bool      `json:"verified"`
	Events      []string  `json:"events"`
	CreatedAt   time.Time `json:"created_at"`
}

var validSubChannelTypes = map[string]bool{
	"email":    true,
	"webhook":  true,
	"wecom":    true,
	"dingtalk": true,
}

func (h *StatusSubscriptionHandler) resolveStatusPageID(ctx context.Context, slug string) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM status_pages WHERE slug = $1`,
		slug,
	).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", apperr.NotFound("status page not found")
	}
	if err != nil {
		return "", apperr.Internal("failed to lookup status page", err)
	}
	return id, nil
}

func (h *StatusSubscriptionHandler) resolveOwnedStatusPageID(ctx context.Context, slug, userID string) (string, error) {
	var id string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM status_pages WHERE slug = $1 AND user_id = $2`,
		slug, userID,
	).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", apperr.NotFound("status page not found")
	}
	if err != nil {
		return "", apperr.Internal("failed to lookup status page", err)
	}
	return id, nil
}

func (h *StatusSubscriptionHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	statusPageID, err := h.resolveStatusPageID(ctx, slug)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var req statusSubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid JSON request body", ""))
		return
	}

	if req.ChannelType == "" {
		response.Error(w, r, apperr.Validation("channel_type is required", "channel_type"))
		return
	}
	if req.Endpoint == "" {
		response.Error(w, r, apperr.Validation("endpoint is required", "endpoint"))
		return
	}
	if !validSubChannelTypes[req.ChannelType] {
		response.Error(w, r, apperr.Validation("channel_type must be one of: email, webhook, wecom, dingtalk", "channel_type"))
		return
	}

	// Validate endpoint format to prevent stored SSRF and email header injection.
	switch req.ChannelType {
	case "email":
		if _, err := mail.ParseAddress(req.Endpoint); err != nil {
			response.Error(w, r, apperr.Validation("endpoint must be a valid email address", "endpoint"))
			return
		}
	case "webhook", "wecom", "dingtalk":
		u, err := url.ParseRequestURI(req.Endpoint)
		if err != nil || u.Scheme != "https" {
			response.Error(w, r, apperr.Validation("endpoint must be a valid HTTPS URL", "endpoint"))
			return
		}
		host := u.Hostname()
		if ip := net.ParseIP(host); ip != nil && isPrivateIP(req.Endpoint) {
			response.Error(w, r, apperr.Validation("endpoint must not point to a private IP address", "endpoint"))
			return
		}
	}

	events := req.Events
	if len(events) == 0 {
		events = []string{"incident", "recovery", "maintenance"}
	}

	id := idgen.StatusSubscription()
	verified := req.ChannelType != "email"
	var verifyToken *string

	if req.ChannelType == "email" {
		tok, err := generateVerifyToken()
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to generate verify token", err))
			return
		}
		verifyToken = &tok
	}

	now := time.Now().UTC()
	_, err = h.pool.Exec(ctx,
		`INSERT INTO status_page_subscriptions (id, status_page_id, channel_type, endpoint, verified, verify_token, events, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, statusPageID, req.ChannelType, req.Endpoint, verified, verifyToken, events, now,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to create subscription", err))
		return
	}

	response.JSON(w, r, http.StatusCreated, statusSubResponse{
		ID:          id,
		ChannelType: req.ChannelType,
		Endpoint:    req.Endpoint,
		Verified:    verified,
		Events:      events,
		CreatedAt:   now,
	})
}

func (h *StatusSubscriptionHandler) Verify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		response.Error(w, r, apperr.Validation("token query parameter is required", "token"))
		return
	}

	var id string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM status_page_subscriptions WHERE verify_token = $1`,
		token,
	).Scan(&id)
	if err == pgx.ErrNoRows {
		response.Error(w, r, apperr.NotFound("invalid or expired token"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to lookup token", err))
		return
	}

	if _, err = h.pool.Exec(ctx,
		`UPDATE status_page_subscriptions SET verified = TRUE, verify_token = NULL WHERE id = $1`,
		id,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to verify subscription", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"verified": true})
}

func (h *StatusSubscriptionHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		response.Error(w, r, apperr.Validation("token query parameter is required", "token"))
		return
	}

	var id string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM status_page_subscriptions WHERE verify_token = $1`,
		token,
	).Scan(&id)
	if err == pgx.ErrNoRows {
		response.Error(w, r, apperr.NotFound("invalid token"))
		return
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to lookup token", err))
		return
	}

	if _, err = h.pool.Exec(ctx,
		`DELETE FROM status_page_subscriptions WHERE id = $1`,
		id,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete subscription", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

func (h *StatusSubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	slug := chi.URLParam(r, "slug")

	statusPageID, err := h.resolveOwnedStatusPageID(ctx, slug, userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	rows, err := h.pool.Query(ctx,
		`SELECT id, channel_type, endpoint, verified, events, created_at
		 FROM status_page_subscriptions
		 WHERE status_page_id = $1
		 ORDER BY created_at DESC`,
		statusPageID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list subscriptions", err))
		return
	}
	defer rows.Close()

	subs := make([]statusSubResponse, 0)
	for rows.Next() {
		var s statusSubResponse
		if err := rows.Scan(&s.ID, &s.ChannelType, &s.Endpoint, &s.Verified, &s.Events, &s.CreatedAt); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan subscription", err))
			return
		}
		subs = append(subs, s)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate subscriptions", err))
		return
	}

	response.JSON(w, r, http.StatusOK, subs)
}

func (h *StatusSubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("authentication required"))
		return
	}

	slug := chi.URLParam(r, "slug")
	subID := chi.URLParam(r, "id")

	statusPageID, err := h.resolveOwnedStatusPageID(ctx, slug, userID)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if _, err = h.pool.Exec(ctx,
		`DELETE FROM status_page_subscriptions WHERE id = $1 AND status_page_id = $2`,
		subID, statusPageID,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to delete subscription", err))
		return
	}

	response.JSON(w, r, http.StatusNoContent, nil)
}

func generateVerifyToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generateVerifyToken: %w", err)
	}
	return hex.EncodeToString(b), nil
}
