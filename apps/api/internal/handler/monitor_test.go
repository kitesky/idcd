package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// --- mock querier ---

type mockMonitorQuerier struct {
	getByID        func(ctx context.Context, id string) (idcdmain.Monitor, error)
	listByUser     func(ctx context.Context, arg idcdmain.ListMonitorsByUserParams) ([]idcdmain.Monitor, error)
	createMonitor  func(ctx context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error)
	updateStatus   func(ctx context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error)
	updateFields   func(ctx context.Context, arg idcdmain.UpdateMonitorFieldsParams) (idcdmain.Monitor, error)
	deleteMonitor  func(ctx context.Context, id string) error
}

func (m *mockMonitorQuerier) GetMonitorByID(ctx context.Context, id string) (idcdmain.Monitor, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	return idcdmain.Monitor{}, errors.New("not found")
}

func (m *mockMonitorQuerier) ListMonitorsByUser(ctx context.Context, arg idcdmain.ListMonitorsByUserParams) ([]idcdmain.Monitor, error) {
	if m.listByUser != nil {
		return m.listByUser(ctx, arg)
	}
	return nil, nil
}

func (m *mockMonitorQuerier) CreateMonitor(ctx context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
	if m.createMonitor != nil {
		return m.createMonitor(ctx, arg)
	}
	return idcdmain.Monitor{}, errors.New("not implemented")
}

func (m *mockMonitorQuerier) UpdateMonitorStatus(ctx context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error) {
	if m.updateStatus != nil {
		return m.updateStatus(ctx, arg)
	}
	return idcdmain.Monitor{}, errors.New("not implemented")
}

func (m *mockMonitorQuerier) UpdateMonitorFields(ctx context.Context, arg idcdmain.UpdateMonitorFieldsParams) (idcdmain.Monitor, error) {
	if m.updateFields != nil {
		return m.updateFields(ctx, arg)
	}
	return idcdmain.Monitor{}, errors.New("not implemented")
}

func (m *mockMonitorQuerier) DeleteMonitor(ctx context.Context, id string) error {
	if m.deleteMonitor != nil {
		return m.deleteMonitor(ctx, id)
	}
	return nil
}

// --- helpers ---

func fakeMonitor(id, userID string) idcdmain.Monitor {
	return idcdmain.Monitor{
		ID:        id,
		UserID:    userID,
		Name:      "Test Monitor",
		Type:      "http",
		Target:    "example.com",
		Config:    []byte("{}"),
		IntervalS: 300,
		NodeCount: 3,
		Status:    "active",
		CreatedAt: pgtype.Timestamptz{Valid: false},
		UpdatedAt: pgtype.Timestamptz{Valid: false},
	}
}

// injectUserID adds a user_id to the request context (simulates authn middleware).
func injectUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

// --- tests ---

