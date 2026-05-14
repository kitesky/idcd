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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

// ─────────────────────────────────────────────
// Mock pool
// ─────────────────────────────────────────────

type mockPATPool struct {
	tokens map[string]*mockPATRow
	err    error
}

type mockPATRow struct {
	id          string
	userID      string
	name        string
	tokenHash   string
	tokenPrefix string
	scopes      []string
	expiresAt   *time.Time
	createdAt   time.Time
}

func newMockPATPool() *mockPATPool {
	return &mockPATPool{tokens: make(map[string]*mockPATRow)}
}

func (m *mockPATPool) QueryRow(_ context.Context, sql string, args ...interface{}) pgx.Row {
	if m.err != nil {
		return &patErrRow{err: m.err}
	}
	// INSERT ... RETURNING
	if strings.Contains(sql, "INSERT INTO personal_access_tokens") {
		id := args[0].(string)
		userID := args[1].(string)
		name := args[2].(string)
		tokenHash := args[3].(string)
		tokenPrefix := args[4].(string)
		scopes := args[5].([]string)
		var expiresAt *time.Time
		if args[6] != nil {
			switch v := args[6].(type) {
			case time.Time:
				expiresAt = &v
			case *time.Time:
				expiresAt = v
			}
		}
		now := time.Now().UTC()
		row := &mockPATRow{
			id:          id,
			userID:      userID,
			name:        name,
			tokenHash:   tokenHash,
			tokenPrefix: tokenPrefix,
			scopes:      scopes,
			expiresAt:   expiresAt,
			createdAt:   now,
		}
		m.tokens[id] = row
		return &patQueryRow{row: row}
	}
	// SELECT user_id FROM personal_access_tokens WHERE id = $1
	if strings.Contains(sql, "SELECT user_id") {
		id := args[0].(string)
		r, ok := m.tokens[id]
		if !ok {
			return &patErrRow{err: pgx.ErrNoRows}
		}
		return &singleStringRow{val: r.userID}
	}
	return &patErrRow{err: fmt.Errorf("unexpected QueryRow sql: %s", sql)}
}

func (m *mockPATPool) Query(_ context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if m.err != nil {
		return nil, m.err
	}
	if strings.Contains(sql, "SELECT id, name, token_prefix") {
		userID := args[0].(string)
		var rows []*mockPATRow
		for _, r := range m.tokens {
			if r.userID == userID {
				rows = append(rows, r)
			}
		}
		return &patRows{rows: rows, idx: -1}, nil
	}
	return nil, fmt.Errorf("unexpected Query sql: %s", sql)
}

func (m *mockPATPool) Exec(_ context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	if m.err != nil {
		return pgconn.NewCommandTag("DELETE 0"), m.err
	}
	if strings.Contains(sql, "DELETE FROM personal_access_tokens") {
		id := args[0].(string)
		if _, ok := m.tokens[id]; !ok {
			return pgconn.NewCommandTag("DELETE 0"), nil
		}
		delete(m.tokens, id)
		return pgconn.NewCommandTag("DELETE 1"), nil
	}
	return pgconn.NewCommandTag(""), fmt.Errorf("unexpected Exec sql: %s", sql)
}

// ── Helper row types ──────────────────────────────────────────────────────────

type patErrRow struct{ err error }

func (e *patErrRow) Scan(...interface{}) error { return e.err }

type singleStringRow struct{ val string }

func (s *singleStringRow) Scan(dest ...interface{}) error {
	if len(dest) < 1 {
		return fmt.Errorf("expected 1 dest")
	}
	*(dest[0].(*string)) = s.val
	return nil
}

type patQueryRow struct{ row *mockPATRow }

func (p *patQueryRow) Scan(dest ...interface{}) error {
	if len(dest) < 6 {
		return fmt.Errorf("expected 6 dest, got %d", len(dest))
	}
	*(dest[0].(*string)) = p.row.id
	*(dest[1].(*string)) = p.row.name
	*(dest[2].(*string)) = p.row.tokenPrefix
	*(dest[3].(*[]string)) = p.row.scopes
	*(dest[4].(**time.Time)) = p.row.expiresAt
	*(dest[5].(*time.Time)) = p.row.createdAt
	return nil
}

type patRows struct {
	rows []*mockPATRow
	idx  int
}

func (r *patRows) Next() bool {
	r.idx++
	return r.idx < len(r.rows)
}

func (r *patRows) Scan(dest ...interface{}) error {
	row := r.rows[r.idx]
	if len(dest) < 6 {
		return fmt.Errorf("expected 6 dest, got %d", len(dest))
	}
	*(dest[0].(*string)) = row.id
	*(dest[1].(*string)) = row.name
	*(dest[2].(*string)) = row.tokenPrefix
	*(dest[3].(*[]string)) = row.scopes
	*(dest[4].(**time.Time)) = row.expiresAt
	*(dest[5].(*time.Time)) = row.createdAt
	return nil
}

func (r *patRows) Close()            {}
func (r *patRows) Err() error        { return nil }
func (r *patRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *patRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *patRows) Values() ([]interface{}, error) { return nil, nil }
func (r *patRows) RawValues() [][]byte            { return nil }
func (r *patRows) Conn() *pgx.Conn               { return nil }

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func withUserIDForPAT(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), userID)
	return req.WithContext(ctx)
}

// ─────────────────────────────────────────────
// Tests — Create
// ─────────────────────────────────────────────

