package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/kite365/idcd/apps/api/internal/billing"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// PricingInvalidator 用于本进程的 in-memory 价格 cache 失效。
// admin 改 pricing_items / pricing_promotions 后调用,避免最长 cacheTTL
// 的 stale 价格窗口。多实例 HA 部署另需 PubSub 广播,S1 单实例可忽略。
type PricingInvalidator interface {
	Invalidate()
}

// AdminPricingHandler exposes CRUD over pricing_items + pricing_promotions.
// Mounted under /v1/admin/billing/* behind the admin token middleware
// (network-level gate; per-user admin role still TODO — same posture as
// AdminBillingHandler refund management).
type AdminPricingHandler struct {
	pool        BillingPool
	invalidator PricingInvalidator // optional; nil 等同于不主动 invalidate
}

// NewAdminPricingHandler wires the handler against the shared pgx pool.
func NewAdminPricingHandler(pool BillingPool) *AdminPricingHandler {
	return &AdminPricingHandler{pool: pool}
}

// WithInvalidator 注入 cache 失效器。建议传入与 BillingHandler 共享的
// *billing.DBPricing,价格 / 促销变更后立即对所有支付 handler 可见。
func (h *AdminPricingHandler) WithInvalidator(inv PricingInvalidator) *AdminPricingHandler {
	h.invalidator = inv
	return h
}

// invalidate 调用注入的 Invalidator(若有)。所有写操作 (UpdatePricingItem /
// CreatePromotion / UpdatePromotion / DeletePromotion) 末尾应该调用一次。
func (h *AdminPricingHandler) invalidate() {
	if h.invalidator != nil {
		h.invalidator.Invalidate()
	}
}

// ---- pricing items ----

type pricingItemDTO struct {
	Kind       string    `json:"kind"`
	ItemKey    string    `json:"item_key"`
	PriceCents int64     `json:"price_cents"`
	Currency   string    `json:"currency"`
	UpdatedAt  time.Time `json:"updated_at"`
	UpdatedBy  *string   `json:"updated_by,omitempty"`
}

type pricingItemsListResponse struct {
	Items []pricingItemDTO `json:"items"`
	Total int              `json:"total"`
}

// ListPricingItems handles GET /v1/admin/billing/pricing-items.
// Optional ?kind=plan filters by kind.
func (h *AdminPricingHandler) ListPricingItems(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	kindFilter := strings.TrimSpace(r.URL.Query().Get("kind"))
	var (
		rows pgx.Rows
		err  error
	)
	if kindFilter != "" {
		rows, err = h.pool.Query(ctx, `
			SELECT kind, item_key, price_cents, currency, updated_at, updated_by
			FROM pricing_items WHERE kind = $1
			ORDER BY kind, price_cents
		`, kindFilter)
	} else {
		rows, err = h.pool.Query(ctx, `
			SELECT kind, item_key, price_cents, currency, updated_at, updated_by
			FROM pricing_items ORDER BY kind, price_cents
		`)
	}
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query pricing_items", err))
		return
	}
	defer rows.Close()

	items := []pricingItemDTO{}
	for rows.Next() {
		var it pricingItemDTO
		if err := rows.Scan(&it.Kind, &it.ItemKey, &it.PriceCents, &it.Currency, &it.UpdatedAt, &it.UpdatedBy); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan pricing_item", err))
			return
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("row iteration error", err))
		return
	}

	response.JSON(w, r, http.StatusOK, pricingItemsListResponse{Items: items, Total: len(items)})
}

type updatePricingItemRequest struct {
	PriceCents *int64  `json:"price_cents,omitempty"`
	Currency   *string `json:"currency,omitempty"`
}

