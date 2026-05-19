package handler

import (
	"context"
	"encoding/json"
	"errors"
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
	pricing  billing.Pricing

	// appBaseURL is the frontend origin used to build user-facing ReturnURL.
	appBaseURL string
	// publicAPIURL is the API service's externally-reachable origin used to
	// build NotifyURL. MUST come from server config — never the Origin header,
	// since trusting a client header lets a forged origin redirect refund /
	// payment webhooks to an attacker-controlled URL.
	publicAPIURL string
}

func NewTeamBillingHandler(pool TeamBillingPool, provider billing.Provider) *TeamBillingHandler {
	return &TeamBillingHandler{pool: pool, provider: provider}
}

// WithURLs configures the trusted server-side base URLs used to construct
// ReturnURL (user browser redirect after pay) and NotifyURL (server-to-server
// webhook callback). Mirrors BillingHandler.WithURLs.
func (h *TeamBillingHandler) WithURLs(appBase, publicAPI string) *TeamBillingHandler {
	h.appBaseURL = appBase
	h.publicAPIURL = publicAPI
	return h
}

// WithPricing wires the unified Pricing service. Subscribe 500s if nil.
func (h *TeamBillingHandler) WithPricing(p billing.Pricing) *TeamBillingHandler {
	h.pricing = p
	return h
}

type teamSubscribeRequest struct {
	Plan   string `json:"plan"`
	Coupon string `json:"coupon,omitempty"`
}

// requireTeamAdmin returns a typed application error when the caller is not
// an owner/admin of the team. See TeamAPIKeyHandler.requireAdminRole for the
// reason this no longer reuses pgx.ErrNoRows as a permission signal.
func (h *TeamBillingHandler) requireTeamAdmin(ctx context.Context, teamID, userID string) *apperr.Error {
	var role string
	err := h.pool.QueryRow(ctx,
		`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Forbidden("not a team member")
		}
		return apperr.Internal("failed to verify team role", err)
	}
	if role != "owner" && role != "admin" {
		return apperr.Forbidden("owner or admin required")
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

	if appErr := h.requireTeamAdmin(r.Context(), teamID, userID); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	var req teamSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	plan := billing.Plan(req.Plan)

	if h.pricing == nil {
		response.Error(w, r, apperr.Internal("team billing pricing service not wired", nil))
		return
	}

	ownerID, err := h.getTeamOwner(r.Context(), teamID)
	if err != nil {
		response.Error(w, r, apperr.NotFound("team not found"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !h.pricing.ValidItem(ctx, billing.KindPlan, string(plan)) {
		response.Error(w, r, apperr.Validation("unknown plan", "plan"))
		return
	}

	price, perr := h.pricing.EffectivePrice(ctx, billing.KindPlan, string(plan), req.Coupon)
	if perr != nil {
		var pricingErr *billing.PricingError
		if errors.As(perr, &pricingErr) {
			response.Error(w, r, apperr.Validation(pricingErr.Reason, "coupon"))
			return
		}
		response.Error(w, r, apperr.Internal("pricing lookup failed", perr))
		return
	}

	// ReturnURL falls back to the request Origin only in non-production setups
	// where app_base_url is unset; production deployments MUST configure it.
	returnBase := h.appBaseURL
	if returnBase == "" {
		returnBase = r.Header.Get("Origin")
	}
	// NotifyURL: server-to-server webhook callback. MUST come from config.
	if h.publicAPIURL == "" {
		response.Error(w, r, apperr.Internal("team billing public_api_url is not configured", nil))
		return
	}

	metadata := map[string]string{"team_id": teamID}
	if price.PromotionID != "" {
		metadata["promotion_id"] = price.PromotionID
	}

	result, err := h.provider.Subscribe(ctx, billing.SubscribeRequest{
		UserID:      ownerID,
		Plan:        plan,
		ReturnURL:   returnBase + "/app/settings/team",
		NotifyURL:   h.publicAPIURL + "/v1/billing/webhook",
		AmountCents: price.FinalCents,
		Currency:    price.Currency,
		Metadata:    metadata,
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
	var promoArg any
	if price.PromotionID != "" {
		promoArg = price.PromotionID
	}

	_, dbErr := h.pool.Exec(ctx, `
		INSERT INTO subscriptions
			(id, user_id, plan, status, provider, ext_sub_id,
			 current_period_start, current_period_end,
			 amount_cents, currency, promotion_id,
			 created_at, updated_at)
		VALUES ($1, $2, $3, 'pending', $4, $5, $6, $7, $8, $9, $10, $11, $11)
		ON CONFLICT (id) DO NOTHING
	`,
		subID, ownerID, string(plan), h.provider.Name(), result.ExtSubID,
		now, result.ExpiresAt,
		price.FinalCents, price.Currency, promoArg,
		now,
	)
	if dbErr != nil {
		response.Error(w, r, apperr.Internal("failed to persist subscription", dbErr))
		return
	}

	if price.PromotionID != "" {
		if err := h.pricing.IncrementPromotionUsage(ctx, price.PromotionID); err != nil {
			// best-effort; log via response.Error 不合适，这里直接吞 + 留 metadata
			_ = err
		}
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

	// Match the {data: {<resource>: ...}} naming the rest of the team API uses
	// (teams, members, api_keys, invitations) — flat struct here would force
	// the frontend into a special-case unwrapper.
	response.JSON(w, r, http.StatusOK, map[string]any{"subscription": sub})
}
