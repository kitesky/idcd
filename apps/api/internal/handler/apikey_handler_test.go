package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/db/repository"
)

// ─────────────────────────────────────────────
// Mock
// ─────────────────────────────────────────────

type mockAPIKeyQuerier struct {
	keys   map[string]idcdmain.ApiKey
	nextID int
	err    error // if non-nil, all write ops return this error
}

func newMockAPIKeyQuerier() *mockAPIKeyQuerier {
	return &mockAPIKeyQuerier{
		keys: make(map[string]idcdmain.ApiKey),
	}
}

func (m *mockAPIKeyQuerier) CreateAPIKey(_ context.Context, arg idcdmain.CreateAPIKeyParams) (idcdmain.ApiKey, error) {
	if m.err != nil {
		return idcdmain.ApiKey{}, m.err
	}
	now := pgtype.Timestamptz{}
	_ = now.Scan(time.Now())
	k := idcdmain.ApiKey{
		ID:        arg.ID,
		OwnerType: arg.OwnerType,
		OwnerID:   arg.OwnerID,
		Name:      arg.Name,
		Prefix:    arg.Prefix,
		SecretHash: arg.SecretHash,
		Scopes:    arg.Scopes,
		Status:    "active",
		CreatedBy: arg.CreatedBy,
		CreatedAt: now,
	}
	m.keys[arg.ID] = k
	return k, nil
}

func (m *mockAPIKeyQuerier) ListAPIKeysByOwner(_ context.Context, arg idcdmain.ListAPIKeysByOwnerParams) ([]idcdmain.ApiKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []idcdmain.ApiKey
	for _, k := range m.keys {
		if k.OwnerType == arg.OwnerType && k.OwnerID == arg.OwnerID && k.Status == "active" {
			result = append(result, k)
		}
	}
	return result, nil
}

func (m *mockAPIKeyQuerier) GetAPIKeyByID(_ context.Context, id string) (idcdmain.ApiKey, error) {
	k, ok := m.keys[id]
	if !ok || k.Status != "active" {
		return idcdmain.ApiKey{}, repository.ErrNotFound
	}
	return k, nil
}

func (m *mockAPIKeyQuerier) RevokeAPIKey(_ context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	k, ok := m.keys[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	k.Status = "revoked"
	m.keys[id] = k
	return nil
}

// withUserIDForAPIKey injects a user ID into a request context (same helper pattern as account_test).
func withUserIDForAPIKey(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), userID)
	return req.WithContext(ctx)
}

// seedKey adds a pre-existing active key for a user.
func (m *mockAPIKeyQuerier) seedKey(userID, keyID, name string) {
	now := pgtype.Timestamptz{}
	_ = now.Scan(time.Now())
	m.keys[keyID] = idcdmain.ApiKey{
		ID:        keyID,
		OwnerType: "user",
		OwnerID:   userID,
		Name:      name,
		Prefix:    "abcd1234",
		SecretHash: "fakehash",
		Scopes:    []string{"read", "write"},
		Status:    "active",
		CreatedBy: userID,
		CreatedAt: now,
	}
}

// ─────────────────────────────────────────────
// Tests — CreateAPIKey
// ─────────────────────────────────────────────