func TestPATCreate_success(t *testing.T) {
	pool := newMockPATPool()
	h := NewPATHandler(pool)

	body := `{"name":"My CLI Token","scopes":["read:monitors"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/account/tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data patCreateResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(wrapped.Data.Token, patTokenPrefix) {
		t.Errorf("token should start with %q, got %q", patTokenPrefix, wrapped.Data.Token)
	}
	if wrapped.Data.Name != "My CLI Token" {
		t.Errorf("unexpected name: %q", wrapped.Data.Name)
	}
	if wrapped.Data.ID == "" {
		t.Error("expected non-empty ID")
	}
	if wrapped.Data.TokenPrefix == "" {
		t.Error("expected non-empty token_prefix")
	}
}

func TestPATCreate_noAuth(t *testing.T) {
	h := NewPATHandler(newMockPATPool())

	req := httptest.NewRequest(http.MethodPost, "/v1/account/tokens", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestPATCreate_missingName(t *testing.T) {
	h := NewPATHandler(newMockPATPool())

	req := httptest.NewRequest(http.MethodPost, "/v1/account/tokens", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPATCreate_withExpiry(t *testing.T) {
	pool := newMockPATPool()
	h := NewPATHandler(pool)

	days := 365
	bodyObj := map[string]interface{}{
		"name":         "Expiring Token",
		"scopes":       []string{"read:monitors"},
		"expires_days": days,
	}
	bodyBytes, _ := json.Marshal(bodyObj)
	req := httptest.NewRequest(http.MethodPost, "/v1/account/tokens", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data patCreateResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.ExpiresAt == nil {
		t.Error("expected non-nil expires_at")
	}
}

// ─────────────────────────────────────────────
// Tests — List
// ─────────────────────────────────────────────

func TestPATList_success(t *testing.T) {
	pool := newMockPATPool()
	now := time.Now().UTC()
	pool.tokens["pat_001"] = &mockPATRow{
		id:          "pat_001",
		userID:      "usr_001",
		name:        "CLI",
		tokenPrefix: patTokenPrefix + "abcd1234",
		scopes:      []string{"read:monitors"},
		createdAt:   now,
	}
	pool.tokens["pat_002"] = &mockPATRow{
		id:          "pat_002",
		userID:      "usr_001",
		name:        "MCP",
		tokenPrefix: patTokenPrefix + "ef567890",
		scopes:      []string{},
		createdAt:   now,
	}
	pool.tokens["pat_003"] = &mockPATRow{
		id:          "pat_003",
		userID:      "usr_002",
		name:        "Other",
		tokenPrefix: patTokenPrefix + "xxxxxxxx",
		scopes:      []string{},
		createdAt:   now,
	}

	h := NewPATHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/tokens", nil)
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var wrapped struct {
		Data []patResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrapped.Data) != 2 {
		t.Errorf("expected 2 tokens for usr_001, got %d", len(wrapped.Data))
	}
	// Verify list response does not contain "token" key
	rawBody := rr.Body.String()
	if strings.Contains(rawBody, `"token":"idcd_pat_`) {
		t.Error("list response should not contain full token value")
	}
	for _, tok := range wrapped.Data {
		if tok.TokenPrefix == "" {
			t.Error("token_prefix should not be empty")
		}
	}
}

func TestPATList_noAuth(t *testing.T) {
	h := NewPATHandler(newMockPATPool())

	req := httptest.NewRequest(http.MethodGet, "/v1/account/tokens", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Tests — Delete
// ─────────────────────────────────────────────

func TestPATDelete_success(t *testing.T) {
	pool := newMockPATPool()
	pool.tokens["pat_abc"] = &mockPATRow{
		id:     "pat_abc",
		userID: "usr_001",
		name:   "to delete",
	}
	h := NewPATHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/tokens/pat_abc", nil)
	req = withUserIDForPAT(req, "usr_001")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "pat_abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if _, ok := pool.tokens["pat_abc"]; ok {
		t.Error("token should have been deleted")
	}
}

func TestPATDelete_noAuth(t *testing.T) {
	h := NewPATHandler(newMockPATPool())

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/tokens/pat_abc", nil)
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestPATDelete_wrongOwner(t *testing.T) {
	pool := newMockPATPool()
	pool.tokens["pat_abc"] = &mockPATRow{
		id:     "pat_abc",
		userID: "usr_001",
		name:   "someone else's token",
	}
	h := NewPATHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/tokens/pat_abc", nil)
	req = withUserIDForPAT(req, "usr_002")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "pat_abc")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForPAT(req, "usr_002")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPATDelete_notFound(t *testing.T) {
	h := NewPATHandler(newMockPATPool())

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/tokens/nonexistent", nil)
	req = withUserIDForPAT(req, "usr_001")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = withUserIDForPAT(req, "usr_001")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// ─────────────────────────────────────────────
// Tests — generatePAT helper
// ─────────────────────────────────────────────

func TestGeneratePAT_format(t *testing.T) {
	fullToken, tokenPrefix, tokenHash, err := generatePAT()
	if err != nil {
		t.Fatalf("generatePAT error: %v", err)
	}
	if !strings.HasPrefix(fullToken, patTokenPrefix) {
		t.Errorf("fullToken should start with %q, got %q", patTokenPrefix, fullToken)
	}
	if !strings.HasPrefix(tokenPrefix, patTokenPrefix) {
		t.Errorf("tokenPrefix should start with %q, got %q", patTokenPrefix, tokenPrefix)
	}
	if tokenHash == "" {
		t.Error("hash should not be empty")
	}
	if len(tokenPrefix) != len(patTokenPrefix)+patDisplayPrefixN {
		t.Errorf("tokenPrefix length should be %d, got %d", len(patTokenPrefix)+patDisplayPrefixN, len(tokenPrefix))
	}
}

func TestGeneratePAT_uniqueness(t *testing.T) {
	tok1, _, _, _ := generatePAT()
	tok2, _, _, _ := generatePAT()
	if tok1 == tok2 {
		t.Error("two generated tokens should not be equal")
	}
}
