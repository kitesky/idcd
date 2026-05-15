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
	"github.com/jackc/pgx/v5/pgconn"
)

type mockStatusSubPool struct {
	queryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	exec     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockStatusSubPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRow != nil {
		return m.queryRow(ctx, sql, args...)
	}
	return &errRow{err: pgx.ErrNoRows}
}

func (m *mockStatusSubPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.query != nil {
		return m.query(ctx, sql, args...)
	}
	return &emptyRows{}, nil
}

func (m *mockStatusSubPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.exec != nil {
		return m.exec(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func newSubRouter(pool StatusSubPool) http.Handler {
	h := NewStatusSubscriptionHandler(pool)
	r := chi.NewRouter()
	r.Post("/{slug}/subscribe", h.Subscribe)
	r.Get("/{slug}/verify", h.Verify)
	r.Delete("/{slug}/unsubscribe", h.Unsubscribe)
	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	}).Get("/{slug}/subscriptions", h.List)
	r.Delete("/{slug}/subscriptions/{id}", h.Delete)
	return r
}

func TestStatusSubscription_Subscribe_Email_201_UnverifiedFalse(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &scanRow{values: []any{"sp_test001"}}
		},
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	body := `{"channel_type":"email","endpoint":"user@example.com","events":["incident","recovery"]}`
	req := httptest.NewRequest(http.MethodPost, "/my-page/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["verified"] != false {
		t.Errorf("email subscription should be unverified, got verified=%v", data["verified"])
	}
	if data["channel_type"] != "email" {
		t.Errorf("expected channel_type=email, got %v", data["channel_type"])
	}
}

func TestStatusSubscription_Subscribe_Webhook_201_VerifiedTrue(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &scanRow{values: []any{"sp_test002"}}
		},
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	body := `{"channel_type":"webhook","endpoint":"https://example.com/hook"}`
	req := httptest.NewRequest(http.MethodPost, "/my-page/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["verified"] != true {
		t.Errorf("webhook subscription should be verified, got verified=%v", data["verified"])
	}
}

func TestStatusSubscription_Subscribe_StatusPageNotFound_404(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &errRow{err: pgx.ErrNoRows}
		},
	}

	body := `{"channel_type":"email","endpoint":"user@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/nonexistent/subscribe", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestStatusSubscription_Verify_ValidToken_200_VerifiedTrue(t *testing.T) {
	scanCalls := 0
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			scanCalls++
			if scanCalls == 1 {
				return &scanRow{values: []any{"ssub_abc123"}}
			}
			return &errRow{err: errors.New("unexpected call")}
		},
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/my-page/verify?token=validtoken123", nil)
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["verified"] != true {
		t.Errorf("expected verified=true, got %v", data["verified"])
	}
}

func TestStatusSubscription_Verify_InvalidToken_404(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &errRow{err: pgx.ErrNoRows}
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/my-page/verify?token=badtoken", nil)
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestStatusSubscription_Unsubscribe_ValidToken_200(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &scanRow{values: []any{"ssub_abc456"}}
		},
		exec: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/my-page/unsubscribe?token=validtoken456", nil)
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestStatusSubscription_Unsubscribe_InvalidToken_404(t *testing.T) {
	pool := &mockStatusSubPool{
		queryRow: func(_ context.Context, _ string, args ...any) pgx.Row {
			return &errRow{err: pgx.ErrNoRows}
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/my-page/unsubscribe?token=bad", nil)
	rr := httptest.NewRecorder()
	newSubRouter(pool).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

