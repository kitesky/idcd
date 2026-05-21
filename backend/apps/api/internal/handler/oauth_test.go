package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// --- mocks ---

type mockOAuthQuerier struct {
	creds map[string]idcdmain.UserCredential // keyed by "type:externalID"
	users map[string]idcdmain.User           // keyed by id
}

func newMockOAuthQuerier() *mockOAuthQuerier {
	return &mockOAuthQuerier{
		creds: make(map[string]idcdmain.UserCredential),
		users: make(map[string]idcdmain.User),
	}
}

func (m *mockOAuthQuerier) GetUserCredentialByTypeAndExternal(_ context.Context, arg idcdmain.GetUserCredentialByTypeAndExternalParams) (idcdmain.UserCredential, error) {
	key := arg.Type + ":" + safeDeref(arg.ExternalID)
	c, ok := m.creds[key]
	if !ok {
		return idcdmain.UserCredential{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *mockOAuthQuerier) CreateUserCredential(_ context.Context, arg idcdmain.CreateUserCredentialParams) (idcdmain.UserCredential, error) {
	key := arg.Type + ":" + safeDeref(arg.ExternalID)
	c := idcdmain.UserCredential{
		ID:         arg.ID,
		UserID:     arg.UserID,
		Type:       arg.Type,
		ExternalID: arg.ExternalID,
		Metadata:   arg.Metadata,
	}
	m.creds[key] = c
	return c, nil
}

func (m *mockOAuthQuerier) CreateUser(_ context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error) {
	u := idcdmain.User{
		ID:          arg.ID,
		Email:       arg.Email,
		DisplayName: arg.DisplayName,
		Locale:      arg.Locale,
		Timezone:    arg.Timezone,
		Status:      "active",
	}
	m.users[arg.ID] = u
	return u, nil
}

func (m *mockOAuthQuerier) GetUserByID(_ context.Context, id string) (idcdmain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return idcdmain.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type mockStateStore struct {
	data map[string]string
	err  error
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{data: make(map[string]string)}
}

func (m *mockStateStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.data[key] = value
	return nil
}

func (m *mockStateStore) Get(_ context.Context, key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (m *mockStateStore) Del(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockStateStore) GetDel(_ context.Context, key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	v, ok := m.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	delete(m.data, key)
	return v, nil
}

func newTestOAuthHandler(q *mockOAuthQuerier, stateStore OAuthStateStore, dingtalkSrv, feishuSrv *httptest.Server) *OAuthHandler {
	cfg := OAuthConfig{
		DingTalkAppID:  "test_dt_app",
		DingTalkSecret: "test_dt_secret",
		FeishuAppID:    "test_fs_app",
		FeishuSecret:   "test_fs_secret",
		CallbackBase:   "http://localhost:8080",
	}
	h := NewOAuthHandler(cfg, q, &mockJWT{token: "tok.en.here"}, &mockSession{}, stateStore)
	if dingtalkSrv != nil {
		h.dingtalkTokenURL = dingtalkSrv.URL + "/token"
		h.dingtalkUserURL = dingtalkSrv.URL + "/user"
	}
	if feishuSrv != nil {
		h.feishuTokenURL = feishuSrv.URL + "/token"
		h.feishuUserURL = feishuSrv.URL + "/user"
	}
	return h
}

// --- DingTalk tests ---

func TestDingTalkLogin_redirects(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/dingtalk", nil)
	rr := httptest.NewRecorder()
	h.DingTalkLogin(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "login.dingtalk.com") {
		t.Errorf("expected DingTalk auth URL, got %q", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("expected state param in URL, got %q", loc)
	}
}

func TestDingTalkCallback_invalidState(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/dingtalk/callback?code=abc&state=badstate", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid state, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDingTalkCallback_missingParams(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/dingtalk/callback", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing params, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestDingTalkCallback_validCode_newUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"accessToken":"dt_access_tok"}`)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"openId":"dt_open_001","nick":"钉钉用户","email":"user@corp.com"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stateStore := newMockStateStore()
	q := newMockOAuthQuerier()
	h := newTestOAuthHandler(q, stateStore, srv, nil)

	stateKey := oauthStateKey(providerDingTalk, "validstate")
	_ = stateStore.Set(context.Background(), stateKey, "1", oauthStateTTL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/dingtalk/callback?code=authcode&state=validstate", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "/auth/oauth-callback") {
		t.Errorf("expected /auth/oauth-callback redirect, got %q", loc)
	}

	if _, err := stateStore.Get(context.Background(), stateKey); err == nil {
		t.Error("expected state to be deleted after callback")
	}
}

func TestDingTalkCallback_validCode_existingUser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"accessToken":"dt_tok"}`)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"openId":"dt_existing_001","nick":"已有用户","email":"existing@corp.com"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stateStore := newMockStateStore()
	q := newMockOAuthQuerier()

	extID := "dt_existing_001"
	q.creds["dingtalk:dt_existing_001"] = idcdmain.UserCredential{
		ID:         "cred_01",
		UserID:     "usr_existing",
		Type:       "dingtalk",
		ExternalID: &extID,
	}
	q.users["usr_existing"] = idcdmain.User{ID: "usr_existing", Email: "existing@corp.com", Status: "active"}

	h := newTestOAuthHandler(q, stateStore, srv, nil)

	stateKey := oauthStateKey(providerDingTalk, "st2")
	_ = stateStore.Set(context.Background(), stateKey, "1", oauthStateTTL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/dingtalk/callback?code=code2&state=st2", nil)
	rr := httptest.NewRecorder()
	h.DingTalkCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- Feishu tests ---

func TestFeishuLogin_redirects(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/feishu", nil)
	rr := httptest.NewRecorder()
	h.FeishuLogin(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "feishu.cn") {
		t.Errorf("expected Feishu auth URL, got %q", loc)
	}
}

func TestFeishuCallback_invalidState(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/feishu/callback?code=abc&state=invalid", nil)
	rr := httptest.NewRecorder()
	h.FeishuCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid state, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFeishuCallback_validCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]string{"access_token": "fs_tok"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"code": 0,
			"msg":  "success",
			"data": map[string]string{
				"name":    "飞书用户",
				"open_id": "fs_open_001",
				"email":   "user@feishu.com",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	stateStore := newMockStateStore()
	h := newTestOAuthHandler(newMockOAuthQuerier(), stateStore, nil, srv)

	stateKey := oauthStateKey(providerFeishu, "fs_state")
	_ = stateStore.Set(context.Background(), stateKey, "1", oauthStateTTL)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/feishu/callback?code=fscode&state=fs_state", nil)
	rr := httptest.NewRecorder()
	h.FeishuCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", rr.Code, rr.Body.String())
	}
	loc := rr.Header().Get("Location")
	if !strings.Contains(loc, "/auth/oauth-callback") {
		t.Errorf("expected /auth/oauth-callback, got %q", loc)
	}
}

func TestFeishuCallback_missingParams(t *testing.T) {
	h := newTestOAuthHandler(newMockOAuthQuerier(), newMockStateStore(), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/feishu/callback?code=x", nil)
	rr := httptest.NewRecorder()
	h.FeishuCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing state, got %d", rr.Code)
	}
}

// --- Transaction tests ---

// failCredentialQuerier wraps mockOAuthQuerier but fails on CreateUserCredential.
type failCredentialQuerier struct {
	*mockOAuthQuerier
}

func (f *failCredentialQuerier) CreateUserCredential(_ context.Context, _ idcdmain.CreateUserCredentialParams) (idcdmain.UserCredential, error) {
	return idcdmain.UserCredential{}, fmt.Errorf("simulated credential write failure")
}

// mockTx implements pgx.Tx for testing.
type mockOAuthTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
}

func (m *mockOAuthTx) Commit(_ context.Context) error   { m.committed = true; return nil }
func (m *mockOAuthTx) Rollback(_ context.Context) error  { m.rolledBack = true; return nil }

// mockTxBeginner implements dbtx.TxBeginner for testing.
type mockOAuthTxBeginner struct {
	tx *mockOAuthTx
}

func (m *mockOAuthTxBeginner) Begin(_ context.Context) (pgx.Tx, error) {
	m.tx = &mockOAuthTx{}
	return m.tx, nil
}

func TestFindOrCreateOAuthUser_TxRollbackOnCredentialFailure(t *testing.T) {
	baseQ := newMockOAuthQuerier()
	failQ := &failCredentialQuerier{mockOAuthQuerier: newMockOAuthQuerier()}
	txBeginner := &mockOAuthTxBeginner{}

	cfg := OAuthConfig{
		DingTalkAppID:  "app",
		DingTalkSecret: "secret",
		CallbackBase:   "http://localhost",
	}
	h := NewOAuthHandler(cfg, baseQ, &mockJWT{token: "tok"}, &mockSession{}, newMockStateStore())
	h.WithTxPool(txBeginner, func(_ pgx.Tx) OAuthQuerier {
		return failQ
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, _, err := h.findOrCreateOAuthUser(req.Context(), req, "dingtalk", "ext_001", "Test", "test@example.com")

	if err == nil {
		t.Fatal("expected error when credential creation fails, got nil")
	}
	if !strings.Contains(err.Error(), "simulated credential write failure") {
		t.Errorf("expected credential failure error, got: %v", err)
	}
	if txBeginner.tx == nil {
		t.Fatal("expected transaction to be started")
	}
	if txBeginner.tx.committed {
		t.Error("transaction should NOT have been committed after credential failure")
	}
	if !txBeginner.tx.rolledBack {
		t.Error("transaction should have been rolled back after credential failure")
	}

	if len(baseQ.users) != 0 {
		t.Error("user should not exist in base querier after tx rollback")
	}
}

func TestFindOrCreateOAuthUser_TxCommitOnSuccess(t *testing.T) {
	baseQ := newMockOAuthQuerier()
	txQ := newMockOAuthQuerier()
	txBeginner := &mockOAuthTxBeginner{}

	cfg := OAuthConfig{
		DingTalkAppID:  "app",
		DingTalkSecret: "secret",
		CallbackBase:   "http://localhost",
	}
	h := NewOAuthHandler(cfg, baseQ, &mockJWT{token: "tok"}, &mockSession{}, newMockStateStore())
	h.WithTxPool(txBeginner, func(_ pgx.Tx) OAuthQuerier {
		return txQ
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	userID, locale, err := h.findOrCreateOAuthUser(req.Context(), req, "dingtalk", "ext_002", "User2", "user2@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID == "" {
		t.Error("expected non-empty userID")
	}
	if locale == "" {
		t.Error("expected non-empty locale")
	}
	if txBeginner.tx == nil {
		t.Fatal("expected transaction to be started")
	}
	if !txBeginner.tx.committed {
		t.Error("transaction should have been committed on success")
	}

	if len(txQ.users) != 1 {
		t.Errorf("expected 1 user in tx querier, got %d", len(txQ.users))
	}
	if len(txQ.creds) != 1 {
		t.Errorf("expected 1 credential in tx querier, got %d", len(txQ.creds))
	}
}

func TestFindOrCreateOAuthUser_LegacyPathWithoutTxPool(t *testing.T) {
	baseQ := newMockOAuthQuerier()

	cfg := OAuthConfig{
		DingTalkAppID:  "app",
		DingTalkSecret: "secret",
		CallbackBase:   "http://localhost",
	}
	h := NewOAuthHandler(cfg, baseQ, &mockJWT{token: "tok"}, &mockSession{}, newMockStateStore())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	userID, _, err := h.findOrCreateOAuthUser(req.Context(), req, "feishu", "ext_003", "User3", "user3@example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID == "" {
		t.Error("expected non-empty userID")
	}
	if len(baseQ.users) != 1 {
		t.Errorf("expected 1 user in base querier (legacy path), got %d", len(baseQ.users))
	}
	if len(baseQ.creds) != 1 {
		t.Errorf("expected 1 credential in base querier (legacy path), got %d", len(baseQ.creds))
	}
}
