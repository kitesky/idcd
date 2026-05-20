package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAdminPricingTestHandler(t *testing.T) (*AdminPricingHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewAdminPricingHandler(mockPool), mockPool
}

// helper that runs the handler against a chi router so URL params resolve.
func runWithChi(method, path string, body []byte, mount func(r chi.Router)) *httptest.ResponseRecorder {
	r := chi.NewRouter()
	mount(r)
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// ---- ListPricingItems ----

func TestAdminPricing_ListPricingItems_All(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	now := time.Now()
	mock.ExpectQuery(`FROM pricing_items ORDER BY`).
		WillReturnRows(pgxmock.NewRows([]string{"kind", "item_key", "price_cents", "currency", "updated_at", "updated_by"}).
			AddRow("plan", "pro", int64(9900), "CNY", now, nil).
			AddRow("verdict_template", "sla", int64(29900), "CNY", now, nil))

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/pricing-items", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/pricing-items", h.ListPricingItems)
	})
	require.Equal(t, http.StatusOK, rr.Code)
	var resp pricingItemsListResponse
	require.NoError(t, decodeData(t, rr.Body.Bytes(), &resp))
	assert.Equal(t, 2, resp.Total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPricing_ListPricingItems_FilterByKind(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectQuery(`FROM pricing_items WHERE kind = \$1`).
		WithArgs("plan").
		WillReturnRows(pgxmock.NewRows([]string{"kind", "item_key", "price_cents", "currency", "updated_at", "updated_by"}).
			AddRow("plan", "pro", int64(9900), "CNY", time.Now(), nil))

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/pricing-items?kind=plan", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/pricing-items", h.ListPricingItems)
	})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---- UpdatePricingItem ----

func TestAdminPricing_UpdatePricingItem_Success(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`UPDATE pricing_items`).
		WithArgs("plan", "pro", i64Ptr(8900), nilStrPtr(), nilStrAny()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery(`SELECT kind, item_key, price_cents, currency, updated_at, updated_by`).
		WithArgs("plan", "pro").
		WillReturnRows(pgxmock.NewRows([]string{"kind", "item_key", "price_cents", "currency", "updated_at", "updated_by"}).
			AddRow("plan", "pro", int64(8900), "CNY", time.Now(), nil))

	body, _ := json.Marshal(map[string]any{"price_cents": 8900})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/pricing-items/plan/pro", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/pricing-items/{kind}/{item_key}", h.UpdatePricingItem)
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPricing_UpdatePricingItem_NotFound(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`UPDATE pricing_items`).
		WithArgs("plan", "ghost", i64Ptr(1), nilStrPtr(), nilStrAny()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	body, _ := json.Marshal(map[string]any{"price_cents": 1})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/pricing-items/plan/ghost", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/pricing-items/{kind}/{item_key}", h.UpdatePricingItem)
	})
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestAdminPricing_UpdatePricingItem_NoFields(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/pricing-items/plan/pro", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/pricing-items/{kind}/{item_key}", h.UpdatePricingItem)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminPricing_UpdatePricingItem_NegativePrice(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{"price_cents": -100})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/pricing-items/plan/pro", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/pricing-items/{kind}/{item_key}", h.UpdatePricingItem)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- ListPromotions ----

func TestAdminPricing_ListPromotions_All(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	now := time.Now()
	mock.ExpectQuery(`FROM pricing_promotions\s*\s+ORDER BY created_at DESC`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "coupon_code",
			"start_at", "end_at", "max_uses", "used_count", "active", "created_at", "updated_at",
		}).AddRow(
			"promo_1", strPtrLocal("plan"), strPtrLocal("pro"), "percent", int64(10), nilStrPtr(),
			now, now.Add(24*time.Hour), nilIntPtr(), 0, true, now, now,
		))

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/promotions", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/promotions", h.ListPromotions)
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp promotionsListResponse
	require.NoError(t, decodeData(t, rr.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

func TestAdminPricing_ListPromotions_FilterActive(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectQuery(`FROM pricing_promotions\s+WHERE active = TRUE`).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "coupon_code",
			"start_at", "end_at", "max_uses", "used_count", "active", "created_at", "updated_at",
		}))

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/promotions?active=true", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/promotions", h.ListPromotions)
	})
	require.Equal(t, http.StatusOK, rr.Code)
}

