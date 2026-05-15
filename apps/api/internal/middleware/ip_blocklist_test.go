package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockBlocklistStore implements IPBlocklistStore for testing.
type mockBlocklistStore struct {
	members map[string]bool
	err     error
}

func (m *mockBlocklistStore) SIsMember(_ context.Context, _, member string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.members[member], nil
}

func passHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestIPBlocklist_NilStore_PassesThrough(t *testing.T) {
	mw := IPBlocklist(nil)
	h := mw(passHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestIPBlocklist_BlockedIP_Returns403(t *testing.T) {
	store := &mockBlocklistStore{
		members: map[string]bool{
			"203.0.113.1": true,
		},
	}
	mw := IPBlocklist(store)
	h := mw(passHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestIPBlocklist_AllowedIP_PassesThrough(t *testing.T) {
	store := &mockBlocklistStore{
		members: map[string]bool{
			"203.0.113.2": true,
		},
	}
	mw := IPBlocklist(store)
	h := mw(passHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.5:5678" // not in blocklist
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestIPBlocklist_StoreError_PassesThrough(t *testing.T) {
	store := &mockBlocklistStore{
		err: errors.New("redis connection lost"),
	}
	mw := IPBlocklist(store)
	h := mw(passHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1:1234"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (fail-open), got %d", rr.Code)
	}
}
