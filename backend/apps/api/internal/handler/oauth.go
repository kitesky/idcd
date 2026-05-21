package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"crypto/rand"
	"encoding/hex"

	"github.com/jackc/pgx/v5"
	"github.com/kite365/idcd/apps/api/internal/response"
	dbtx "github.com/kite365/idcd/lib/db"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	sharedi18n "github.com/kite365/idcd/lib/shared/i18n"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	oauthStateTTL = 5 * time.Minute
	oauthStateLen = 16

	dingtalkAuthBase = "https://login.dingtalk.com/oauth2/auth"
	dingtalkTokenDef = "https://api.dingtalk.com/v1.0/oauth2/userAccessToken"
	dingtalkUserDef  = "https://api.dingtalk.com/v1.0/contact/users/me"

	feishuAuthBase  = "https://accounts.feishu.cn/open-apis/authen/v1/authorize"
	feishuTokenDef  = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	feishuUserDef   = "https://open.feishu.cn/open-apis/authen/v1/user_info"

	providerDingTalk = "dingtalk"
	providerFeishu   = "feishu"
)

// OAuthStateStore stores and validates short-lived CSRF state tokens.
// GetDel atomically reads and removes a key — prevents TOCTOU replay on concurrent callbacks.
type OAuthStateStore interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
	GetDel(ctx context.Context, key string) (string, error)
}

// OAuthQuerier is the subset of DB queries used by OAuthHandler.
type OAuthQuerier interface {
	GetUserCredentialByTypeAndExternal(ctx context.Context, arg idcdmain.GetUserCredentialByTypeAndExternalParams) (idcdmain.UserCredential, error)
	CreateUserCredential(ctx context.Context, arg idcdmain.CreateUserCredentialParams) (idcdmain.UserCredential, error)
	CreateUser(ctx context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error)
	GetUserByID(ctx context.Context, id string) (idcdmain.User, error)
}

// OAuthQuerierFactory returns an OAuthQuerier bound to the given pgx.Tx.
// Production wires this to func(tx pgx.Tx) OAuthQuerier { return idcdmain.New(tx) }
// so each transactional handler sees a tx-scoped sqlc Queries.
type OAuthQuerierFactory func(tx pgx.Tx) OAuthQuerier

// OAuthHandler handles DingTalk and Feishu OAuth flows.
type OAuthHandler struct {
	q              OAuthQuerier
	jwtSvc         JWTSigner
	sessSvc        SessionStorer
	stateStore     OAuthStateStore
	dingTalkAppID  string
	dingTalkSecret string
	feishuAppID    string
	feishuSecret   string
	callbackBase   string

	dingtalkTokenURL string
	dingtalkUserURL  string
	feishuTokenURL   string
	feishuUserURL    string

	// Tx wiring for findOrCreateOAuthUser (P1-10).
	// When both fields are set, new-user creation wraps CreateUser +
	// CreateUserCredential in db.WithTxBeginner so a partial write can never
	// persist. Both nil = legacy non-transactional path (kept so existing
	// unit tests that don't wire a pool keep passing).
	txPool   dbtx.TxBeginner
	qFactory OAuthQuerierFactory
}

// OAuthConfig carries the third-party app credentials needed by OAuthHandler.
type OAuthConfig struct {
	DingTalkAppID  string
	DingTalkSecret string
	FeishuAppID    string
	FeishuSecret   string
	CallbackBase   string
}

// NewOAuthHandler creates an OAuthHandler wired to the given services.
func NewOAuthHandler(cfg OAuthConfig, q OAuthQuerier, jwtSvc JWTSigner, sessSvc SessionStorer, stateStore OAuthStateStore) *OAuthHandler {
	return &OAuthHandler{
		q:              q,
		jwtSvc:         jwtSvc,
		sessSvc:        sessSvc,
		stateStore:     stateStore,
		dingTalkAppID:  cfg.DingTalkAppID,
		dingTalkSecret: cfg.DingTalkSecret,
		feishuAppID:    cfg.FeishuAppID,
		feishuSecret:   cfg.FeishuSecret,
		callbackBase:   cfg.CallbackBase,

		dingtalkTokenURL: dingtalkTokenDef,
		dingtalkUserURL:  dingtalkUserDef,
		feishuTokenURL:   feishuTokenDef,
		feishuUserURL:    feishuUserDef,
	}
}

// WithTxPool wires the transaction pool + sqlc Queries factory used by
// findOrCreateOAuthUser so CreateUser + CreateUserCredential commit
// atomically (or both roll back). See ARCHITECTURE-REVIEW-2026-05-21 P1-10.
func (h *OAuthHandler) WithTxPool(pool dbtx.TxBeginner, factory OAuthQuerierFactory) *OAuthHandler {
	h.txPool = pool
	h.qFactory = factory
	return h
}