func TestCreateAPIKey_success(t *testing.T) {
	q := newMockAPIKeyQuerier()
	h := NewAPIKeyHandler(q)

	body := `{"name":"test key"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/account/api-keys", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForAPIKey(req, "usr_001")
	rr := httptest.NewRecorder()

	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data apiKeyCreateResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if wrapped.Data.Name != "test key" {
		t.Errorf("expected name %q, got %q", "test key", wrapped.Data.Name)
	}
	if !strings.HasPrefix(wrapped.Data.Key, apiKeyPrefix) {
		t.Errorf("key should start with %q, got %q", apiKeyPrefix, wrapped.Data.Key)
	}
	if wrapped.Data.ID == "" {
		t.Error("expected non-empty key ID")
	}
}

func TestCreateAPIKey_noAuth(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodPost, "/v1/account/api-keys", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestCreateAPIKey_missingName(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodPost, "/v1/account/api-keys", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForAPIKey(req, "usr_001")
	rr := httptest.NewRecorder()

	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateAPIKey_invalidJSON(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodPost, "/v1/account/api-keys", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForAPIKey(req, "usr_001")
	rr := httptest.NewRecorder()

	h.CreateAPIKey(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Tests — ListAPIKeys
// ─────────────────────────────────────────────

func TestListAPIKeys_success(t *testing.T) {
	q := newMockAPIKeyQuerier()
	q.seedKey("usr_001", "key_001", "key one")
	q.seedKey("usr_001", "key_002", "key two")
	q.seedKey("usr_002", "key_003", "other user key") // should not appear
	h := NewAPIKeyHandler(q)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/api-keys", nil)
	req = withUserIDForAPIKey(req, "usr_001")
	rr := httptest.NewRecorder()

	h.ListAPIKeys(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data []apiKeyResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(wrapped.Data) != 2 {
		t.Errorf("expected 2 keys, got %d", len(wrapped.Data))
	}
}

func TestListAPIKeys_noAuth(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodGet, "/v1/account/api-keys", nil)
	rr := httptest.NewRecorder()

	h.ListAPIKeys(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestListAPIKeys_empty(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodGet, "/v1/account/api-keys", nil)
	req = withUserIDForAPIKey(req, "usr_empty")
	rr := httptest.NewRecorder()

	h.ListAPIKeys(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var wrapped struct {
		Data []apiKeyResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrapped.Data) != 0 {
		t.Errorf("expected 0 keys, got %d", len(wrapped.Data))
	}
}

// ─────────────────────────────────────────────
// Tests — RevokeAPIKey
// ─────────────────────────────────────────────

func TestRevokeAPIKey_success(t *testing.T) {
	q := newMockAPIKeyQuerier()
	q.seedKey("usr_001", "key_abc", "my key")
	h := NewAPIKeyHandler(q)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/api-keys/key_abc", nil)
	req = withUserIDForAPIKey(req, "usr_001")

	// Inject chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "key_abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForAPIKey(req, "usr_001")

	rr := httptest.NewRecorder()
	h.RevokeAPIKey(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// Verify key is revoked.
	if q.keys["key_abc"].Status != "revoked" {
		t.Errorf("expected key status 'revoked', got %q", q.keys["key_abc"].Status)
	}
}

func TestRevokeAPIKey_noAuth(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/api-keys/key_abc", nil)
	rr := httptest.NewRecorder()
	h.RevokeAPIKey(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRevokeAPIKey_notFound(t *testing.T) {
	h := NewAPIKeyHandler(newMockAPIKeyQuerier())

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/api-keys/nonexistent", nil)
	req = withUserIDForAPIKey(req, "usr_001")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForAPIKey(req, "usr_001")

	rr := httptest.NewRecorder()
	h.RevokeAPIKey(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestRevokeAPIKey_wrongOwner(t *testing.T) {
	q := newMockAPIKeyQuerier()
	q.seedKey("usr_001", "key_abc", "my key")
	h := NewAPIKeyHandler(q)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/api-keys/key_abc", nil)
	req = withUserIDForAPIKey(req, "usr_002") // different user

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "key_abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForAPIKey(req, "usr_002")

	rr := httptest.NewRecorder()
	h.RevokeAPIKey(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Tests — generateAPIKey helper
// ─────────────────────────────────────────────

func TestGenerateAPIKey_format(t *testing.T) {
	fullKey, prefix, hash, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generateAPIKey error: %v", err)
	}
	if !strings.HasPrefix(fullKey, apiKeyPrefix) {
		t.Errorf("fullKey should start with %q, got %q", apiKeyPrefix, fullKey)
	}
	if len(prefix) != apiKeyPrefixLen {
		t.Errorf("prefix length should be %d, got %d", apiKeyPrefixLen, len(prefix))
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	// Full key should contain the prefix after sk_live_.
	if !strings.HasPrefix(fullKey[len(apiKeyPrefix):], prefix) {
		t.Errorf("fullKey[%d:] should start with prefix %q", len(apiKeyPrefix), prefix)
	}
}

func TestGenerateAPIKey_uniqueness(t *testing.T) {
	key1, _, _, _ := generateAPIKey()
	key2, _, _, _ := generateAPIKey()
	if key1 == key2 {
		t.Error("two generated keys should not be equal")
	}
}
