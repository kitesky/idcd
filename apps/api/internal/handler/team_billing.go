package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

type TeamBillingPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TeamBillingHandler struct {
	pool     TeamBillingPool
	provider billing.Provider
}

func NewTeamBillingHandler(pool TeamBillingPool, provider billing.Provider) *TeamBillingHandler {
	return &TeamBillingHandler{pool: pool, provider: provider}
}

type teamSubscribeRequest struct {
	Plan string `json:"plan"`
}

func (h *TeamBillingHandler) requireTeamAdmin(ctx context.Context, teamID, userID string) error {
	var role string
	err := h.pool.QueryRow(ctx,
		`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return pgx.ErrNoRows
	}
	return nil
}

func (h *TeamBillingHandler) getTeamOwner(ctx context.Context, teamID string) (string, error) {
	var ownerID string
	err := h.pool.QueryRow(ctx,
		`SELECT owner_id FROM teams WHERE id = $1`,
		teamID,
	).Scan(&ownerID)
	return ownerID, err
}

func (h *TeamBillingHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	if err := h.requireTeamAdmin(r.Context(), teamID, userID); err != nil {
		response.Error(w, r, apperr.Forbidden("owner or admin required"))
		return
	}

	var req teamSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	plan := billing.Plan(req.Plan)
	if !billing.ValidPlan(plan) {
		response.Error(w, r, apperr.Validation("unknown plan", "plan"))
		return
	}

	ownerID, err := h.getTeamOwner(r.Context(), teamID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("team not found"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result, err := h.provider.Subscribe(ctx, billing.SubscribeRequest{
		UserID:    ownerID,
		Plan:      plan,
		ReturnURL: r.Header.Get("Origin") + "/app/settings/team",
		NotifyURL: r.Header.Get("Origin") + "/v1/billing/webhook",
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to initiate subscription", err))
		return
	}

	now := time.Now().UTC()
	subID := result.SubscriptionID
	if subID == "" {
		subID = idgen.Subscription()
	}

	_, dbErr := h.pool.Exec(ctx, `
		INSERT INTO subscriptions
			(id, user_id, plan, status, provider, ext_sub_id,
			 current_period_start, current_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $8)
		ON CONFLICT (id) DO NOTHING
	`,
		subID, ownerID, string(plan), h.provider.Name(), result.ExtSubID,
		now, result.ExpiresAt, now,
	)
	if dbErr != nil {
		response.Error(w, r, apperr.Internal("failed to persist subscription", dbErr))
		return
	}

	response.JSON(w, r, http.StatusOK, subscribeResponse{
		SubscriptionID: subID,
		PayURL:         result.PayURL,
		ExpiresAt:      result.ExpiresAt,
	})
}

func (h *TeamBillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	teamID := chi.URLParam(r, "id")

	var isMember bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM team_memberships WHERE team_id = $1 AND user_id = $2)`,
		teamID, userID,
	).Scan(&isMember); err != nil || !isMember {
		response.Error(w, r, apperr.Forbidden("not a team member"))
		return
	}

	ownerID, err := h.getTeamOwner(r.Context(), teamID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("team not found"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, plan, status, provider, ext_sub_id,
		       current_period_start, current_period_end, cancel_at, created_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, ownerID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query subscription", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("no subscription found"))
		return
	}

	var sub subscriptionResponse
	if err := rows.Scan(
		&sub.ID, &sub.Plan, &sub.Status, &sub.Provider, &sub.ExtSubID,
		&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CancelAt, &sub.CreatedAt,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan subscription", err))
		return
	}

	response.JSON(w, r, http.StatusOK, sub)
}
