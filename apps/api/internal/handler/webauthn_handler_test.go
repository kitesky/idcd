package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/lib/auth/webauthn"
)

func setupWebAuthnHandler(t *testing.T) (*WebAuthnHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	h := &WebAuthnHandler{pool: mockPool, rpID: "idcd.com"}
	return h, mockPool
}

func injectWebAuthnUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	return r.WithContext(ctx)
}

func withChiParamWebAuthn(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestWebAuthnRegisterBegin_Unauthenticated_Returns401(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/begin", nil)
	rr := httptest.NewRecorder()

	h.RegisterBegin(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnRegisterBegin_Authenticated_Returns200WithOptions(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT email FROM "user"`).
		WithArgs("u1").
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow("alice@example.com"))

	mockPool.ExpectExec(`INSERT INTO webauthn_challenges`).
		WithArgs(pgxmock.AnyArg(), "u1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/begin", nil)
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.RegisterBegin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data registerBeginResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.ChallengeID == "" {
		t.Error("expected non-empty challenge_id")
	}
	if wrapped.Data.Options.Challenge == "" {
		t.Error("expected non-empty options.challenge")
	}
	if wrapped.Data.Options.RelyingParty.ID != "idcd.com" {
		t.Errorf("expected rp.id idcd.com, got %q", wrapped.Data.Options.RelyingParty.ID)
	}
}

func TestWebAuthnRegisterComplete_InvalidChallenge_Returns400(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT user_id, expires_at FROM webauthn_challenges`).
		WithArgs("bad-challenge").
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(registerCompleteRequest{
		Challenge:  "bad-challenge",
		Response:   map[string]any{"id": "x"},
		DeviceName: "My Mac",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/complete", bytes.NewReader(body))
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.RegisterComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnList_Returns200WithList(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	now := time.Now()
	mockPool.ExpectQuery(`SELECT id, device_name, created_at, last_used_at`).
		WithArgs("u1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_name", "created_at", "last_used_at"}).
			AddRow("wc_1", "My MacBook", now, nil))

	req := httptest.NewRequest(http.MethodGet, "/v1/account/passkeys", nil)
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data []passkeyListItem `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(wrapped.Data) != 1 {
		t.Errorf("expected 1 passkey, got %d", len(wrapped.Data))
	}
}

func TestWebAuthnList_Unauthenticated_Returns401(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/account/passkeys", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnDelete_Returns204(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(`DELETE FROM webauthn_credentials`).
		WithArgs("wc_1", "u1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/passkeys/wc_1", nil)
	req = injectWebAuthnUserID(req, "u1")
	req = withChiParamWebAuthn(req, "id", "wc_1")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnDelete_Unauthenticated_Returns401(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodDelete, "/v1/account/passkeys/wc_1", nil)
	req = withChiParamWebAuthn(req, "id", "wc_1")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnAuthBegin_Returns200WithRequestOptions(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(`INSERT INTO webauthn_challenges`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(authBeginRequest{})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/begin", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthBegin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data authBeginResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.ChallengeID == "" {
		t.Error("expected non-empty challenge_id")
	}
	if wrapped.Data.Options.RPID != "idcd.com" {
		t.Errorf("expected rpId idcd.com, got %q", wrapped.Data.Options.RPID)
	}
}

func TestWebAuthnAuthBegin_WithUserEmail_Returns200WithCredentials(t *testing.T) {
	h, mockPool := setupWebAuthnHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT id FROM "user"`).
		WithArgs("alice@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("u1"))

	mockPool.ExpectQuery(`SELECT credential_id FROM webauthn_credentials`).
		WithArgs("u1").
		WillReturnRows(pgxmock.NewRows([]string{"credential_id"}).AddRow("cred123"))

	mockPool.ExpectExec(`INSERT INTO webauthn_challenges`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(authBeginRequest{UserIDOrEmail: "alice@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/begin", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthBegin(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestWebAuthnChallengeIsBase64URL(t *testing.T) {
	ch, err := webauthn.GenerateChallenge()
	if err != nil {
		t.Fatalf("generate challenge: %v", err)
	}
	_, err = base64.RawURLEncoding.DecodeString(ch)
	if err != nil {
		t.Errorf("challenge is not valid base64url: %v", err)
	}
}
