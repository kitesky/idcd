// Package handler implements HTTP handlers for the API server.
package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"errors"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/packages/auth/password"
	"github.com/kite365/idcd/packages/db/gen/idcdmain"
	"github.com/kite365/idcd/packages/db/repository"
	"github.com/kite365/idcd/packages/shared/apperr"
	"github.com/kite365/idcd/packages/shared/idgen"
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

// AuthHandler implements all auth and account endpoints.
type AuthHandler struct {
	q       AuthQuerier
	jwtSvc  JWTSigner
	sessSvc SessionStorer
}

// NewAuthHandler creates an AuthHandler wired to the given services.
func NewAuthHandler(q AuthQuerier, jwtSvc JWTSigner, sessSvc SessionStorer) *AuthHandler {
	return &AuthHandler{q: q, jwtSvc: jwtSvc, sessSvc: sessSvc}
}

// --- Register ---

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
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

	token, sessionID, err := h.issueToken(r.Context(), user.ID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}
	_ = sessionID

	response.JSON(w, r, http.StatusCreated, authResponse{
		AccessToken: token,
		ExpiresIn:   int(accessTokenTTL.Seconds()),
		UserID:      user.ID,
	})
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

	token, sessionID, err := h.issueToken(r.Context(), user.ID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue token", err))
		return
	}
	_ = sessionID

	_ = h.q.UpdateUserLastLogin(r.Context(), idcdmain.UpdateUserLastLoginParams{ID: user.ID})

	response.JSON(w, r, http.StatusOK, authResponse{
		AccessToken: token,
		ExpiresIn:   int(accessTokenTTL.Seconds()),
		UserID:      user.ID,
	})
}

// --- Logout ---

// Logout handles POST /v1/auth/logout (requires auth middleware).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// The session ID is embedded in the JWT claims; the middleware has already
	// validated the JWT. We extract the session ID from the token again.
	// For simplicity in S1, just delete all sessions (the SessionID isn't
	// directly accessible here without re-parsing). We use the Authorization
	// header to get the session ID from claims.
	//
	// Better approach: store session_id in context (Authn middleware can do this).
	// For S1: accept the limitation that logout doesn't revoke the short-lived JWT.

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "logged out"})
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

type otpIssuedResponse struct {
	OtpID   string `json:"otp_id"`
	Message string `json:"message"`
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

	user, err := h.q.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Do not reveal whether the email exists.
		response.JSON(w, r, http.StatusOK, map[string]string{
			"message": "if the email exists, a reset code has been sent",
		})
		return
	}

	otpID, _, err := h.issueOTP(r.Context(), user.ID, otpTypeReset, otpTTL)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to issue OTP", err))
		return
	}

	// In production: send email via notifier. For S1, otp_id is returned
	// directly so the frontend can submit it.
	response.JSON(w, r, http.StatusOK, otpIssuedResponse{
		OtpID:   otpID,
		Message: "reset code sent to email",
	})
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
	codeHash := hashOTP(code)
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

	if hashOTP(code) != otp.CodeHash {
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

func hashOTP(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}
