package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

// VerdictOrderHandler handles POST /v1/verdict/orders + GET /v1/verdict/orders/{id}.
//
// The flow on Create:
//  1. Validate request (template, target, time window, channel).
//  2. Resolve price from template.
//  3. INSERT a `pending` row into idcd_attest.verdict_order.
//  4. Call billing.Provider.Charge with verdict_order_id in metadata.
//  5. Persist charge_id + pay_url back onto the row (so webhook can correlate
//     and the user can re-open the pay URL from order detail).
//  6. Return {order_id, pay_url, price_cny}.
//
// The payment-succeeded webhook (see billing.go) flips status pending→paid and
// pushes the order onto the Redis Stream `verdict_generation_queue` — at that
// point apps/attest takes over.
type VerdictOrderHandler struct {
	pool     BillingPool
	provider billing.Provider
	pricing  billing.Pricing
}

// NewVerdictOrderHandler wires a VerdictOrderHandler.
// Pricing must be wired via WithPricing before Create — otherwise 500.
func NewVerdictOrderHandler(pool BillingPool, provider billing.Provider) *VerdictOrderHandler {
	return &VerdictOrderHandler{pool: pool, provider: provider}
}

// WithPricing wires the unified Pricing service for verdict template pricing
// + promo evaluation. Create 500s if nil.
func (h *VerdictOrderHandler) WithPricing(p billing.Pricing) *VerdictOrderHandler {
	h.pricing = p
	return h
}

// ---- request / response types ----

// CreateVerdictOrderRequest is the JSON body posted by users to create
// a verdict report order.
type CreateVerdictOrderRequest struct {
	Template        string    `json:"template"`          // sla|incident|compliance|legal
	Target          string    `json:"target"`            // domain|url|ip
	TimeWindowStart time.Time `json:"time_window_start"` // RFC3339
	TimeWindowEnd   time.Time `json:"time_window_end"`   // RFC3339
	Channel         string    `json:"channel"`           // alipay|wechat_pay
	ReturnURL       string    `json:"return_url,omitempty"`
	Coupon          string    `json:"coupon,omitempty"`  // optional discount code
}

// CreateVerdictOrderResponse is returned on successful order creation.
type CreateVerdictOrderResponse struct {
	OrderID  string `json:"order_id"`  // v_*
	PayURL   string `json:"pay_url"`   // gateway-supplied checkout / QR URL
	PriceCNY int64  `json:"price_cny"` // 元 (yuan), not fen
}

