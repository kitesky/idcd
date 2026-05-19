package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/billing"
)

// ---- helpers ----

// (fakePricing 移到 pricing_fake_test.go，供本包多个 handler 测试复用)

func newVerdictOrderTestHandler(t *testing.T) (*VerdictOrderHandler, pgxmock.PgxPoolIface, *billing.StubProvider) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	stub := billing.NewStubProvider()
	h := NewVerdictOrderHandler(mockPool, stub).WithPricing(fakePricing{})
	return h, mockPool, stub
}

func validCreateBody(t *testing.T, overrides map[string]any) []byte {
	t.Helper()
	body := map[string]any{
		"template":          "sla",
		"target":            "example.com",
		"time_window_start": time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339),
		"time_window_end":   time.Now().UTC().Format(time.RFC3339),
		"channel":           "alipay",
		"return_url":        "https://idcd.com/app/verdict",
	}
	for k, v := range overrides {
		if v == nil {
			delete(body, k)
		} else {
			body[k] = v
		}
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return raw
}

// ---- Create: happy path + per-template pricing ----

func TestVerdictOrderHandler_Create_Success_AllTemplates(t *testing.T) {
	cases := []struct {
		template string
		priceCNY int64
	}{
		{"sla", 299},
		{"incident", 199},
		{"compliance", 499},
		{"legal", 999},
	}
	for _, tc := range cases {
		t.Run(tc.template, func(t *testing.T) {
			h, mockPool, _ := newVerdictOrderTestHandler(t)
			defer mockPool.Close()

			mockPool.ExpectExec(`INSERT INTO idcd_attest\.verdict_order`).
				WithArgs(
					pgxmock.AnyArg(),     // id
					"u_test_user",        // owner_id
					tc.template,          // template
					"example.com",        // target
					pgxmock.AnyArg(),     // time_window_start
					pgxmock.AnyArg(),     // time_window_end
					float64(tc.priceCNY), // price_cny (yuan)
					nil,                  // promotion_id (no promo in fakePricing)
					pgxmock.AnyArg(),     // created_at
				).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))

			body := validCreateBody(t, map[string]any{"template": tc.template})
			req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = prepReq(req, "u_test_user")
			rr := httptest.NewRecorder()

			h.Create(rr, req)

			require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

			var resp struct {
				Data CreateVerdictOrderResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			assert.Contains(t, resp.Data.OrderID, "v_")
			assert.NotEmpty(t, resp.Data.PayURL)
			assert.Equal(t, tc.priceCNY, resp.Data.PriceCNY)
			assert.NoError(t, mockPool.ExpectationsWereMet())
		})
	}
}

