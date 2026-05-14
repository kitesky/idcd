package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

const enrollTestAdminToken = "test-admin-token"
const testGatewayURL = "wss://gateway.idcd.com"

func setupEnrollmentHandler(t *testing.T) (*NodeEnrollmentHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	h := &NodeEnrollmentHandler{
		pool:       mockPool,
		gatewayURL: testGatewayURL,
		adminToken: enrollTestAdminToken,
	}
	return h, mockPool
}

func TestCreateEnrollmentToken_Success(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(`INSERT INTO node_enrollment_tokens`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), "tok-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(createEnrollmentTokenRequest{Label: "tok-1"})
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/nodes/enrollment-tokens", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	rr := httptest.NewRecorder()

	h.CreateEnrollmentToken(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data createEnrollmentTokenResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Token) < 10 || resp.Data.Token[:4] != "ent_" {
		t.Errorf("unexpected token format: %q", resp.Data.Token)
	}
	if resp.Data.ExpiresAt.Before(time.Now()) {
		t.Errorf("expires_at should be in the future")
	}
}

func TestCreateEnrollmentToken_Unauthorized(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/internal/admin/nodes/enrollment-tokens", nil)
	req.Header.Set("X-Admin-Token", "wrong-token")
	rr := httptest.NewRecorder()

	h.CreateEnrollmentToken(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestCreateEnrollmentToken_InvalidExpiry(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	body, _ := json.Marshal(createEnrollmentTokenRequest{ExpiresIn: "not-a-duration"})
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/nodes/enrollment-tokens", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	rr := httptest.NewRecorder()

	h.CreateEnrollmentToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestEnroll_Success(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	tokenVal := "ent_" + "a" + "b"
	tokenHash := hashToken(tokenVal)

	expiresAt := time.Now().Add(time.Hour)

	// First QueryRow: token lookup
	mockPool.ExpectQuery(`SELECT id, expires_at, used_at`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "expires_at", "used_at"}).
			AddRow("et_001", expiresAt, nil))

	// Exec: insert enrolled_nodes
	mockPool.ExpectExec(`INSERT INTO enrolled_nodes`).
		WithArgs(
			pgxmock.AnyArg(), // id
			pgxmock.AnyArg(), // node_id
			pgxmock.AnyArg(), // secret_hash
			"myhost",         // hostname
			"amd64",          // arch
			"linux",          // os
			"6.1.0",          // kernel
			pgxmock.AnyArg(), // ip_address
			"1.0.0",          // version
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	// Exec: mark token used
	mockPool.ExpectExec(`UPDATE node_enrollment_tokens`).
		WithArgs(pgxmock.AnyArg(), "et_001").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(enrollRequest{
		Token:    tokenVal,
		Hostname: "myhost",
		Arch:     "amd64",
		OS:       "linux",
		Kernel:   "6.1.0",
		Version:  "1.0.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Enroll(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data enrollResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.NodeID[:3] != "nd_" {
		t.Errorf("unexpected node_id format: %q", resp.Data.NodeID)
	}
	if len(resp.Data.SecretKey) != 64 {
		t.Errorf("expected 64-char secret_key (32 bytes hex), got %d", len(resp.Data.SecretKey))
	}
	if resp.Data.GatewayURL != testGatewayURL {
		t.Errorf("expected gateway_url %q, got %q", testGatewayURL, resp.Data.GatewayURL)
	}
}

func TestEnroll_InvalidToken(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	tokenHash := hashToken("ent_badtoken")
	mockPool.ExpectQuery(`SELECT id, expires_at, used_at`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(enrollRequest{Token: "ent_badtoken", Hostname: "h", Arch: "amd64", OS: "linux"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Enroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestEnroll_AlreadyUsedToken(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	tokenHash := hashToken("ent_usedtoken")
	usedAt := time.Now().Add(-time.Hour)
	mockPool.ExpectQuery(`SELECT id, expires_at, used_at`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "expires_at", "used_at"}).
			AddRow("et_001", time.Now().Add(time.Hour), &usedAt))

	body, _ := json.Marshal(enrollRequest{Token: "ent_usedtoken", Hostname: "h", Arch: "amd64", OS: "linux"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Enroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestEnroll_ExpiredToken(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	tokenHash := hashToken("ent_expiredtoken")
	mockPool.ExpectQuery(`SELECT id, expires_at, used_at`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "expires_at", "used_at"}).
			AddRow("et_001", time.Now().Add(-time.Hour), nil))

	body, _ := json.Marshal(enrollRequest{Token: "ent_expiredtoken", Hostname: "h", Arch: "amd64", OS: "linux"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Enroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("test-token")
	h2 := hashToken("test-token")
	if h1 != h2 {
		t.Error("hashToken must be deterministic")
	}
	if h1 == hashToken("other-token") {
		t.Error("different tokens must produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d", len(h1))
	}
}
