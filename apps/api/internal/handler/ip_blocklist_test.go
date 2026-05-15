package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockBlocklistStore implements BlocklistStore for testing.
type mockBlocklistStore struct {
	addedMembers   []string
	removedMembers []string
	err            error
}

func (m *mockBlocklistStore) SAdd(_ context.Context, _ string, members ...string) error {
	if m.err != nil {
		return m.err
	}
	m.addedMembers = append(m.addedMembers, members...)
	return nil
}

func (m *mockBlocklistStore) SRem(_ context.Context, _ string, members ...string) error {
	if m.err != nil {
		return m.err
	}
	m.removedMembers = append(m.removedMembers, members...)
	return nil
}

func newBlocklistRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/internal/admin/block-ip", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestBlockIP_Success(t *testing.T) {
	store := &mockBlocklistStore{}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": "1.2.3.4"})
	req := newBlocklistRequest(http.MethodPost, string(body))
	rr := httptest.NewRecorder()

	h.BlockIP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(store.addedMembers) != 1 || store.addedMembers[0] != "1.2.3.4" {
		t.Errorf("expected IP to be added to store, got %v", store.addedMembers)
	}
}

func TestBlockIP_InvalidIP_Returns400(t *testing.T) {
	store := &mockBlocklistStore{}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": "not-an-ip"})
	req := newBlocklistRequest(http.MethodPost, string(body))
	rr := httptest.NewRecorder()

	h.BlockIP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestBlockIP_EmptyIP_Returns400(t *testing.T) {
	store := &mockBlocklistStore{}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": ""})
	req := newBlocklistRequest(http.MethodPost, string(body))
	rr := httptest.NewRecorder()

	h.BlockIP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestBlockIP_StoreError_Returns500(t *testing.T) {
	store := &mockBlocklistStore{err: errors.New("redis down")}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": "1.2.3.4"})
	req := newBlocklistRequest(http.MethodPost, string(body))
	rr := httptest.NewRecorder()

	h.BlockIP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestUnblockIP_Success(t *testing.T) {
	store := &mockBlocklistStore{}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": "5.6.7.8"})
	req := newBlocklistRequest(http.MethodDelete, string(body))
	rr := httptest.NewRecorder()

	h.UnblockIP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(store.removedMembers) != 1 || store.removedMembers[0] != "5.6.7.8" {
		t.Errorf("expected IP to be removed from store, got %v", store.removedMembers)
	}
}

func TestUnblockIP_InvalidIP_Returns400(t *testing.T) {
	store := &mockBlocklistStore{}
	h := NewAdminBlocklistHandler(store)

	body, _ := json.Marshal(map[string]string{"ip": "bad-ip"})
	req := newBlocklistRequest(http.MethodDelete, string(body))
	rr := httptest.NewRecorder()

	h.UnblockIP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
