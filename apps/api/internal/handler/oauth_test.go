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
		resp := map[string]interface{}{
			"code": 0,
			"msg":  "success",
			"data": map[string]string{"access_token": "fs_tok"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
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