// ---- GetPromotion ----

func TestAdminPricing_GetPromotion_Success(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	now := time.Now()
	mock.ExpectQuery(`FROM pricing_promotions WHERE id = \$1`).
		WithArgs("promo_x").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "coupon_code",
			"start_at", "end_at", "max_uses", "used_count", "active", "created_at", "updated_at",
		}).AddRow(
			"promo_x", strPtrLocal("plan"), strPtrLocal("pro"), "percent", int64(10), nilStrPtr(),
			now, now.Add(24*time.Hour), nilIntPtr(), 0, true, now, now,
		))

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/promotions/promo_x", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/promotions/{id}", h.GetPromotion)
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestAdminPricing_GetPromotion_NotFound(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectQuery(`FROM pricing_promotions WHERE id = \$1`).
		WithArgs("promo_missing").
		WillReturnError(pgx.ErrNoRows)

	rr := runWithChi(http.MethodGet, "/v1/admin/billing/promotions/promo_missing", nil, func(r chi.Router) {
		r.Get("/v1/admin/billing/promotions/{id}", h.GetPromotion)
	})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- CreatePromotion ----

func TestAdminPricing_CreatePromotion_Success(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`INSERT INTO pricing_promotions`).
		WithArgs(
			pgxmock.AnyArg(),    // id (generated)
			strPtrLocal("plan"), // applies_kind
			strPtrLocal("pro"),  // applies_key
			"percent",
			int64(10),
			nilStrPtr(),       // coupon_code
			pgxmock.AnyArg(),  // start_at
			pgxmock.AnyArg(),  // end_at
			nilIntPtr(),       // max_uses
			true,              // active
		).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(map[string]any{
		"applies_kind":  "plan",
		"applies_key":   "pro",
		"discount_type": "percent",
		"value":         10,
		"start_at":      time.Now().UTC().Format(time.RFC3339),
		"end_at":        time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
	})
	rr := runWithChi(http.MethodPost, "/v1/admin/billing/promotions", body, func(r chi.Router) {
		r.Post("/v1/admin/billing/promotions", h.CreatePromotion)
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPricing_CreatePromotion_InvalidPercent(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{
		"discount_type": "percent",
		"value":         150, // > 100
		"start_at":      time.Now().UTC().Format(time.RFC3339),
		"end_at":        time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	rr := runWithChi(http.MethodPost, "/v1/admin/billing/promotions", body, func(r chi.Router) {
		r.Post("/v1/admin/billing/promotions", h.CreatePromotion)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminPricing_CreatePromotion_BadTimeRange(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{
		"discount_type": "amount",
		"value":         500,
		"start_at":      time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		"end_at":        time.Now().UTC().Format(time.RFC3339), // earlier than start
	})
	rr := runWithChi(http.MethodPost, "/v1/admin/billing/promotions", body, func(r chi.Router) {
		r.Post("/v1/admin/billing/promotions", h.CreatePromotion)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestAdminPricing_CreatePromotion_KeyWithoutKind(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{
		"applies_key":   "pro", // without applies_kind
		"discount_type": "amount",
		"value":         500,
		"start_at":      time.Now().UTC().Format(time.RFC3339),
		"end_at":        time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	rr := runWithChi(http.MethodPost, "/v1/admin/billing/promotions", body, func(r chi.Router) {
		r.Post("/v1/admin/billing/promotions", h.CreatePromotion)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- UpdatePromotion ----

func TestAdminPricing_UpdatePromotion_ToggleActive(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`UPDATE pricing_promotions SET\s+active`).
		WithArgs("promo_x", boolPtr(false), nilTimeAny(), nilIntPtr(), nilStrPtr()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(map[string]any{"active": false})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/promotions/promo_x", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/promotions/{id}", h.UpdatePromotion)
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestAdminPricing_UpdatePromotion_NoFields(t *testing.T) {
	h, _ := newAdminPricingTestHandler(t)
	body, _ := json.Marshal(map[string]any{})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/promotions/promo_x", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/promotions/{id}", h.UpdatePromotion)
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- DeletePromotion ----

func TestAdminPricing_DeletePromotion_Success(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`UPDATE pricing_promotions SET active = FALSE`).
		WithArgs("promo_x").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	rr := runWithChi(http.MethodDelete, "/v1/admin/billing/promotions/promo_x", nil, func(r chi.Router) {
		r.Delete("/v1/admin/billing/promotions/{id}", h.DeletePromotion)
	})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminPricing_DeletePromotion_NotFound(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	mock.ExpectExec(`UPDATE pricing_promotions SET active = FALSE`).
		WithArgs("promo_missing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	rr := runWithChi(http.MethodDelete, "/v1/admin/billing/promotions/promo_missing", nil, func(r chi.Router) {
		r.Delete("/v1/admin/billing/promotions/{id}", h.DeletePromotion)
	})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- Invalidator wiring ----

// countingInvalidator 记录 Invalidate 被调用的次数,用来断言 admin 写操作
// 完成后 cache invalidation 真的被触发了(否则会出现最长 5min stale 价格窗口)。
type countingInvalidator struct{ n int }

func (c *countingInvalidator) Invalidate() { c.n++ }

func TestAdminPricing_UpdatePricingItem_TriggersInvalidate(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	inv := &countingInvalidator{}
	h = h.WithInvalidator(inv)

	mock.ExpectExec(`UPDATE pricing_items`).
		WithArgs("plan", "pro", i64Ptr(8800), nilStrPtr(), nilStrAny()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery(`SELECT kind, item_key, price_cents, currency, updated_at, updated_by`).
		WithArgs("plan", "pro").
		WillReturnRows(pgxmock.NewRows([]string{"kind", "item_key", "price_cents", "currency", "updated_at", "updated_by"}).
			AddRow("plan", "pro", int64(8800), "CNY", time.Now(), nil))

	body, _ := json.Marshal(map[string]any{"price_cents": 8800})
	rr := runWithChi(http.MethodPatch, "/v1/admin/billing/pricing-items/plan/pro", body, func(r chi.Router) {
		r.Patch("/v1/admin/billing/pricing-items/{kind}/{item_key}", h.UpdatePricingItem)
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, 1, inv.n, "cache invalidate must fire after successful update")
}

func TestAdminPricing_DeletePromotion_TriggersInvalidate(t *testing.T) {
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	inv := &countingInvalidator{}
	h = h.WithInvalidator(inv)

	mock.ExpectExec(`UPDATE pricing_promotions SET active = FALSE`).
		WithArgs("promo_x").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	rr := runWithChi(http.MethodDelete, "/v1/admin/billing/promotions/promo_x", nil, func(r chi.Router) {
		r.Delete("/v1/admin/billing/promotions/{id}", h.DeletePromotion)
	})
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 1, inv.n, "cache invalidate must fire after delete")
}

func TestAdminPricing_NotFoundDoesNotInvalidate(t *testing.T) {
	// 404 时 cache 没改动,不该走 invalidate。
	h, mock := newAdminPricingTestHandler(t)
	defer mock.Close()
	inv := &countingInvalidator{}
	h = h.WithInvalidator(inv)

	mock.ExpectExec(`UPDATE pricing_promotions SET active = FALSE`).
		WithArgs("promo_ghost").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	rr := runWithChi(http.MethodDelete, "/v1/admin/billing/promotions/promo_ghost", nil, func(r chi.Router) {
		r.Delete("/v1/admin/billing/promotions/{id}", h.DeletePromotion)
	})
	require.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, 0, inv.n, "404 must not invalidate cache")
}

// ---- helpers ----

func decodeData(t *testing.T, raw []byte, into any) error {
	t.Helper()
	// API returns {"data": <...>, "request_id": "..."}; unwrap data field.
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return err
	}
	if len(wrap.Data) == 0 {
		// 直接是裸数据（CreatePromotion 用 map[string]string 不带 data 包装）
		return errors.New("no data field — body=" + strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(wrap.Data, into)
}

func strPtrLocal(s string) *string { return &s }
func i64Ptr(i int64) *int64        { return &i }
func boolPtr(b bool) *bool         { return &b }
func nilIntPtr() *int              { return nil }
func nilStrPtr() *string           { return nil }
func nilStrAny() any               { return nil }
func nilTimeAny() any              { return nil }