// GetVerdictOrderResponse is the user-facing projection of one verdict_order row.
type GetVerdictOrderResponse struct {
	ID              string     `json:"id"`
	OwnerID         string     `json:"owner_id"`
	Template        string     `json:"template"`
	Target          string     `json:"target"`
	TimeWindowStart time.Time  `json:"time_window_start"`
	TimeWindowEnd   time.Time  `json:"time_window_end"`
	Status          string     `json:"status"`
	PriceCNY        float64    `json:"price_cny"`
	PricePaidCNY    *float64   `json:"price_paid_cny,omitempty"`
	ExtOrderID   *string    `json:"ext_order_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
}

// ---- Create ----

// Create handles POST /v1/verdict/orders.
func (h *VerdictOrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req CreateVerdictOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	// --- validation ---
	if !billing.ValidUserChannel(req.Channel) {
		response.Error(w, r, apperr.Validation("channel must be alipay or wechat_pay", "channel"))
		return
	}
	target := req.Target
	if l := len(target); l == 0 || l > 512 {
		response.Error(w, r, apperr.Validation("target is required (1-512 chars)", "target"))
		return
	}
	if req.TimeWindowStart.IsZero() || req.TimeWindowEnd.IsZero() {
		response.Error(w, r, apperr.Validation("time_window_start and time_window_end are required", "time_window"))
		return
	}
	if !req.TimeWindowStart.Before(req.TimeWindowEnd) {
		response.Error(w, r, apperr.Validation("time_window_start must be before time_window_end", "time_window"))
		return
	}

	// Pricing service required for item-existence + price calc. Wire bug surfaces
	// as 500 (system error), distinct from 400 user-validation failures above.
	if h.pricing == nil {
		response.Error(w, r, apperr.Internal("verdict pricing service not wired", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !h.pricing.ValidItem(ctx, billing.KindVerdictTemplate, req.Template) {
		response.Error(w, r, apperr.Validation("template must be sla|incident|compliance|legal", "template"))
		return
	}

	// Compute final price (auto-promo + coupon validation). PricingError → 400.
	price, perr := h.pricing.EffectivePrice(ctx, billing.KindVerdictTemplate, req.Template, req.Coupon)
	if perr != nil {
		var pricingErr *billing.PricingError
		if errors.As(perr, &pricingErr) {
			response.Error(w, r, apperr.Validation(pricingErr.Reason, "coupon"))
			return
		}
		response.Error(w, r, apperr.Internal("pricing lookup failed", perr))
		return
	}
	priceCNY := float64(price.FinalCents) / 100

	// --- persist pending order ---
	orderID := idgen.VerdictOrder()
	now := time.Now().UTC()
	var promoArg any
	if price.PromotionID != "" {
		promoArg = price.PromotionID
	}

	if _, err := h.pool.Exec(ctx, `
		INSERT INTO idcd_attest.verdict_order
			(id, owner_id, template, target, time_window_start, time_window_end,
			 status, price_cny, promotion_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9)
	`,
		orderID, userID, req.Template, target,
		req.TimeWindowStart.UTC(), req.TimeWindowEnd.UTC(),
		priceCNY, promoArg, now,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to persist verdict_order", err))
		return
	}

	// --- gateway charge ---
	metadata := map[string]string{"verdict_order_id": orderID}
	if price.PromotionID != "" {
		metadata["promotion_id"] = price.PromotionID
	}
	notifyURL := buildVerdictNotifyURL(r)
	result, err := h.provider.Charge(ctx, billing.ChargeRequest{
		UserID:      userID,
		AmountCents: price.FinalCents,
		Currency:    price.Currency,
		Channel:     req.Channel,
		ReturnURL:   req.ReturnURL,
		NotifyURL:   notifyURL,
		ItemRef:     orderID,
		Description: fmt.Sprintf("Verdict %s 报告", req.Template),
		Metadata:    metadata,
	})
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to initiate charge", err))
		return
	}

	if price.PromotionID != "" {
		if err := h.pricing.IncrementPromotionUsage(ctx, price.PromotionID); err != nil {
			_ = err // best-effort
		}
	}

	response.JSON(w, r, http.StatusOK, CreateVerdictOrderResponse{
		OrderID:  orderID,
		PayURL:   result.PayURL,
		PriceCNY: price.FinalCents / 100,
	})
}

// ---- Get ----

// Get handles GET /v1/verdict/orders/{id}.
// Only the owner can read their own orders (D1: cross-schema row guard).
func (h *VerdictOrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("missing id", "id"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT id, owner_id, template, target,
		       time_window_start, time_window_end,
		       status, price_cny, price_paid_cny, ext_order_id,
		       created_at, paid_at, delivered_at
		FROM idcd_attest.verdict_order
		WHERE id = $1
	`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query verdict_order", err))
		return
	}
	defer rows.Close()

	if !rows.Next() {
		response.Error(w, r, apperr.NotFound("verdict_order not found"))
		return
	}

	var resp GetVerdictOrderResponse
	if err := rows.Scan(
		&resp.ID, &resp.OwnerID, &resp.Template, &resp.Target,
		&resp.TimeWindowStart, &resp.TimeWindowEnd,
		&resp.Status, &resp.PriceCNY, &resp.PricePaidCNY, &resp.ExtOrderID,
		&resp.CreatedAt, &resp.PaidAt, &resp.DeliveredAt,
	); err != nil {
		response.Error(w, r, apperr.Internal("failed to scan verdict_order", err))
		return
	}
	rows.Close()

	if resp.OwnerID != userID {
		response.Error(w, r, apperr.NotFound("verdict_order not found"))
		return
	}

	response.JSON(w, r, http.StatusOK, resp)
}

// buildVerdictNotifyURL builds the webhook URL the gateway should POST to
// when the charge clears.  Uses the Origin header (matches existing
// Subscribe handler convention) so the dev / staging / prod hosts work
// without per-env config.
func buildVerdictNotifyURL(r *http.Request) string {
	origin := r.Header.Get("Origin")
	return origin + "/v1/billing/webhook"
}
