// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/errcode"
	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/password"
	"github.com/kite365/idcd/lib/auth/totp"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/db/repository"
	"github.com/kite365/idcd/lib/shared/aesenc"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
	sharedi18n "github.com/kite365/idcd/lib/shared/i18n"
)

const (
	accessTokenTTL = 24 * time.Hour
	sessionTTL     = 90 * 24 * time.Hour
	otpTTL         = 10 * time.Minute
	otpTypeVerify  = "email_verify"
	otpTypeReset   = "password_reset"
	otpMaxAttempts = 5
)

// AuthEnqueuer enqueues async notification tasks without exposing asynq types.
type AuthEnqueuer interface {
	// EnqueueTask enqueues taskType with JSON payload into the named queue.
	// Implementations must be safe to call concurrently.
	EnqueueTask(ctx context.Context, taskType string, payload []byte, queue string) error
}

// task type constants — must match the notifier worker's task registry.
const (
	taskSendVerifyEmail   = "task:send_verify_email"
	taskSendResetPassword = "task:send_reset_password"
)

// AuthQuerier is the subset of sqlc Queries used by AuthHandler.
type AuthQuerier interface {
	CreateUser(ctx context.Context, arg idcdmain.CreateUserParams) (idcdmain.User, error)
	GetUserByEmail(ctx context.Context, email string) (idcdmain.User, error)
	GetUserByID(ctx context.Context, id string) (idcdmain.User, error)
	UpdateUserEmailVerified(ctx context.Context, id string) (idcdmain.User, error)
	UpdateUserPasswordHash(ctx context.Context, arg idcdmain.UpdateUserPasswordHashParams) error
	UpdateUserLastLogin(ctx context.Context, arg idcdmain.UpdateUserLastLoginParams) error
	CreateUserOTP(ctx context.Context, arg idcdmain.CreateUserOTPParams) (idcdmain.UserOtp, error)
	GetUserOTPByIDAndType(ctx context.Context, arg idcdmain.GetUserOTPByIDAndTypeParams) (idcdmain.UserOtp, error)
	IncrementUserOTPAttempts(ctx context.Context, id string) error
	MarkUserOTPUsed(ctx context.Context, id string) error
	SoftDeleteUser(ctx context.Context, id string) error
	UpdateUserProfile(ctx context.Context, arg idcdmain.UpdateUserProfileParams) (idcdmain.User, error)
}

// JWTSigner signs JWT tokens. SignWithLocale embeds an optional locale claim
// ("" omits it). Sign is retained for backwards compatibility with callers
// that don't yet carry a locale (legacy code paths / tests).
type JWTSigner interface {
	Sign(userID, sessionID string, expiry time.Duration) (string, error)
	SignWithLocale(userID, sessionID, locale string, expiry time.Duration) (string, error)
}

// SessionStorer stores and deletes sessions.
type SessionStorer interface {
	Store(ctx context.Context, sessionID, userID string, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
}

// MFAPool is the minimal pgx interface needed for MFA checks.
type MFAPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// AuthHandler implements all auth and account endpoints.
type AuthHandler struct {
	q            AuthQuerier
	jwtSvc       JWTSigner
	sessSvc      SessionStorer
	otpSecret    []byte // HMAC key for OTP hashing
	referralPool ReferralPool
	mfaPool      MFAPool
	mfaRedis     *redis.Client
	fieldCipher  *aesenc.Cipher
	enqueuer     AuthEnqueuer // optional: nil disables async email dispatch
	appBaseURL   string       // e.g. "https://app.idcd.com", used to build reset links
}

// NewAuthHandler creates an AuthHandler wired to the given services.
// otpSecret must be a strong random secret (≥ 32 bytes); the JWT secret is a good choice.
func NewAuthHandler(q AuthQuerier, jwtSvc JWTSigner, sessSvc SessionStorer, otpSecret string) *AuthHandler {
	return &AuthHandler{q: q, jwtSvc: jwtSvc, sessSvc: sessSvc, otpSecret: []byte(otpSecret)}
}

// WithReferralPool wires a DB pool for referral tracking during registration.
func (h *AuthHandler) WithReferralPool(pool ReferralPool) *AuthHandler {
	h.referralPool = pool
	return h
}

// WithMFA wires a pgx pool and Redis client for MFA support.
func (h *AuthHandler) WithMFA(pool MFAPool, rdb *redis.Client) *AuthHandler {
	h.mfaPool = pool
	h.mfaRedis = rdb
	return h
}

// WithFieldCipher wires the AES-256-GCM cipher used to decrypt stored TOTP secrets.
func (h *AuthHandler) WithFieldCipher(cipher *aesenc.Cipher) *AuthHandler {
	h.fieldCipher = cipher
	return h
}

// WithEnqueuer wires the async task enqueuer for email dispatch.
// When enqueuer is nil (default), email tasks are silently skipped (fail-open).
func (h *AuthHandler) WithEnqueuer(enqueuer AuthEnqueuer) *AuthHandler {
	h.enqueuer = enqueuer
	return h
}

// WithAppBaseURL sets the frontend base URL used to construct password-reset deep-links.
// Example: "https://app.idcd.com"
func (h *AuthHandler) WithAppBaseURL(baseURL string) *AuthHandler {
	h.appBaseURL = baseURL
	return h
}

// --- Register ---

type registerRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	DisplayName  string `json:"display_name"`
	ReferralCode string `json:"referral_code"`
}

type authResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	UserID      string `json:"user_id"`
}

// writeAuthSuccess installs the access_token cookie and writes the JSON
// response. The body also carries AccessToken so non-cookie clients (CLI,
// mobile, server-to-server) can authenticate via Authorization: Bearer.
func writeAuthSuccess(w http.ResponseWriter, r *http.Request, status int, token, userID string) {
	setAuthCookie(w, r, token)
	response.JSON(w, r, status, authResponse{
		AccessToken: token,
		ExpiresIn:   int(accessTokenTTL.Seconds()),
		UserID:      userID,
	})
}

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}

	if req.Email == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, map[string]any{"field": "email"})
		return
	}
	if err := password.ValidatePassword(req.Password, req.Email); err != nil {
		response.ErrorCode(ctx, w, r, errcode.AuthPasswordWeak, map[string]any{"detail": err.Error()})
		return
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to hash password", err))
		return
	}

	userID := idgen.New("usr")
	displayName := req.DisplayName
	var displayNamePtr *string
	if displayName != "" {
		displayNamePtr = &displayName
	}

	// Infer locale from Accept-Language (i18n Phase 2c): persist the short
	// registry code so every downstream component (JWT claim, notifier email,
	// etc.) sees a canonical value. Negotiate always returns a supported code.
	locale := negotiateRegisterLocale(r, "")

	user, err := h.q.CreateUser(ctx, idcdmain.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: &hash,
		DisplayName:  displayNamePtr,
		Locale:       locale,
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			response.ErrorCode(ctx, w, r, errcode.AccountEmailTaken, map[string]any{"email": req.Email})
			return
		}
		response.Error(w, r, apperr.Internal("failed to create user", err))
		return
	}

	if req.ReferralCode != "" && h.referralPool != nil {
		h.recordReferral(ctx, req.ReferralCode, user.ID)
	}

	// Send verification email asynchronously (fail-open: does not block registration).
	if h.enqueuer != nil {
		if otpID, code, err := h.issueOTP(ctx, user.ID, otpTypeVerify, 30*time.Minute); err == nil {
			h.enqueueVerifyEmail(ctx, req.Email, otpID, code, user.Locale)
		}
	}

	token, _, err := h.issueToken(ctx, user.ID, user.Locale)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	writeAuthSuccess(w, r, http.StatusCreated, token, user.ID)
}

func (h *AuthHandler) recordReferral(ctx context.Context, code, referredID string) {
	var referrerID string
	var codeID string
	err := h.referralPool.QueryRow(ctx,
		`SELECT id, user_id FROM referral_codes WHERE code = $1`,
		code,
	).Scan(&codeID, &referrerID)
	if err != nil {
		return
	}
	_ = codeID

	// Prevent self-referral.
	if referrerID == referredID {
		return
	}

	rewardID := idgen.New("rwd_")
	// ON CONFLICT guards against duplicate rows from concurrent registrations with the same code.
	_, _ = h.referralPool.Exec(ctx, `
		INSERT INTO referral_rewards (id, referrer_id, referred_id, code, status, reward_amount)
		VALUES ($1, $2, $3, $4, 'pending', 10.00)
		ON CONFLICT (referrer_id, referred_id) DO NOTHING
	`, rewardID, referrerID, referredID, code)

	_, _ = h.referralPool.Exec(ctx,
		`UPDATE referral_codes SET uses_count = uses_count + 1 WHERE code = $1`,
		code,
	)
}