func TestMonitorHandler_Create_success(t *testing.T) {
	mock := &mockMonitorQuerier{
		createMonitor: func(_ context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{
				ID:        arg.ID,
				UserID:    arg.UserID,
				Name:      arg.Name,
				Type:      arg.Type,
				Target:    arg.Target,
				Config:    arg.Config,
				IntervalS: arg.IntervalS,
				NodeCount: arg.NodeCount,
				Status:    "active",
			}, nil
		},
	}
	h := NewMonitorHandler(mock)

	body, _ := json.Marshal(map[string]any{
		"name":       "Test Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 300,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_testuser")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMonitorHandler_Create_noAuth(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader([]byte(`{"name":"x","type":"http","target":"example.com"}`)))
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMonitorHandler_Create_invalidType(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	body, _ := json.Marshal(map[string]any{"name": "x", "type": "invalid", "target": "example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_testuser")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMonitorHandler_Create_invalidTarget(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	body, _ := json.Marshal(map[string]any{"name": "x", "type": "http", "target": "169.254.169.254"})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_testuser")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMonitorHandler_Create_invalidInterval(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	body, _ := json.Marshal(map[string]any{"name": "x", "type": "http", "target": "example.com", "interval_s": 999})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_testuser")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMonitorHandler_Create_badJSON(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader([]byte(`{bad json}`)))
	req = injectUserID(req, "u_testuser")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMonitorHandler_List_success(t *testing.T) {
	mock := &mockMonitorQuerier{
		listByUser: func(_ context.Context, arg idcdmain.ListMonitorsByUserParams) ([]idcdmain.Monitor, error) {
			return []idcdmain.Monitor{fakeMonitor("mon_001", arg.UserID)}, nil
		},
	}
	h := NewMonitorHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/v1/monitors", nil)
	req = injectUserID(req, "u_testuser")
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMonitorHandler_List_noAuth(t *testing.T) {
	h := NewMonitorHandler(&mockMonitorQuerier{})
	req := httptest.NewRequest(http.MethodGet, "/v1/monitors", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMonitorHandler_Get_success(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, userID), nil
		},
	}
	h := NewMonitorHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/v1/monitors/mon_001", nil)
	req = injectUserID(req, userID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMonitorHandler_Get_ownership(t *testing.T) {
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, "u_otheruser"), nil
		},
	}
	h := NewMonitorHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/v1/monitors/mon_001", nil)
	req = injectUserID(req, "u_testuser")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestMonitorHandler_Delete_success(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, userID), nil
		},
		deleteMonitor: func(_ context.Context, id string) error {
			return nil
		},
	}
	h := NewMonitorHandler(mock)
	req := httptest.NewRequest(http.MethodDelete, "/v1/monitors/mon_001", nil)
	req = injectUserID(req, userID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMonitorHandler_Pause_success(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, userID), nil
		},
		updateStatus: func(_ context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error) {
			m := fakeMonitor(arg.ID, userID)
			m.Status = arg.Status
			return m, nil
		},
	}
	h := NewMonitorHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors/mon_001/pause", nil)
	req = injectUserID(req, userID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Pause(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if data, ok := resp["data"].(map[string]any); ok {
		if data["status"] != "paused" {
			t.Errorf("expected status=paused, got %v", data["status"])
		}
	}
}

func TestMonitorHandler_Resume_success(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			m := fakeMonitor(id, userID)
			m.Status = "paused"
			return m, nil
		},
		updateStatus: func(_ context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error) {
			m := fakeMonitor(arg.ID, userID)
			m.Status = arg.Status
			return m, nil
		},
	}
	h := NewMonitorHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors/mon_001/resume", nil)
	req = injectUserID(req, userID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Resume(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMonitorHandler_Update_statusOnly(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, userID), nil
		},
		updateStatus: func(_ context.Context, arg idcdmain.UpdateMonitorStatusParams) (idcdmain.Monitor, error) {
			m := fakeMonitor(arg.ID, userID)
			m.Status = arg.Status
			return m, nil
		},
	}
	h := NewMonitorHandler(mock)
	body, _ := json.Marshal(map[string]any{"status": "paused"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/monitors/mon_001", bytes.NewReader(body))
	req = injectUserID(req, userID)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMonitorHandler_Update_invalidStatus(t *testing.T) {
	const userID = "u_testuser"
	mock := &mockMonitorQuerier{
		getByID: func(_ context.Context, id string) (idcdmain.Monitor, error) {
			return fakeMonitor(id, userID), nil
		},
	}
	h := NewMonitorHandler(mock)
	body, _ := json.Marshal(map[string]any{"status": "archived"}) // archived not allowed via PATCH
	req := httptest.NewRequest(http.MethodPatch, "/v1/monitors/mon_001", bytes.NewReader(body))
	req = injectUserID(req, userID)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "mon_001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		query     string
		wantPage  int
		wantLimit int
	}{
		{"", 1, 20},
		{"page=2&limit=10", 2, 10},
		{"page=0&limit=200", 1, 20},  // page 0 → default 1; limit 200 > 100 → default 20
		{"page=abc", 1, 20},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		page, limit := parsePagination(req)
		if page != tc.wantPage || limit != tc.wantLimit {
			t.Errorf("query=%q: got page=%d limit=%d, want page=%d limit=%d",
				tc.query, page, limit, tc.wantPage, tc.wantLimit)
		}
	}
}

func TestMonitorToResponse(t *testing.T) {
	m := fakeMonitor("mon_001", "u_testuser")
	resp := monitorToResponse(m)
	if resp.ID != "mon_001" {
		t.Errorf("ID mismatch")
	}
	if resp.Status != "active" {
		t.Errorf("Status mismatch")
	}
	// Config should marshal as valid JSON
	var config map[string]any
	if err := json.Unmarshal(resp.Config, &config); err != nil {
		t.Errorf("Config is not valid JSON: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Quota enforcement tests
// ─────────────────────────────────────────────────────────────────────────────

// mockQuotaRow is a fake pgx.Row that scans a single pre-configured value.
type mockQuotaRow struct {
	val interface{}
	err error
}

func (m *mockQuotaRow) Scan(dest ...interface{}) error {
	if m.err != nil {
		return m.err
	}
	if len(dest) > 0 {
		switch d := dest[0].(type) {
		case *string:
			if v, ok := m.val.(string); ok {
				*d = v
			}
		case *int:
			if v, ok := m.val.(int); ok {
				*d = v
			}
		}
	}
	return nil
}

// mockQuotaPool returns configurable responses for QueryRow calls.
// The first call returns planRow, all subsequent calls return countRow.
// It satisfies the QuotaPool interface (returns pgx.Row and no-ops Exec).
type mockQuotaPool struct {
	planRow  *mockQuotaRow
	countRow *mockQuotaRow
	calls    int
}

func (m *mockQuotaPool) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	m.calls++
	if m.calls == 1 {
		return m.planRow
	}
	return m.countRow
}


// freePlanPoolWith returns a mockQuotaPool set to "free" plan with given monitor count.
func freePlanPoolWith(count int) *mockQuotaPool {
	return &mockQuotaPool{
		planRow:  &mockQuotaRow{val: "free"},
		countRow: &mockQuotaRow{val: count},
	}
}

func proPlanPoolWith(count int) *mockQuotaPool {
	return &mockQuotaPool{
		planRow:  &mockQuotaRow{val: "pro"},
		countRow: &mockQuotaRow{val: count},
	}
}

// TestMonitorHandler_Create_QuotaExceeded_MonitorCount tests that a free user
// at 3 monitors receives HTTP 402.
func TestMonitorHandler_Create_QuotaExceeded_MonitorCount(t *testing.T) {
	mock := &mockMonitorQuerier{}
	pool := freePlanPoolWith(3) // free limit is 3; at limit → 402

	h := NewMonitorHandler(mock).WithQuotaPool(pool)

	body, _ := json.Marshal(map[string]any{
		"name":       "Fourth Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 300,
		"node_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_free")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402, got %d: %s", rec.Code, rec.Body.String())
	}

	// Response body should contain "quota_exceeded"
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["error"] != "quota_exceeded" {
		t.Errorf(`expected error="quota_exceeded", got %v`, resp["error"])
	}
	if resp["upgrade_url"] != "/app/billing" {
		t.Errorf(`expected upgrade_url="/app/billing", got %v`, resp["upgrade_url"])
	}
}

// TestMonitorHandler_Create_QuotaOK_FreeUnderLimit tests that a free user with
// 2 monitors can still create a 3rd.
func TestMonitorHandler_Create_QuotaOK_FreeUnderLimit(t *testing.T) {
	mock := &mockMonitorQuerier{
		createMonitor: func(_ context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{
				ID:        arg.ID,
				UserID:    arg.UserID,
				Name:      arg.Name,
				Type:      arg.Type,
				Target:    arg.Target,
				Config:    arg.Config,
				IntervalS: arg.IntervalS,
				NodeCount: arg.NodeCount,
				Status:    "active",
			}, nil
		},
	}
	pool := freePlanPoolWith(2) // free limit is 3; 2 < 3 → OK

	h := NewMonitorHandler(mock).WithQuotaPool(pool)

	body, _ := json.Marshal(map[string]any{
		"name":       "Third Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 300,
		"node_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_free")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMonitorHandler_Create_QuotaExceeded_Interval tests that a free user
// requesting a 60s interval (below 300s minimum) receives HTTP 402.
func TestMonitorHandler_Create_QuotaExceeded_Interval(t *testing.T) {
	mock := &mockMonitorQuerier{}
	pool := freePlanPoolWith(0)

	h := NewMonitorHandler(mock).WithQuotaPool(pool)

	body, _ := json.Marshal(map[string]any{
		"name":       "Fast Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 60, // valid interval, but below free plan minimum (300s)
		"node_count": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_free")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402 for interval below free plan minimum, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMonitorHandler_Create_QuotaExceeded_NodeCount tests that a free user
// requesting 2 nodes (above free plan max of 1) receives HTTP 402.
func TestMonitorHandler_Create_QuotaExceeded_NodeCount(t *testing.T) {
	mock := &mockMonitorQuerier{}
	pool := freePlanPoolWith(0)

	h := NewMonitorHandler(mock).WithQuotaPool(pool)

	body, _ := json.Marshal(map[string]any{
		"name":       "Multi-Node Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 300,
		"node_count": 2, // free plan max is 1
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_free")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402 for node count above free plan max, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMonitorHandler_Create_NoQuotaPool_Succeeds tests that when no quota pool
// is configured (nil), quota checks are skipped and creation proceeds normally.
func TestMonitorHandler_Create_NoQuotaPool_Succeeds(t *testing.T) {
	mock := &mockMonitorQuerier{
		createMonitor: func(_ context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{
				ID:     arg.ID,
				UserID: arg.UserID,
				Name:   arg.Name,
				Type:   arg.Type,
				Target: arg.Target,
				Config: arg.Config,
				Status: "active",
			}, nil
		},
	}

	// No pool — quota checks are skipped (defaults to free/0).
	h := NewMonitorHandler(mock)

	body, _ := json.Marshal(map[string]any{
		"name":       "Monitor Without Quota",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 300,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_free")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestMonitorHandler_Create_ProPlan_60sInterval tests that a pro user can use
// a 60s interval.
func TestMonitorHandler_Create_ProPlan_60sInterval(t *testing.T) {
	mock := &mockMonitorQuerier{
		createMonitor: func(_ context.Context, arg idcdmain.CreateMonitorParams) (idcdmain.Monitor, error) {
			return idcdmain.Monitor{
				ID:        arg.ID,
				UserID:    arg.UserID,
				Name:      arg.Name,
				Type:      arg.Type,
				Target:    arg.Target,
				Config:    arg.Config,
				IntervalS: arg.IntervalS,
				NodeCount: arg.NodeCount,
				Status:    "active",
			}, nil
		},
	}
	pool := proPlanPoolWith(0)
	h := NewMonitorHandler(mock).WithQuotaPool(pool)

	body, _ := json.Marshal(map[string]any{
		"name":       "Fast Pro Monitor",
		"type":       "http",
		"target":     "example.com",
		"interval_s": 60,
		"node_count": 3, // within pro limit of 5
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/monitors", bytes.NewReader(body))
	req = injectUserID(req, "u_pro")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 for pro plan with 60s interval, got %d: %s", rec.Code, rec.Body.String())
	}
}