// --- DingTalk ---

// DingTalkLogin handles GET /v1/auth/dingtalk — redirects to DingTalk authorization page.
func (h *OAuthHandler) DingTalkLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate state", err))
		return
	}

	key := oauthStateKey(providerDingTalk, state)
	if err := h.stateStore.Set(r.Context(), key, "1", oauthStateTTL); err != nil {
		response.Error(w, r, apperr.Internal("failed to store state", err))
		return
	}

	redirectURI := h.callbackBase + "/v1/auth/dingtalk/callback"
	authURL := fmt.Sprintf(
		"%s?response_type=code&client_id=%s&redirect_uri=%s&scope=openid&prompt=consent&state=%s",
		dingtalkAuthBase,
		url.QueryEscape(h.dingTalkAppID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// DingTalkCallback handles GET /v1/auth/dingtalk/callback.
func (h *OAuthHandler) DingTalkCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		response.Error(w, r, apperr.Validation("missing code or state", ""))
		return
	}

	key := oauthStateKey(providerDingTalk, state)
	if _, err := h.stateStore.GetDel(r.Context(), key); err != nil {
		response.Error(w, r, apperr.Validation("invalid or expired state", ""))
		return
	}

	accessToken, err := h.exchangeDingTalkToken(r.Context(), code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to exchange token", err))
		return
	}

	userInfo, err := h.fetchDingTalkUser(r.Context(), accessToken)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user info", err))
		return
	}

	userID, locale, err := h.findOrCreateOAuthUser(r.Context(), r, providerDingTalk, userInfo.openID, userInfo.name, userInfo.email)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to provision user", err))
		return
	}

	token, err := h.issueOAuthToken(r.Context(), userID, locale)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	setAuthCookie(w, r, token)
	// Do NOT echo the JWT back in the redirect URL: it would land in
	// access logs / Referer headers / browser history. setAuthCookie already
	// installed the HttpOnly cookie the frontend needs.
	http.Redirect(w, r, "/auth/oauth-callback", http.StatusFound)
}

type oauthUserInfo struct {
	name   string
	openID string
	email  string
}

func (h *OAuthHandler) exchangeDingTalkToken(ctx context.Context, code string) (string, error) {
	body := map[string]string{
		"clientId":     h.dingTalkAppID,
		"clientSecret": h.dingTalkSecret,
		"code":         code,
		"grantType":    "authorization_code",
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.dingtalkTokenURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dingtalk token exchange: status %d body %s", resp.StatusCode, raw)
	}

	var result struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("dingtalk returned empty access token")
	}
	return result.AccessToken, nil
}

