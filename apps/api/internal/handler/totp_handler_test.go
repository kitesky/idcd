package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/totp"
)

func setupTOTPHandler(t *testing.T) (*TOTPHandler, pgxmock.PgxPoolIface, *miniredis.Miniredis) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	h := &TOTPHandler{pool: mockPool, redis: rdb}
	return h, mockPool, mr
}

func injectTOTPUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func TestTOTPStatus_NotEnabled(t *testing.T) {
	h, mockPool, mr := setupTOTPHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	mockPool.ExpectQuery(`SELECT user_id FROM user_2fa`).
		WithArgs("u1").
		WillReturnError(pgx.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/2fa/status", nil)
	req = injectTOTPUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data totpStatusResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.Enabled {
		t.Errorf("expected enabled=false")
	}
}

func TestTOTPSetup_ReturnsSecretAndURL(t *testing.T) {
	h, mockPool, mr := setupTOTPHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	mockPool.ExpectQuery(`SELECT email FROM "user"`).
		WithArgs("u1").
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("user@example.com"))

	req := httptest.NewRequest(http.MethodPost, "/v1/account/2fa/setup", nil)
	req = injectTOTPUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.Setup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data totpSetupResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.Secret == "" {
		t.Errorf("expected non-empty secret")
	}
	if wrapped.Data.OtpauthURL == "" {
		t.Errorf("expected non-empty otpauth_url")
	}
}

func TestTOTPVerify_CorrectCode_Returns200WithBackupCodes(t *testing.T) {
	h, mockPool, mr := setupTOTPHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	secret, _ := totp.GenerateSecret()
	code, _ := totp.GenerateCode(secret, time.Now())

	mr.Set(totpSetupKeyPrefix+"u1", secret)

	mockPool.ExpectExec(`INSERT INTO user_2fa`).
		WithArgs("u1", []byte(secret), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(totpVerifyRequest{Code: code})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/2fa/verify", bytes.NewReader(body))
	req = injectTOTPUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.Verify(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data totpVerifyResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrapped.Data.BackupCodes) != 8 {
		t.Errorf("expected 8 backup codes, got %d", len(wrapped.Data.BackupCodes))
	}
}

func TestTOTPVerify_WrongCode_Returns400(t *testing.T) {
	h, mockPool, mr := setupTOTPHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	secret, _ := totp.GenerateSecret()
	mr.Set(totpSetupKeyPrefix+"u1", secret)

	realCode, _ := totp.GenerateCode(secret, time.Now())
	wrongCode := "000000"
	if realCode == wrongCode {
		wrongCode = "111111"
	}

	body, _ := json.Marshal(totpVerifyRequest{Code: wrongCode})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/2fa/verify", bytes.NewReader(body))
	req = injectTOTPUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.Verify(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTOTPDisable_CorrectCode_Returns200(t *testing.T) {
	h, mockPool, mr := setupTOTPHandler(t)
	defer mockPool.Close()
	defer mr.Close()

	secret, _ := totp.GenerateSecret()
	code, _ := totp.GenerateCode(secret, time.Now())

	mockPool.ExpectQuery(`SELECT secret_encrypted FROM user_2fa`).
		WithArgs("u1").
		WillReturnRows(pgxmock.NewRows([]string{"secret_encrypted"}).AddRow([]byte(secret)))

	mockPool.ExpectExec(`DELETE FROM user_2fa`).
		WithArgs("u1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	body, _ := json.Marshal(totpVerifyRequest{Code: code})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/2fa/disable", bytes.NewReader(body))
	req = injectTOTPUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.Disable(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
