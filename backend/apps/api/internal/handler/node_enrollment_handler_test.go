package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/kite365/idcd/lib/shared/idgen"
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
	tokenHash := idgen.SHA256Hex(tokenVal)

	// Enroll now runs inside a transaction.
	mockPool.ExpectBegin()

	// Atomic UPDATE RETURNING — claims the token in one shot
	mockPool.ExpectQuery(`UPDATE node_enrollment_tokens`).
		WithArgs(tokenHash).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("et_001"))

	// INSERT enrolled node
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

	// UPDATE used_by (non-fatal, fires after node is registered)
	mockPool.ExpectExec(`UPDATE node_enrollment_tokens SET used_by`).
		WithArgs(pgxmock.AnyArg(), "et_001").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	mockPool.ExpectCommit()

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

	tokenHash := idgen.SHA256Hex("ent_badtoken")
	mockPool.ExpectBegin()
	// Atomic UPDATE finds nothing (token not found / expired / already used)
	mockPool.ExpectQuery(`UPDATE node_enrollment_tokens`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)
	mockPool.ExpectRollback()

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

	tokenHash := idgen.SHA256Hex("ent_usedtoken")
	mockPool.ExpectBegin()
	// Token is already used: WHERE used_at IS NULL fails → ErrNoRows
	mockPool.ExpectQuery(`UPDATE node_enrollment_tokens`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)
	mockPool.ExpectRollback()

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

	tokenHash := idgen.SHA256Hex("ent_expiredtoken")
	mockPool.ExpectBegin()
	// Token is expired: WHERE expires_at > now() fails → ErrNoRows
	mockPool.ExpectQuery(`UPDATE node_enrollment_tokens`).
		WithArgs(tokenHash).
		WillReturnError(pgx.ErrNoRows)
	mockPool.ExpectRollback()

	body, _ := json.Marshal(enrollRequest{Token: "ent_expiredtoken", Hostname: "h", Arch: "amd64", OS: "linux"})
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/enroll", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.Enroll(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

// activateReq is a small helper to build a request with the node_id URL param
// wired through chi's RouteContext, so chi.URLParam(r, "node_id") works.
func activateReq(t *testing.T, nodeID, adminToken string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/admin/nodes/"+nodeID+"/activate", nil)
	if adminToken != "" {
		req.Header.Set("X-Admin-Token", adminToken)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("node_id", nodeID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestNodeActivate_Success_FromPending(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`UPDATE enrolled_nodes`).
		WithArgs("nd_test_001").
		WillReturnRows(pgxmock.NewRows([]string{"status"}).AddRow("pending"))

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "nd_test_001", enrollTestAdminToken))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Data activateResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.NodeID != "nd_test_001" {
		t.Errorf("node_id mismatch: %q", resp.Data.NodeID)
	}
	if resp.Data.Status != "active" {
		t.Errorf("expected status=active, got %q", resp.Data.Status)
	}
	if resp.Data.PreviousStatus != "pending" {
		t.Errorf("expected previous_status=pending, got %q", resp.Data.PreviousStatus)
	}
	if err := mockPool.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestNodeActivate_Unauthorized(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "nd_test_001", "wrong-token"))

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestNodeActivate_MissingNodeID(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "", enrollTestAdminToken))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestNodeActivate_NodeNotFound(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	// First UPDATE returns no rows.
	mockPool.ExpectQuery(`UPDATE enrolled_nodes`).
		WithArgs("nd_missing").
		WillReturnError(pgx.ErrNoRows)
	// Disambiguation SELECT also finds nothing → confirms node doesn't exist.
	mockPool.ExpectQuery(`SELECT status FROM enrolled_nodes`).
		WithArgs("nd_missing").
		WillReturnError(pgx.ErrNoRows)

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "nd_missing", enrollTestAdminToken))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNodeActivate_DisabledNode_Conflict(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`UPDATE enrolled_nodes`).
		WithArgs("nd_banned").
		WillReturnError(pgx.ErrNoRows)
	// Disambiguation reveals the node is disabled → 409.
	mockPool.ExpectQuery(`SELECT status FROM enrolled_nodes`).
		WithArgs("nd_banned").
		WillReturnRows(pgxmock.NewRows([]string{"status"}).AddRow("disabled"))

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "nd_banned", enrollTestAdminToken))

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestNodeActivate_DBError(t *testing.T) {
	h, mockPool := setupEnrollmentHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`UPDATE enrolled_nodes`).
		WithArgs("nd_dberr").
		WillReturnError(errors.New("connection lost"))

	rr := httptest.NewRecorder()
	h.NodeActivate(rr, activateReq(t, "nd_dberr", enrollTestAdminToken))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSHA256Hex_Deterministic(t *testing.T) {
	h1 := idgen.SHA256Hex("test-token")
	h2 := idgen.SHA256Hex("test-token")
	if h1 != h2 {
		t.Error("SHA256Hex must be deterministic")
	}
	if h1 == idgen.SHA256Hex("other-token") {
		t.Error("different inputs must produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex SHA-256, got %d", len(h1))
	}
}