func (h *OAuthHandler) fetchDingTalkUser(ctx context.Context, accessToken string) (*oauthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.dingtalkUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dingtalk user info: status %d body %s", resp.StatusCode, raw)
	}

	var result struct {
		Nick   string `json:"nick"`
		OpenID string `json:"openId"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result.OpenID == "" {
		return nil, fmt.Errorf("dingtalk returned empty openId")
	}
	return &oauthUserInfo{name: result.Nick, openID: result.OpenID, email: result.Email}, nil
}

// --- Feishu ---

// FeishuLogin handles GET /v1/auth/feishu — redirects to Feishu authorization page.
func (h *OAuthHandler) FeishuLogin(w http.ResponseWriter, r *http.Request) {
	state, err := generateOAuthState()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate state", err))
		return
	}

	key := oauthStateKey(providerFeishu, state)
	if err := h.stateStore.Set(r.Context(), key, "1", oauthStateTTL); err != nil {
		response.Error(w, r, apperr.Internal("failed to store state", err))
		return
	}

	redirectURI := h.callbackBase + "/v1/auth/feishu/callback"
	authURL := fmt.Sprintf(
		"%s?app_id=%s&redirect_uri=%s&scope=contact:user.base:readonly&state=%s",
		feishuAuthBase,
		url.QueryEscape(h.feishuAppID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// FeishuCallback handles GET /v1/auth/feishu/callback.
func (h *OAuthHandler) FeishuCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		response.Error(w, r, apperr.Validation("missing code or state", ""))
		return
	}

	key := oauthStateKey(providerFeishu, state)
	if _, err := h.stateStore.GetDel(r.Context(), key); err != nil {
		response.Error(w, r, apperr.Validation("invalid or expired state", ""))
		return
	}

	accessToken, err := h.exchangeFeishuToken(r.Context(), code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to exchange token", err))
		return
	}

	userInfo, err := h.fetchFeishuUser(r.Context(), accessToken)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user info", err))
		return
	}

	userID, locale, err := h.findOrCreateOAuthUser(r.Context(), r, providerFeishu, userInfo.openID, userInfo.name, userInfo.email)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to provision user", err))
		return
	}

	token, err := h.issueOAuthToken(r.Context(), userID, locale)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	setAuthCookie(w, r, token)
	// Cookie carries the JWT; never put it in the redirect URL.
	http.Redirect(w, r, "/auth/oauth-callback", http.StatusFound)
}

func (h *OAuthHandler) exchangeFeishuToken(ctx context.Context, code string) (string, error) {
	body := map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.feishuTokenURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+h.feishuAppID+":"+h.feishuSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("feishu token exchange: status %d body %s", resp.StatusCode, raw)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu token exchange error: code %d msg %s", result.Code, result.Msg)
	}
	if result.Data.AccessToken == "" {
		return "", fmt.Errorf("feishu returned empty access token")
	}
	return result.Data.AccessToken, nil
}

func (h *OAuthHandler) fetchFeishuUser(ctx context.Context, accessToken string) (*oauthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.feishuUserURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feishu user info: status %d body %s", resp.StatusCode, raw)
	}

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Name   string `json:"name"`
			OpenID string `json:"open_id"`
			Email  string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("feishu user info error: code %d msg %s", result.Code, result.Msg)
	}
	if result.Data.OpenID == "" {
		return nil, fmt.Errorf("feishu returned empty open_id")
	}
	return &oauthUserInfo{name: result.Data.Name, openID: result.Data.OpenID, email: result.Data.Email}, nil
}

// --- shared helpers ---

// findOrCreateOAuthUser returns the user id and their persisted short locale
// code. Newly provisioned users get a locale negotiated from the inbound
// Accept-Language header; existing users keep whatever locale is on file.
//
// Tx contract (P1-10): the new-user path does CreateUser + CreateUserCredential
// inside db.WithTxBeginner so a partial write (user without credential) can
// never persist. When txPool is unwired (some unit tests), we fall back to the
// legacy non-transactional path.
func (h *OAuthHandler) findOrCreateOAuthUser(ctx context.Context, r *http.Request, provider, externalID, name, email string) (userID, locale string, err error) {
	extID := externalID
	cred, err := h.q.GetUserCredentialByTypeAndExternal(ctx, idcdmain.GetUserCredentialByTypeAndExternalParams{
		Type:       provider,
		ExternalID: &extID,
	})
	if err == nil {
		existing, err := h.q.GetUserByID(ctx, cred.UserID)
		if err != nil {
			return "", "", fmt.Errorf("load oauth user: %w", err)
		}
		return existing.ID, existing.Locale, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("lookup credential: %w", err)
	}

	newID := idgen.New("usr")
	finalEmail := email
	if finalEmail == "" {
		finalEmail = newID + "@oauth.placeholder"
	}
	var namePtr *string
	if name != "" {
		namePtr = &name
	}

	provisionedLocale := sharedi18n.MustDefault().Negotiate(r.Header.Get("Accept-Language"))

	var createdUser idcdmain.User

	doWrites := func(q OAuthQuerier) error {
		user, err := q.CreateUser(ctx, idcdmain.CreateUserParams{
			ID:          newID,
			Email:       finalEmail,
			DisplayName: namePtr,
			Locale:      provisionedLocale,
			Timezone:    "Asia/Shanghai",
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		credID := idgen.New("cred")
		if _, err := q.CreateUserCredential(ctx, idcdmain.CreateUserCredentialParams{
			ID:         credID,
			UserID:     user.ID,
			Type:       provider,
			ExternalID: &extID,
			Metadata:   []byte("{}"),
		}); err != nil {
			return fmt.Errorf("create credential: %w", err)
		}

		createdUser = user
		return nil
	}

	var txErr error
	if h.txPool != nil && h.qFactory != nil {
		txErr = dbtx.WithTxBeginner(ctx, h.txPool, func(tx pgx.Tx) error {
			return doWrites(h.qFactory(tx))
		})
	} else {
		txErr = doWrites(h.q)
	}

	if txErr != nil {
		return "", "", txErr
	}

	return createdUser.ID, createdUser.Locale, nil
}

func (h *OAuthHandler) issueOAuthToken(ctx context.Context, userID, locale string) (string, error) {
	sessionID := idgen.New("sess")
	if err := h.sessSvc.Store(ctx, sessionID, userID, sessionTTL); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	token, err := h.jwtSvc.SignWithLocale(userID, sessionID, locale, accessTokenTTL)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return token, nil
}

func generateOAuthState() (string, error) {
	b := make([]byte, oauthStateLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func oauthStateKey(provider, state string) string {
	return "oauth:state:" + provider + ":" + state
}