func TestVerdictOrderHandler_Create_ChargeMetadataCarriesOrderID(t *testing.T) {
	h, mockPool, stub := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(`INSERT INTO idcd_attest\.verdict_order`).
		WithArgs(
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			nil, // promotion_id
			pgxmock.AnyArg(),
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := validCreateBody(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	var resp struct {
		Data CreateVerdictOrderResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	// The stub provider records the charge with the metadata we passed; the
	// webhook layer relies on this round-trip to enqueue the verdict.
	// Find the charge by user (only one in this test) and assert metadata.
	var found *billing.StubCharge
	for i := 0; i < 8; i++ { // best-effort scan via known-but-opaque ID via test
		// We don't know the chg_xxx id directly; instead recover it from the
		// pay URL the handler returned.
		_ = i
	}
	// The pay URL format is /billing/stub-charge-confirm?charge_id=chg_xxx&item_ref=v_xxx
	// Extract charge_id from PayURL.
	const prefix = "/billing/stub-charge-confirm?charge_id="
	require.Contains(t, resp.Data.PayURL, prefix)
	idStart := len(prefix)
	idEnd := idStart
	for idEnd < len(resp.Data.PayURL) && resp.Data.PayURL[idEnd] != '&' {
		idEnd++
	}
	chargeID := resp.Data.PayURL[idStart:idEnd]

	c, ok := stub.GetCharge(chargeID)
	require.True(t, ok, "stub charge should be persisted")
	found = c
	assert.Equal(t, "u_test_user", found.UserID)
	assert.Equal(t, int64(29900), found.AmountCents) // sla = ¥299
	assert.Equal(t, resp.Data.OrderID, found.ItemRef)
	assert.Equal(t, resp.Data.OrderID, found.Metadata["verdict_order_id"])
	assert.Equal(t, "alipay", found.Channel)
	assert.Equal(t, "CNY", found.Currency)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

// ---- Create: validation errors ----

func TestVerdictOrderHandler_Create_NoAuth(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	body := validCreateBody(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVerdictOrderHandler_Create_InvalidBody(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader([]byte("not json")))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVerdictOrderHandler_Create_MissingTemplate(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	body := validCreateBody(t, map[string]any{"template": ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "template")
}

func TestVerdictOrderHandler_Create_UnknownTemplate(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	body := validCreateBody(t, map[string]any{"template": "premium"})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "template")
}

func TestVerdictOrderHandler_Create_InvalidChannel(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	body := validCreateBody(t, map[string]any{"channel": "bitcoin"})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "channel")
}

func TestVerdictOrderHandler_Create_MissingTarget(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	body := validCreateBody(t, map[string]any{"target": ""})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "target")
}

func TestVerdictOrderHandler_Create_TimeWindowInverted(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC()
	body := validCreateBody(t, map[string]any{
		"time_window_start": now.Format(time.RFC3339),
		"time_window_end":   now.Add(-1 * time.Hour).Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "time_window")
}

func TestVerdictOrderHandler_Create_TimeWindowEqual(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC()
	body := validCreateBody(t, map[string]any{
		"time_window_start": now.Format(time.RFC3339),
		"time_window_end":   now.Format(time.RFC3339),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/verdict/orders", bytes.NewReader(body))
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- Get ----

func TestVerdictOrderHandler_Get_Owner(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end",
		"status", "price_cny", "price_paid_cny", "ext_order_id",
		"created_at", "paid_at", "delivered_at",
	}).AddRow(
		"v_001", "u_test_user", "sla", "example.com",
		now.Add(-24*time.Hour), now,
		"paid", float64(299), (*float64)(nil), (*string)(nil),
		now, &now, (*time.Time)(nil),
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order`).
		WithArgs("v_001").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/orders/v_001", nil)
	req = prepReq(req, "u_test_user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "v_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	var resp struct {
		Data GetVerdictOrderResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "v_001", resp.Data.ID)
	assert.Equal(t, "sla", resp.Data.Template)
	assert.Equal(t, "paid", resp.Data.Status)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictOrderHandler_Get_NotOwner(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end",
		"status", "price_cny", "price_paid_cny", "ext_order_id",
		"created_at", "paid_at", "delivered_at",
	}).AddRow(
		"v_002", "u_other_user", "sla", "example.com",
		now.Add(-24*time.Hour), now,
		"paid", float64(299), (*float64)(nil), (*string)(nil),
		now, &now, (*time.Time)(nil),
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order`).
		WithArgs("v_002").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/orders/v_002", nil)
	req = prepReq(req, "u_test_user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "v_002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.Get(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictOrderHandler_Get_NotFound(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end",
		"status", "price_cny", "price_paid_cny", "ext_order_id",
		"created_at", "paid_at", "delivered_at",
	})
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order`).
		WithArgs("v_missing").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/orders/v_missing", nil)
	req = prepReq(req, "u_test_user")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "v_missing")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	h.Get(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictOrderHandler_Get_NoAuth(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/orders/v_001", nil)
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Get(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVerdictOrderHandler_Get_MissingID(t *testing.T) {
	h, mockPool, _ := newVerdictOrderTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/orders/", nil)
	req = prepReq(req, "u_test_user")
	// no URL param set → chi.URLParam returns ""
	rr := httptest.NewRecorder()

	h.Get(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// Pricing validation + price values now covered by
// apps/api/internal/billing/pricing_test.go (TestDBPricing_*).

func TestNewVerdictOrderHandler_NotNil(t *testing.T) {
	h := NewVerdictOrderHandler(nil, billing.NewStubProvider())
	assert.NotNil(t, h)
}