// --- Login ---

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}

	if req.Email == "" || req.Password == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, nil)
		return
	}

	user, err := h.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.ErrorCode(ctx, w, r, errcode.AuthCredentialsInvalid, nil)
			return
		}
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	if user.PasswordHash == nil || !password.Verify(req.Password, *user.PasswordHash) {
		response.ErrorCode(ctx, w, r, errcode.AuthCredentialsInvalid, nil)
		return
	}

	// users.status enum: active / locked / pending_deletion / deleted.
	// Anything other than 'active' must be rejected.
	if user.Status != "active" {
		response.ErrorCode(ctx, w, r, errcode.AuthAccountDisabled, map[string]any{"status": user.Status})
		return
	}

	if h.mfaPool != nil && h.mfaRedis != nil && h.userHas2FA(ctx, user.ID) {
		mfaToken := idgen.New("mfa")
		if err := h.mfaRedis.Set(ctx, mfaPendingKeyPrefix+mfaToken, user.ID, mfaPendingTTL).Err(); err != nil {
			response.Error(w, r, apperr.Internal("failed to create mfa session", err))
			return
		}
		response.JSON(w, r, http.StatusOK, map[string]any{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
		return
	}

	token, _, err := h.issueToken(ctx, user.ID, user.Locale)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	_ = h.q.UpdateUserLastLogin(ctx, idcdmain.UpdateUserLastLoginParams{ID: user.ID})

	writeAuthSuccess(w, r, http.StatusOK, token, user.ID)
}

// --- Logout ---

// Logout handles POST /v1/auth/logout (requires auth middleware).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID := middleware.SessionIDFromContext(r.Context())
	if sessionID != "" {
		_ = h.sessSvc.Delete(r.Context(), sessionID)
	}
	clearAuthCookie(w, r)
	response.JSON(w, r, http.StatusOK, map[string]string{"message": "logged out"})
}

// setAuthCookie issues the JWT as an HttpOnly cookie. SameSite=Lax is required
// so OAuth top-level redirects (dingtalk/feishu → idcd.com) keep the cookie;
// Strict mode silently dropped it on first navigation. Production deployments
// that split idcd.com/api.idcd.com onto different registrable domains need
// SameSite=None + Secure + CORS Allow-Credentials — see docs/ARCHITECTURE.md.
func setAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(accessTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookie expires the access_token cookie on logout.
func clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
}

// --- Verify Email ---

type verifyEmailRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
	OtpID string `json:"otp_id"`
}

// VerifyEmail handles POST /v1/auth/verify-email.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}
	if req.OtpID == "" || req.Code == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, nil)
		return
	}

	user, err := h.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		response.ErrorCode(ctx, w, r, errcode.AuthOTPInvalid, nil)
		return
	}

	if otpCode, params := h.verifyOTPCoded(ctx, user.ID, req.OtpID, req.Code, otpTypeVerify); otpCode != "" {
		response.ErrorCode(ctx, w, r, otpCode, params)
		return
	}

	if _, err := h.q.UpdateUserEmailVerified(ctx, user.ID); err != nil {
		response.Error(w, r, apperr.Internal("failed to mark email verified", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "email verified"})
}

// --- Forgot Password ---

type forgotPasswordRequest struct {
	Email string `json:"email"`
}


// ForgotPassword handles POST /v1/auth/forgot-password.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}
	if req.Email == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, map[string]any{"field": "email"})
		return
	}

	const sameMsg = "if the email exists, a reset code has been sent"

	user, err := h.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// Timing equalisation: sleep approximately as long as issueOTP would take
		// so that attackers cannot enumerate registered emails via response-time differences.
		time.Sleep(50 * time.Millisecond)
		response.JSON(w, r, http.StatusOK, map[string]string{"message": sameMsg})
		return
	}

	otpID, code, err := h.issueOTP(ctx, user.ID, otpTypeReset, otpTTL)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue OTP", err))
		return
	}

	// Dispatch password-reset email asynchronously (fail-open).
	// otp_id and code must only travel to the user via email — never returned in the API response.
	if h.enqueuer != nil {
		h.enqueueResetPasswordEmail(ctx, req.Email, otpID, code, user.Locale)
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": sameMsg})
}