// UpdatePricingItem handles PATCH /v1/admin/billing/pricing-items/{kind}/{item_key}.
// Both price_cents and currency are optional; at least one must be set.
// updated_by is taken from the admin token holder's user id (when authn middleware
// is in front; otherwise the column stays NULL).
func (h *AdminPricingHandler) UpdatePricingItem(w http.ResponseWriter, r *http.Request) {
	kind := chi.URLParam(r, "kind")
	itemKey := chi.URLParam(r, "item_key")
	if kind == "" || itemKey == "" {
		response.Error(w, r, apperr.Validation("kind and item_key are required", "path"))
		return
	}

	var req updatePricingItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.PriceCents == nil && req.Currency == nil {
		response.Error(w, r, apperr.Validation("at least one of price_cents or currency required", "body"))
		return
	}
	if req.PriceCents != nil && *req.PriceCents < 0 {
		response.Error(w, r, apperr.Validation("price_cents must be >= 0", "price_cents"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	updatedBy := middleware.UserIDFromContext(r.Context())
	var updatedByArg any
	if updatedBy != "" {
		updatedByArg = updatedBy
	}

	tag, err := h.pool.Exec(ctx, `
		UPDATE pricing_items
		SET price_cents = COALESCE($3, price_cents),
		    currency    = COALESCE($4, currency),
		    updated_at  = NOW(),
		    updated_by  = COALESCE($5, updated_by)
		WHERE kind = $1 AND item_key = $2
	`, kind, itemKey, req.PriceCents, req.Currency, updatedByArg)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update pricing_item", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("pricing item not found"))
		return
	}

	// Return the updated row.
	row := h.pool.QueryRow(ctx, `
		SELECT kind, item_key, price_cents, currency, updated_at, updated_by
		FROM pricing_items WHERE kind = $1 AND item_key = $2
	`, kind, itemKey)
	var it pricingItemDTO
	if err := row.Scan(&it.Kind, &it.ItemKey, &it.PriceCents, &it.Currency, &it.UpdatedAt, &it.UpdatedBy); err != nil {
		response.Error(w, r, apperr.Internal("failed to read updated pricing_item", err))
		return
	}
	h.invalidate()
	response.JSON(w, r, http.StatusOK, it)
}

// ---- promotions ----

type promotionDTO struct {
	ID           string    `json:"id"`
	AppliesKind  *string   `json:"applies_kind,omitempty"`
	AppliesKey   *string   `json:"applies_key,omitempty"`
	DiscountType string    `json:"discount_type"`
	Value        int64     `json:"value"`
	CouponCode   *string   `json:"coupon_code,omitempty"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	MaxUses      *int      `json:"max_uses,omitempty"`
	UsedCount    int       `json:"used_count"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type promotionsListResponse struct {
	Promotions []promotionDTO `json:"promotions"`
	Total      int            `json:"total"`
}

// ListPromotions handles GET /v1/admin/billing/promotions.
// Query params: ?active=true|false (default both), ?kind=plan filters by applies_kind.
func (h *AdminPricingHandler) ListPromotions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Build WHERE incrementally; positional args 1-indexed.
	clauses := []string{}
	args := []any{}
	idx := 1
	if v := r.URL.Query().Get("active"); v == "true" {
		clauses = append(clauses, "active = TRUE")
	} else if v == "false" {
		clauses = append(clauses, "active = FALSE")
	}
	if v := strings.TrimSpace(r.URL.Query().Get("kind")); v != "" {
		clauses = append(clauses, "(applies_kind IS NULL OR applies_kind = $"+strconv.Itoa(idx)+")")
		args = append(args, v)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := h.pool.Query(ctx, `
		SELECT id, applies_kind, applies_key, discount_type, value, coupon_code,
		       start_at, end_at, max_uses, used_count, active, created_at, updated_at
		FROM pricing_promotions`+where+`
		ORDER BY created_at DESC
		LIMIT 200
	`, args...)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to query promotions", err))
		return
	}
	defer rows.Close()

	promos := []promotionDTO{}
	for rows.Next() {
		var p promotionDTO
		if err := rows.Scan(
			&p.ID, &p.AppliesKind, &p.AppliesKey, &p.DiscountType, &p.Value, &p.CouponCode,
			&p.StartAt, &p.EndAt, &p.MaxUses, &p.UsedCount, &p.Active, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan promotion", err))
			return
		}
		promos = append(promos, p)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("row iteration error", err))
		return
	}

	response.JSON(w, r, http.StatusOK, promotionsListResponse{Promotions: promos, Total: len(promos)})
}

// GetPromotion handles GET /v1/admin/billing/promotions/{id}.
func (h *AdminPricingHandler) GetPromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("id is required", "path"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	row := h.pool.QueryRow(ctx, `
		SELECT id, applies_kind, applies_key, discount_type, value, coupon_code,
		       start_at, end_at, max_uses, used_count, active, created_at, updated_at
		FROM pricing_promotions WHERE id = $1
	`, id)
	var p promotionDTO
	if err := row.Scan(
		&p.ID, &p.AppliesKind, &p.AppliesKey, &p.DiscountType, &p.Value, &p.CouponCode,
		&p.StartAt, &p.EndAt, &p.MaxUses, &p.UsedCount, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(w, r, apperr.NotFound("promotion not found"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to read promotion", err))
		return
	}
	response.JSON(w, r, http.StatusOK, p)
}

type createPromotionRequest struct {
	AppliesKind  *string   `json:"applies_kind,omitempty"`
	AppliesKey   *string   `json:"applies_key,omitempty"`
	DiscountType string    `json:"discount_type"`
	Value        int64     `json:"value"`
	CouponCode   *string   `json:"coupon_code,omitempty"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	MaxUses      *int      `json:"max_uses,omitempty"`
	Active       *bool     `json:"active,omitempty"`
}

// CreatePromotion handles POST /v1/admin/billing/promotions.
func (h *AdminPricingHandler) CreatePromotion(w http.ResponseWriter, r *http.Request) {
	var req createPromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if appErr := validatePromotionPayload(req.DiscountType, req.Value, req.AppliesKind, req.AppliesKey, req.StartAt, req.EndAt); appErr != nil {
		response.Error(w, r, appErr)
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}
	id := billing.NewPromotionID()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err := h.pool.Exec(ctx, `
		INSERT INTO pricing_promotions
			(id, applies_kind, applies_key, discount_type, value, coupon_code,
			 start_at, end_at, max_uses, used_count, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, NOW(), NOW())
	`,
		id, req.AppliesKind, req.AppliesKey, req.DiscountType, req.Value, req.CouponCode,
		req.StartAt.UTC(), req.EndAt.UTC(), req.MaxUses, active,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to insert promotion", err))
		return
	}
	h.invalidate()
	response.JSON(w, r, http.StatusCreated, map[string]string{"id": id})
}

type updatePromotionRequest struct {
	Active     *bool      `json:"active,omitempty"`
	EndAt      *time.Time `json:"end_at,omitempty"`
	MaxUses    *int       `json:"max_uses,omitempty"`
	CouponCode *string    `json:"coupon_code,omitempty"`
}

// UpdatePromotion handles PATCH /v1/admin/billing/promotions/{id}.
// Intentionally narrow: only fields safe to mutate after launch
// (active toggle, end_at extend, max_uses bump, coupon_code rename).
// To change discount_type / value / applies_*, archive (DELETE) and create a new promo.
func (h *AdminPricingHandler) UpdatePromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("id is required", "path"))
		return
	}
	var req updatePromotionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Active == nil && req.EndAt == nil && req.MaxUses == nil && req.CouponCode == nil {
		response.Error(w, r, apperr.Validation("at least one mutable field required", "body"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var endAtArg any
	if req.EndAt != nil {
		endAtArg = req.EndAt.UTC()
	}

	tag, err := h.pool.Exec(ctx, `
		UPDATE pricing_promotions SET
			active      = COALESCE($2, active),
			end_at      = COALESCE($3, end_at),
			max_uses    = COALESCE($4, max_uses),
			coupon_code = COALESCE($5, coupon_code),
			updated_at  = NOW()
		WHERE id = $1
	`, id, req.Active, endAtArg, req.MaxUses, req.CouponCode)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to update promotion", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("promotion not found"))
		return
	}
	h.invalidate()
	response.JSON(w, r, http.StatusOK, map[string]string{"id": id, "status": "updated"})
}

// DeletePromotion handles DELETE /v1/admin/billing/promotions/{id}.
// Soft delete: sets active=false instead of removing the row, preserving used_count
// and historical correlation from subscriptions/verdict_order.promotion_id.
func (h *AdminPricingHandler) DeletePromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("id is required", "path"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tag, err := h.pool.Exec(ctx,
		`UPDATE pricing_promotions SET active = FALSE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to deactivate promotion", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("promotion not found"))
		return
	}
	h.invalidate()
	response.JSON(w, r, http.StatusOK, map[string]string{"id": id, "status": "deactivated"})
}

// ---- helpers ----

// validatePromotionPayload mirrors the CHECK constraints in 00049 plus a few
// extra-friendly 400 messages (start < end, kind/key combination sanity).
func validatePromotionPayload(discountType string, value int64, appliesKind, appliesKey *string, startAt, endAt time.Time) *apperr.Error {
	switch discountType {
	case "percent":
		if value <= 0 || value > 100 {
			return apperr.Validation("percent value must be 1..100", "value")
		}
	case "amount", "override":
		if value <= 0 {
			return apperr.Validation("value must be > 0", "value")
		}
	default:
		return apperr.Validation("discount_type must be percent|amount|override", "discount_type")
	}
	if startAt.IsZero() || endAt.IsZero() {
		return apperr.Validation("start_at and end_at are required", "time")
	}
	if !startAt.Before(endAt) {
		return apperr.Validation("start_at must be before end_at", "time")
	}
	if appliesKind == nil && appliesKey != nil {
		return apperr.Validation("applies_key requires applies_kind to be set", "applies_kind")
	}
	return nil
}

