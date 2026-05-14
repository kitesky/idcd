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

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/password"
	"github.com/kite365/idcd/lib/auth/totp"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/db/repository"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	accessTokenTTL = 24 * time.Hour
	sessionTTL     = 90 * 24 * time.Hour
	otpTTL         = 10 * time.Minute
	otpTypeVerify  = "email_verify"
	otpTypeReset   = "password_reset"
	otpMaxAttempts = 5
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

// JWTSigner signs JWT tokens.
type JWTSigner interface {
	Sign(userID, sessionID string, expiry time.Duration) (string, error)
}

// SessionStorer stores and deletes sessions.
type SessionStorer interface {
	Store(ctx context.Context, sessionID, userID string, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
}

// MFAPool is the minimal pgx interface needed for MFA checks.
type MFAPool interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
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

// Register handles POST /v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	if req.Email == "" {
		response.Error(w, r, apperr.Validation("email is required", ""))
		return
	}
	if err := password.ValidatePassword(req.Password, req.Email); err != nil {
		response.Error(w, r, apperr.Validation(err.Error(), ""))
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

	user, err := h.q.CreateUser(r.Context(), idcdmain.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: &hash,
		DisplayName:  displayNamePtr,
		Locale:       "zh-CN",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			response.Error(w, r, apperr.Duplicate("email already registered"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to create user", err))
		return
	}

	if req.ReferralCode != "" && h.referralPool != nil {
		h.recordReferral(r.Context(), req.ReferralCode, user.ID)
	}

	token, _, err := h.issueToken(r.Context(), user.ID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	setAuthCookie(w, r, token)
	response.JSON(w, r, http.StatusCreated, authResponse{
		ExpiresIn: int(accessTokenTTL.Seconds()),
		UserID:    user.ID,
	})
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

	rewardID := idgen.New("rwd_")
	_, _ = h.referralPool.Exec(ctx, `
		INSERT INTO referral_rewards (id, referrer_id, referred_id, code, status, reward_amount)
		VALUES ($1, $2, $3, $4, 'pending', 10.00)
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
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}

	if req.Email == "" || req.Password == "" {
		response.Error(w, r, apperr.Validation("email and password are required", ""))
		return
	}

	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			response.Error(w, r, apperr.Unauthorized("invalid credentials"))
			return
		}
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	if user.PasswordHash == nil || !password.Verify(req.Password, *user.PasswordHash) {
		response.Error(w, r, apperr.Unauthorized("invalid credentials"))
		return
	}

	if user.Status == "suspended" || user.Status == "deleted" {
		response.Error(w, r, apperr.Forbidden("account is not active"))
		return
	}

	if h.mfaPool != nil && h.mfaRedis != nil && h.userHas2FA(r.Context(), user.ID) {
		mfaToken := idgen.New("mfa")
		if err := h.mfaRedis.Set(r.Context(), mfaPendingKeyPrefix+mfaToken, user.ID, mfaPendingTTL).Err(); err != nil {
			response.Error(w, r, apperr.Internal("failed to create mfa session", err))
			return
		}
		response.JSON(w, r, http.StatusOK, map[string]interface{}{
			"mfa_required": true,
			"mfa_token":    mfaToken,
		})
		return
	}

	token, _, err := h.issueToken(r.Context(), user.ID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	_ = h.q.UpdateUserLastLogin(r.Context(), idcdmain.UpdateUserLastLoginParams{ID: user.ID})

	setAuthCookie(w, r, token)
	response.JSON(w, r, http.StatusOK, authResponse{
		ExpiresIn: int(accessTokenTTL.Seconds()),
		UserID:    user.ID,
	})
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

// setAuthCookie issues the JWT as an HttpOnly, Secure, SameSite=Strict cookie.
// Storing the token in a cookie (not localStorage) prevents XSS-based token theft.
func setAuthCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(accessTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
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
		SameSite: http.SameSiteStrictMode,
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
	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.OtpID == "" || req.Code == "" {
		response.Error(w, r, apperr.Validation("otp_id and code are required", ""))
		return
	}

	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("invalid request"))
		return
	}

	if err := h.verifyOTP(r.Context(), user.ID, req.OtpID, req.Code, otpTypeVerify); err != nil {
		response.Error(w, r, err)
		return
	}

	if _, err := h.q.UpdateUserEmailVerified(r.Context(), user.ID); err != nil {
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
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Email == "" {
		response.Error(w, r, apperr.Validation("email is required", ""))
		return
	}

	const sameMsg = "if the email exists, a reset code has been sent"

	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Timing equalisation: sleep approximately as long as issueOTP would take
		// so that attackers cannot enumerate registered emails via response-time differences.
		time.Sleep(50 * time.Millisecond)
		response.JSON(w, r, http.StatusOK, map[string]string{"message": sameMsg})
		return
	}

	otpID, _, err := h.issueOTP(r.Context(), user.ID, otpTypeReset, otpTTL)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue OTP", err))
		return
	}
	// otp_id must only travel via email to the user — never returned in the API response.
	_ = otpID
	response.JSON(w, r, http.StatusOK, map[string]string{"message": sameMsg})
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
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Email == "" || req.OtpID == "" || req.Code == "" || req.NewPassword == "" {
		response.Error(w, r, apperr.Validation("email, otp_id, code, and new_password are required", ""))
		return
	}

	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("invalid request"))
		return
	}

	if err := h.verifyOTP(r.Context(), user.ID, req.OtpID, req.Code, otpTypeReset); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := password.ValidatePassword(req.NewPassword, req.Email); err != nil {
		response.Error(w, r, apperr.Validation(err.Error(), ""))
		return
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to hash password", err))
		return
	}

	if err := h.q.UpdateUserPasswordHash(r.Context(), idcdmain.UpdateUserPasswordHashParams{
		ID:           user.ID,
		PasswordHash: &hash,
	}); err != nil {
		response.Error(w, r, apperr.Internal("failed to update password", err))
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

// --- Helpers ---

func (h *AuthHandler) issueToken(ctx context.Context, userID string) (token, sessionID string, err error) {
	sessionID = idgen.New("sess")
	if err := h.sessSvc.Store(ctx, sessionID, userID, sessionTTL); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}
	token, err = h.jwtSvc.Sign(userID, sessionID, accessTokenTTL)
	if err != nil {
		return "", "", fmt.Errorf("sign token: %w", err)
	}
	return token, sessionID, nil
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
	var req twoFactorLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.MFAToken == "" || req.Code == "" {
		response.Error(w, r, apperr.Validation("mfa_token and code are required", ""))
		return
	}

	userID, err := h.mfaRedis.Get(r.Context(), mfaPendingKeyPrefix+req.MFAToken).Result()
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("invalid or expired mfa token"))
		return
	}

	var secretBytes []byte
	err = h.mfaPool.QueryRow(r.Context(),
		`SELECT secret_encrypted FROM user_2fa WHERE user_id = $1`, userID,
	).Scan(&secretBytes)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch 2FA secret", err))
		return
	}

	secret := strings.TrimSpace(string(secretBytes))
	ok, err := totp.ValidateCode(secret, req.Code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to validate code", err))
		return
	}
	if !ok {
		response.Error(w, r, apperr.Unauthorized("invalid TOTP code"))
		return
	}

	_ = h.mfaRedis.Del(r.Context(), mfaPendingKeyPrefix+req.MFAToken)

	user, err := h.q.GetUserByID(r.Context(), userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	token, _, err := h.issueToken(r.Context(), user.ID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}

	_ = h.q.UpdateUserLastLogin(r.Context(), idcdmain.UpdateUserLastLoginParams{ID: user.ID})

	setAuthCookie(w, r, token)
	response.JSON(w, r, http.StatusOK, authResponse{
		ExpiresIn: int(accessTokenTTL.Seconds()),
		UserID:    user.ID,
	})
}