// --- Resend Verify Email ---

// ResendVerifyEmail handles POST /v1/auth/resend-verify.
// Requires authentication (authnMW). Rate-limited by the existing auth rate limiter.
// If the account is already verified it returns 200 with a descriptive message.
func (h *AuthHandler) ResendVerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		response.ErrorCode(ctx, w, r, errcode.AuthRequired, nil)
		return
	}

	user, err := h.q.GetUserByID(ctx, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	if user.EmailVerifiedAt.Valid {
		response.JSON(w, r, http.StatusOK, map[string]string{"message": "email already verified"})
		return
	}

	if h.enqueuer == nil {
		// Enqueuer not wired — acknowledge but do nothing.
		response.JSON(w, r, http.StatusOK, map[string]string{"message": "verification email sent"})
		return
	}

	otpID, code, err := h.issueOTP(ctx, userID, otpTypeVerify, 30*time.Minute)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue OTP", err))
		return
	}

	h.enqueueVerifyEmail(ctx, user.Email, otpID, code, user.Locale)
	response.JSON(w, r, http.StatusOK, map[string]string{"message": "verification email sent"})
}

// --- Reset Password ---

type resetPasswordRequest struct {
	Email       string `json:"email"`
	OtpID       string `json:"otp_id"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

// ResetPassword handles POST /v1/auth/reset-password.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}
	if req.Email == "" || req.OtpID == "" || req.Code == "" || req.NewPassword == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, nil)
		return
	}

	user, err := h.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		response.ErrorCode(ctx, w, r, errcode.AuthOTPInvalid, nil)
		return
	}

	if otpCode, params := h.verifyOTPCoded(ctx, user.ID, req.OtpID, req.Code, otpTypeReset); otpCode != "" {
		response.ErrorCode(ctx, w, r, otpCode, params)
		return
	}

	if err := password.ValidatePassword(req.NewPassword, req.Email); err != nil {
		response.ErrorCode(ctx, w, r, errcode.AuthPasswordWeak, map[string]any{"detail": err.Error()})
		return
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to hash password", err))
		return
	}

	if err := h.q.UpdateUserPasswordHash(ctx, idcdmain.UpdateUserPasswordHashParams{
		ID:           user.ID,
		PasswordHash: &hash,
	}); err != nil {
		response.Error(w, r, apperr.Internal("failed to update password", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// --- Helpers ---

// enqueueVerifyEmail enqueues a send_verify_email task.
// It is fail-open: errors are silently dropped so callers are never blocked.
//
// userLocale must be the short registry code (cn/en) for the recipient — the
// notifier worker honours `payload.locale` when picking templates / subject
// strings. An empty string falls back to the registry default downstream.
func (h *AuthHandler) enqueueVerifyEmail(ctx context.Context, to, otpID, code, userLocale string) {
	loc := normalizeLocale(userLocale)
	payload, err := json.Marshal(map[string]any{
		"to":         to,
		"otp_id":     otpID,
		"code":       code,
		"expires_in": "30 minutes",
		"locale":     loc,
	})
	if err != nil {
		return
	}
	_ = h.enqueuer.EnqueueTask(ctx, taskSendVerifyEmail, payload, "email")
}

// enqueueResetPasswordEmail enqueues a send_reset_password task.
// The reset deep-link embeds otp_id and code so the frontend can pre-fill the form.
// It is fail-open: errors are silently dropped.
//
// userLocale follows the same contract as enqueueVerifyEmail's locale arg.
func (h *AuthHandler) enqueueResetPasswordEmail(ctx context.Context, to, otpID, code, userLocale string) {
	baseURL := h.appBaseURL
	if baseURL == "" {
		baseURL = "https://app.idcd.com"
	}
	loc := normalizeLocale(userLocale)
	// Note: reset link inherits the user's locale prefix downstream when the
	// notifier worker rewrites URLs via worker.BuildLocalizedURL. We still send
	// the raw URL here so the notifier can decide whether to prefix `/en/`.
	resetURL := fmt.Sprintf("%s/auth/reset-password?otp_id=%s&code=%s", baseURL, otpID, code)

	payload, err := json.Marshal(map[string]any{
		"to":         to,
		"reset_url":  resetURL,
		"expires_in": "10 minutes",
		"locale":     loc,
	})
	if err != nil {
		return
	}
	_ = h.enqueuer.EnqueueTask(ctx, taskSendResetPassword, payload, "email")
}

// normalizeLocale coerces the persisted users.locale value to a registry-
// supported short code. Historical data still contains the legacy BCP 47
// strings ("zh-CN" / "en-US") so we run a best-effort Negotiate first; if
// nothing matches we return the registry default rather than the empty
// string so the notifier's resolveLocale fallback is a strict no-op.
func normalizeLocale(raw string) string {
	reg := sharedi18n.MustDefault()
	if raw == "" {
		return reg.DefaultCode()
	}
	if reg.IsSupported(raw) {
		return raw
	}
	if negotiated := reg.Negotiate(raw); negotiated != "" {
		return negotiated
	}
	return reg.DefaultCode()
}

// issueToken creates a new session and signs a JWT carrying the user's
// preferred locale. Pass the short registry code (e.g. "cn" / "en") that the
// caller already has — the middleware's i18n chain trusts this claim to be
// a supported code, so callers must normalize before getting here.
func (h *AuthHandler) issueToken(ctx context.Context, userID, locale string) (token, sessionID string, err error) {
	sessionID = idgen.New("sess")
	if err := h.sessSvc.Store(ctx, sessionID, userID, sessionTTL); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}
	token, err = h.jwtSvc.SignWithLocale(userID, sessionID, locale, accessTokenTTL)
	if err != nil {
		return "", "", fmt.Errorf("sign token: %w", err)
	}
	return token, sessionID, nil
}

// negotiateRegisterLocale derives a short locale code for a freshly
// registered user. Order: explicit request override → Accept-Language → default.
// Always returns a value supported by the shared registry.
func negotiateRegisterLocale(r *http.Request, explicit string) string {
	reg := sharedi18n.MustDefault()
	if explicit != "" && reg.IsSupported(explicit) {
		return explicit
	}
	return reg.Negotiate(r.Header.Get("Accept-Language"))
}

func (h *AuthHandler) issueOTP(ctx context.Context, userID, otpType string, ttl time.Duration) (otpID, code string, err error) {
	code, err = generateOTP()
	if err != nil {
		return "", "", err
	}
	codeHash := h.hashOTP(code)
	otpID = idgen.New("otp")

	expiresAt := pgtype.Timestamptz{}
	_ = expiresAt.Scan(time.Now().Add(ttl))

	if _, err := h.q.CreateUserOTP(ctx, idcdmain.CreateUserOTPParams{
		ID:        otpID,
		UserID:    userID,
		Type:      otpType,
		CodeHash:  codeHash,
		ExpiresAt: expiresAt,
	}); err != nil {
		return "", "", fmt.Errorf("create OTP: %w", err)
	}
	return otpID, code, nil
}

func (h *AuthHandler) verifyOTP(ctx context.Context, userID, otpID, code, otpType string) *apperr.Error {
	otp, err := h.q.GetUserOTPByIDAndType(ctx, idcdmain.GetUserOTPByIDAndTypeParams{
		ID:   otpID,
		Type: otpType,
	})
	if err != nil {
		return apperr.Unauthorized("invalid or expired code")
	}

	// Verify ownership — otp must belong to the requesting user.
	if otp.UserID != userID {
		return apperr.Unauthorized("invalid or expired code")
	}
	if otp.UsedAt.Valid {
		return apperr.Unauthorized("code already used")
	}
	if otp.Attempts >= otpMaxAttempts {
		return apperr.Unauthorized("too many attempts")
	}
	if otp.ExpiresAt.Valid && time.Now().After(otp.ExpiresAt.Time) {
		return apperr.Unauthorized("code expired")
	}

	if h.hashOTP(code) != otp.CodeHash {
		_ = h.q.IncrementUserOTPAttempts(ctx, otp.ID)
		return apperr.Unauthorized("invalid code")
	}

	_ = h.q.MarkUserOTPUsed(ctx, otp.ID)
	return nil
}

// verifyOTPCoded is the errcode-flavoured twin of verifyOTP. Returns
// (codeName, params) when validation fails, or ("", nil) on success. Handlers
// translate the errcode through response.ErrorCode so the user sees a
// localized message instead of the legacy English literal.
//
// Kept alongside verifyOTP (rather than replacing it) so callers that still
// thread apperr.Error through the codebase (e.g. internal helpers) keep
// compiling; both functions share the same underlying logic, so any future
// rule change must land in both.
func (h *AuthHandler) verifyOTPCoded(ctx context.Context, userID, otpID, code, otpType string) (errcode.Code, map[string]any) {
	otp, err := h.q.GetUserOTPByIDAndType(ctx, idcdmain.GetUserOTPByIDAndTypeParams{
		ID:   otpID,
		Type: otpType,
	})
	if err != nil {
		return errcode.AuthOTPInvalid, nil
	}
	if otp.UserID != userID {
		return errcode.AuthOTPInvalid, nil
	}
	if otp.UsedAt.Valid {
		return errcode.AuthOTPInvalid, nil
	}
	if otp.Attempts >= otpMaxAttempts {
		return errcode.AuthOTPAttemptsExceeded, nil
	}
	if otp.ExpiresAt.Valid && time.Now().After(otp.ExpiresAt.Time) {
		return errcode.AuthOTPExpired, nil
	}
	if h.hashOTP(code) != otp.CodeHash {
		_ = h.q.IncrementUserOTPAttempts(ctx, otp.ID)
		return errcode.AuthOTPInvalid, nil
	}
	_ = h.q.MarkUserOTPUsed(ctx, otp.ID)
	return "", nil
}

func generateOTP() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (h *AuthHandler) hashOTP(code string) string {
	mac := hmac.New(sha256.New, h.otpSecret)
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *AuthHandler) userHas2FA(ctx context.Context, userID string) bool {
	var dummy string
	err := h.mfaPool.QueryRow(ctx,
		`SELECT user_id FROM user_2fa WHERE user_id = $1`, userID,
	).Scan(&dummy)
	return err == nil
}

type twoFactorLoginRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

func (h *AuthHandler) TwoFactorLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req twoFactorLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorCode(ctx, w, r, errcode.RequestBodyBad, nil)
		return
	}
	if req.MFAToken == "" || req.Code == "" {
		response.ErrorCode(ctx, w, r, errcode.ValidationFailed, nil)
		return
	}

	userID, err := h.mfaRedis.Get(ctx, mfaPendingKeyPrefix+req.MFAToken).Result()
	if err != nil {
		response.ErrorCode(ctx, w, r, errcode.Auth2FAInvalid, nil)
		return
	}

	var secretBytes []byte
	err = h.mfaPool.QueryRow(ctx,
		`SELECT secret_encrypted FROM user_2fa WHERE user_id = $1`, userID,
	).Scan(&secretBytes)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch 2FA secret", err))
		return
	}

	var plainSecret []byte
	if h.fieldCipher != nil {
		plainSecret, err = h.fieldCipher.Decrypt(secretBytes)
		if err != nil {
			response.Error(w, r, apperr.Internal("failed to decrypt 2FA secret", err))
			return
		}
	} else {
		plainSecret = secretBytes
	}
	secret := strings.TrimSpace(string(plainSecret))
	ok, err := totp.ValidateCode(secret, req.Code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to validate code", err))
		return
	}
	if !ok {
		response.ErrorCode(ctx, w, r, errcode.Auth2FAInvalid, nil)
		return
	}

	_ = h.mfaRedis.Del(ctx, mfaPendingKeyPrefix+req.MFAToken)

	user, err := h.q.GetUserByID(ctx, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	token, _, err := h.issueToken(ctx, user.ID, user.Locale)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	_ = h.q.UpdateUserLastLogin(ctx, idcdmain.UpdateUserLastLoginParams{ID: user.ID})

	writeAuthSuccess(w, r, http.StatusOK, token, user.ID)
}
