package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/totp"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/aesenc"
)

const (
	totpSetupKeyPrefix  = "2fa:setup:"
	totpSetupTTL        = 10 * time.Minute
	mfaPendingKeyPrefix = "mfa:pending:"
	mfaPendingTTL       = 5 * time.Minute
	totpIssuer          = "idcd"
)

type TOTPPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type TOTPHandler struct {
	pool   TOTPPool
	redis  *redis.Client
	cipher *aesenc.Cipher
}

func NewTOTPHandler(pool *pgxpool.Pool, rdb *redis.Client, cipher *aesenc.Cipher) *TOTPHandler {
	return &TOTPHandler{pool: pool, redis: rdb, cipher: cipher}
}

type totpSetupResponse struct {
	Secret     string `json:"secret"`
	OtpauthURL string `json:"otpauth_url"`
}

type totpVerifyRequest struct {
	Code string `json:"code"`
}

type totpVerifyResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

type totpStatusResponse struct {
	Enabled bool `json:"enabled"`
}

func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	secret, err := totp.GenerateSecret()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate secret", err))
		return
	}

	key := totpSetupKeyPrefix + userID
	if err := h.redis.Set(r.Context(), key, secret, totpSetupTTL).Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to store setup secret", err))
		return
	}

	var email string
	err = h.pool.QueryRow(r.Context(),
		`SELECT email FROM "user" WHERE id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&email)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	url := totp.OTPAuthURL(totpIssuer, email, secret)
	response.JSON(w, r, http.StatusOK, totpSetupResponse{
		Secret:     secret,
		OtpauthURL: url,
	})
}

func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req totpVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Code == "" {
		response.Error(w, r, apperr.Validation("code is required", ""))
		return
	}

	key := totpSetupKeyPrefix + userID
	secret, err := h.redis.Get(r.Context(), key).Result()
	if err != nil {
		response.Error(w, r, apperr.Validation("setup session expired, please restart setup", ""))
		return
	}

	// Atomic replay check via SetNX — replaces the prior non-atomic Exists+Set
	// (TOCTOU: two concurrent requests could both pass Exists, then both Set).
	v := &totp.Validator{Replay: totp.NewRedisReplayStore(h.redis)}
	ok, err := v.Validate(r.Context(), secret, userID, req.Code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to validate code", err))
		return
	}
	if !ok {
		response.Error(w, r, apperr.Validation("invalid TOTP code", ""))
		return
	}

	backupCodes, err := totp.GenerateBackupCodes()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate backup codes", err))
		return
	}

	backupJSON, err := json.Marshal(backupCodes)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to encode backup codes", err))
		return
	}

	encSecret, err := h.cipher.Encrypt([]byte(secret))
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to encrypt secret", err))
		return
	}
	encBackup, err := h.cipher.Encrypt(backupJSON)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to encrypt backup codes", err))
		return
	}

	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO user_2fa (user_id, type, secret_encrypted, backup_codes_encrypted, enabled_at)
		VALUES ($1, 'totp', $2, $3, now())
		ON CONFLICT (user_id) DO UPDATE
		  SET type = 'totp',
		      secret_encrypted = EXCLUDED.secret_encrypted,
		      backup_codes_encrypted = EXCLUDED.backup_codes_encrypted,
		      enabled_at = now()
	`, userID, encSecret, encBackup)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to enable 2FA", err))
		return
	}

	_ = h.redis.Del(r.Context(), key)

	response.JSON(w, r, http.StatusOK, totpVerifyResponse{BackupCodes: backupCodes})
}

func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req totpVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Code == "" {
		response.Error(w, r, apperr.Validation("code is required", ""))
		return
	}

	var secretBytes []byte
	err := h.pool.QueryRow(r.Context(),
		`SELECT secret_encrypted FROM user_2fa WHERE user_id = $1`, userID,
	).Scan(&secretBytes)
	if err != nil {
		response.Error(w, r, apperr.Validation("2FA is not enabled", ""))
		return
	}

	decSecret, err := h.cipher.Decrypt(secretBytes)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to decrypt secret", err))
		return
	}
	secret := strings.TrimSpace(string(decSecret))

	// Atomic replay check via SetNX — replaces the prior non-atomic Exists+Set.
	v := &totp.Validator{Replay: totp.NewRedisReplayStore(h.redis)}
	ok, err := v.Validate(r.Context(), secret, userID, req.Code)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to validate code", err))
		return
	}
	if !ok {
		response.Error(w, r, apperr.Validation("invalid TOTP code", ""))
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`DELETE FROM user_2fa WHERE user_id = $1`, userID,
	)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to disable 2FA", err))
		return
	}

	auditLog(r, "audit.2fa.disabled")
	response.JSON(w, r, http.StatusOK, map[string]string{"message": "2FA disabled"})
}

func (h *TOTPHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var dummy string
	err := h.pool.QueryRow(r.Context(),
		`SELECT user_id FROM user_2fa WHERE user_id = $1`, userID,
	).Scan(&dummy)

	enabled := err == nil
	response.JSON(w, r, http.StatusOK, totpStatusResponse{Enabled: enabled})
}
