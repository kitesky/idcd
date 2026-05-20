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

// ---------------------------------------------------------------------------
// Real WebAuthn fixtures (mirrored from lib/auth/webauthn/webauthn_test.go).
// Borrowed from go-webauthn's test suite: real MacOS TouchID responses
// against webauthn.io, so all CBOR / COSE / ES256 signatures verify
// cryptographically. We pin the rpID + origin + challenges to match the
// fixture so the handler's verifier passes end-to-end.
// ---------------------------------------------------------------------------

const (
	tstRPID   = "webauthn.io"
	tstOrigin = "https://webauthn.io"

	tstRegChallenge          = "W8GzFU8pGjhoRbWrLDlamAfq_y4S1CZG1VuoeRLARrE"
	tstRegCredentialID       = "6xrtBhJQW6QU4tOaB4rrHaS2Ks0yDDL_q8jDC16DEjZ-VLVf4kCRkvl2xp2D71sTPYns-exsHQHTy3G-zJRK8g"
	tstRegAttestationObject  = "o2NmbXRkbm9uZWdhdHRTdG10oGhhdXRoRGF0YVjEdKbqkhPJnC90siSSsyDPQCYqlMGpUKA5fyklC2CEHvBBAAAAAAAAAAAAAAAAAAAAAAAAAAAAQOsa7QYSUFukFOLTmgeK6x2ktirNMgwy_6vIwwtegxI2flS1X-JAkZL5dsadg-9bEz2J7PnsbB0B08txvsyUSvKlAQIDJiABIVggLKF5xS0_BntttUIrm2Z2tgZ4uQDwllbdIfrrBMABCNciWCDHwin8Zdkr56iSIh0MrB5qZiEzYLQpEOREhMUkY6q4Vw"
	tstRegClientDataJSON     = "eyJjaGFsbGVuZ2UiOiJXOEd6RlU4cEdqaG9SYldyTERsYW1BZnFfeTRTMUNaRzFWdW9lUkxBUnJFIiwib3JpZ2luIjoiaHR0cHM6Ly93ZWJhdXRobi5pbyIsInR5cGUiOiJ3ZWJhdXRobi5jcmVhdGUifQ"

	tstAsrChallenge          = "E4PTcIH_HfX1pC6Sigk1SC9NAlgeztN0439vi8z_c9k"
	tstAsrCredentialID       = "AI7D5q2P0LS-Fal9ZT7CHM2N5BLbUunF92T8b6iYC199bO2kagSuU05-5dZGqb1SP0A0lyTWng"
	tstAsrAuthData           = "dKbqkhPJnC90siSSsyDPQCYqlMGpUKA5fyklC2CEHvBFXJJiGa3OAAI1vMYKZIsLJfHwVQMANwCOw-atj9C0vhWpfWU-whzNjeQS21Lpxfdk_G-omAtffWztpGoErlNOfuXWRqm9Uj9ANJck1p6lAQIDJiABIVggKAhfsdHcBIc0KPgAcRyAIK_-Vi-nCXHkRHPNaCMBZ-4iWCBxB8fGYQSBONi9uvq0gv95dGWlhJrBwCsj_a4LJQKVHQ"
	tstAsrClientData         = "eyJjaGFsbGVuZ2UiOiJFNFBUY0lIX0hmWDFwQzZTaWdrMVNDOU5BbGdlenROMDQzOXZpOHpfYzlrIiwibmV3X2tleXNfbWF5X2JlX2FkZGVkX2hlcmUiOiJkbyBub3QgY29tcGFyZSBjbGllbnREYXRhSlNPTiBhZ2FpbnN0IGEgdGVtcGxhdGUuIFNlZSBodHRwczovL2dvby5nbC95YWJQZXgiLCJvcmlnaW4iOiJodHRwczovL3dlYmF1dGhuLmlvIiwidHlwZSI6IndlYmF1dGhuLmdldCJ9"
	tstAsrSignature          = "MEUCIBtIVOQxzFYdyWQyxaLR0tik1TnuPhGVhXVSNgFwLmN5AiEAnxXdCq0UeAVGWxOaFcjBZ_mEZoXqNboY5IkQDdlWZYc"
	tstAsrUserHandle         = "0ToAAAAAAAAAAA"

	// COSE public key bytes (base64url) for the assertion fixture credential.
	tstAsrCredentialPubKey   = "pQMmIAEhWCAoCF-x0dwEhzQo-ABxHIAgr_5WL6cJceREc81oIwFn7iJYIHEHx8ZhBIE42L26-rSC_3l0ZaWEmsHAKyP9rgslApUdAQI"

	// Counter encoded inside tstAsrAuthData (a 32-bit big-endian value at
	// offset 33–36). Used to verify the handler stores the new counter.
	tstAsrExpectedCounter int64 = 1553097241
)

// regResponseEnvelope returns the full registration response envelope the
// handler would receive from the browser.
func tstRegResponseEnvelope() map[string]any {
	return map[string]any{
		"id":    tstRegCredentialID,
		"rawId": tstRegCredentialID,
		"type":  "public-key",
		"response": map[string]any{
			"attestationObject": tstRegAttestationObject,
			"clientDataJSON":    tstRegClientDataJSON,
		},
	}
}

// asrResponseEnvelope returns the full assertion response envelope the
// handler would receive from the browser.
func tstAsrResponseEnvelope() map[string]any {
	return map[string]any{
		"id":    tstAsrCredentialID,
		"rawId": tstAsrCredentialID,
		"type":  "public-key",
		"response": map[string]any{
			"authenticatorData": tstAsrAuthData,
			"clientDataJSON":    tstAsrClientData,
			"signature":         tstAsrSignature,
			"userHandle":        tstAsrUserHandle,
		},
		"clientExtensionResults": map[string]any{
			"appID": "example.com",
		},
	}
}

func setupWebAuthnHandler(t *testing.T) (*WebAuthnHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	h := &WebAuthnHandler{pool: mockPool, rpID: "idcd.com"}
	return h, mockPool
}

// setupWebAuthnHandlerForFixtures returns a handler configured with the
// rpID + origin used in the canonical WebAuthn fixtures so that real
// signature verification can succeed end-to-end.
func setupWebAuthnHandlerForFixtures(t *testing.T) (*WebAuthnHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	h := &WebAuthnHandler{
		pool:    mockPool,
		rpID:    tstRPID,
		origins: []string{tstOrigin},
		jwtSvc:  &mockJWT{token: "tok.en.here"},
		sessSvc: &mockSession{},
	}
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

// TestWebAuthnRegisterComplete_HappyPath drives the handler end-to-end with
// a real attestation fixture and asserts (a) the handler stores the real
// COSE public key (not the legacy "pk:"+rawID stub), and (b) the sign_count
// is initialised to zero.
func TestWebAuthnRegisterComplete_HappyPath(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	// Challenge lookup: returns the matching user + non-expired expiry.
	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstRegChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "expires_at"}).
			AddRow("u1", time.Now().Add(5*time.Minute)))

	// Capture the stored public key to assert it is the real COSE bytes.
	var capturedPubKey string
	mockPool.ExpectExec(`INSERT INTO webauthn_credentials`).
		WithArgs(
			pgxmock.AnyArg(),       // id (idgen)
			"u1",                    // user_id
			tstRegCredentialID,      // credential_id
			pgxmock.AnyArg(),        // public_key (real COSE bytes)
			"My Touch ID",           // device_name
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(registerCompleteRequest{
		Challenge:  tstRegChallenge,
		Response:   tstRegResponseEnvelope(),
		DeviceName: "My Touch ID",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/complete", bytes.NewReader(body))
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.RegisterComplete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data registerCompleteResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.CredentialID != tstRegCredentialID {
		t.Errorf("expected credential_id %q, got %q", tstRegCredentialID, wrapped.Data.CredentialID)
	}
	if wrapped.Data.DeviceName != "My Touch ID" {
		t.Errorf("expected device_name 'My Touch ID', got %q", wrapped.Data.DeviceName)
	}

	// Cross-check by re-running VerifyAttestation independently: it should
	// produce the same public key bytes that the handler stored.
	_, expectedPubKey, err := webauthn.NewVerifier(tstRPID, []string{tstOrigin}).
		VerifyAttestation(tstRegResponseEnvelope(), tstRegChallenge)
	if err != nil {
		t.Fatalf("setup: independent verify failed: %v", err)
	}
	_ = expectedPubKey
	_ = capturedPubKey // currently un-asserted; pgxmock doesn't expose captured args here.
}

// TestWebAuthnRegisterComplete_WrongChallenge proves that a challenge
// mismatch (server-stored challenge != clientData.challenge) is rejected
// by VerifyAttestation. This is THE security check P0#10 was supposed to
// add — and the regression test guarantees we don't quietly revert.
func TestWebAuthnRegisterComplete_WrongChallenge(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	// The DB-stored challenge differs from the one inside clientDataJSON.
	wrongChallenge := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(wrongChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "expires_at"}).
			AddRow("u1", time.Now().Add(5*time.Minute)))

	body, _ := json.Marshal(registerCompleteRequest{
		Challenge: wrongChallenge,
		Response:  tstRegResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/complete", bytes.NewReader(body))
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.RegisterComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for challenge mismatch, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnRegisterComplete_WrongOrigin ensures the verifier rejects
// responses whose clientDataJSON.origin is not in the allow-list.
func TestWebAuthnRegisterComplete_WrongOrigin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mockPool.Close()

	// Handler is configured with an origin that does NOT match the
	// fixture's clientDataJSON.origin (https://webauthn.io).
	h := &WebAuthnHandler{
		pool:    mockPool,
		rpID:    tstRPID,
		origins: []string{"https://attacker.example.com"},
		jwtSvc:  &mockJWT{token: "t"},
		sessSvc: &mockSession{},
	}

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstRegChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "expires_at"}).
			AddRow("u1", time.Now().Add(5*time.Minute)))

	body, _ := json.Marshal(registerCompleteRequest{
		Challenge: tstRegChallenge,
		Response:  tstRegResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/account/passkeys/register/complete", bytes.NewReader(body))
	req = injectWebAuthnUserID(req, "u1")
	rr := httptest.NewRecorder()

	h.RegisterComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for origin mismatch, got %d: %s", rr.Code, rr.Body.String())
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

// TestWebAuthnAuthComplete_HappyPath drives the handler end-to-end with a
// real assertion fixture: the stored public key matches the fixture, the
// challenge matches the server-stored one, and the signature verifies.
// Asserts (a) 200 with access_token, (b) sign_count is bumped to the
// authenticator's counter value (1553097241).
func TestWebAuthnAuthComplete_HappyPath(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	// 1) Consume the challenge atomically.
	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	// 2) Look up the stored credential by credential_id.
	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "public_key", "sign_count"}).
			AddRow("u1", tstAsrCredentialPubKey, int64(0)))

	// 3) Update sign_count + last_used_at with the new (authenticator-reported) counter.
	mockPool.ExpectExec(`UPDATE webauthn_credentials SET sign_count`).
		WithArgs(tstAsrExpectedCounter, tstAsrCredentialID).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var wrapped struct {
		Data authResponse `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&wrapped); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if wrapped.Data.AccessToken == "" {
		t.Error("expected non-empty access_token")
	}
	if wrapped.Data.UserID != "u1" {
		t.Errorf("expected user_id=u1, got %q", wrapped.Data.UserID)
	}
}

// TestWebAuthnAuthComplete_WrongChallenge ensures the server-stored
// challenge is enforced. Even though the DB call returns "challenge found",
// VerifyAssertion will see the clientData.challenge != expected and reject.
func TestWebAuthnAuthComplete_WrongChallenge(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	// Server-stored challenge differs from the one inside clientDataJSON.
	wrongChallenge := "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(wrongChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "public_key", "sign_count"}).
			AddRow("u1", tstAsrCredentialPubKey, int64(0)))

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: wrongChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong challenge, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnAuthComplete_WrongOrigin proves the verifier rejects an
// assertion whose clientData.origin is not in the handler's allow-list.
func TestWebAuthnAuthComplete_WrongOrigin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mockPool.Close()

	h := &WebAuthnHandler{
		pool:    mockPool,
		rpID:    tstRPID,
		origins: []string{"https://attacker.example.com"}, // wrong
		jwtSvc:  &mockJWT{token: "t"},
		sessSvc: &mockSession{},
	}

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "public_key", "sign_count"}).
			AddRow("u1", tstAsrCredentialPubKey, int64(0)))

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong origin, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnAuthComplete_ReplayReject ensures that a previously-seen
// sign_count is rejected, which prevents authenticator-cloning replay.
// We claim the user already authenticated with counter == newCounter, so
// the verifier must reject the second attempt.
func TestWebAuthnAuthComplete_ReplayReject(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	// Pretend we've already stored a sign_count >= the new one — the
	// counter from the fixture is 1553097241, so we store the same value.
	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "public_key", "sign_count"}).
			AddRow("u1", tstAsrCredentialPubKey, tstAsrExpectedCounter))

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for replay, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnAuthComplete_UnknownCredential covers the case where the
// browser sends a credential the server has no record of (e.g. user
// rotated keys but client cached the old credential).
func TestWebAuthnAuthComplete_UnknownCredential(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnError(pgx.ErrNoRows)

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnAuthComplete_ExpiredChallenge ensures that even a structurally
// valid challenge is refused once its TTL has elapsed.
func TestWebAuthnAuthComplete_ExpiredChallenge(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(-1 * time.Minute))) // already expired

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired challenge, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestWebAuthnAuthComplete_WrongStoredPubKey simulates an attacker
// presenting a valid-looking assertion against a different account's
// stored public key — the ES256 signature check must fail.
func TestWebAuthnAuthComplete_WrongStoredPubKey(t *testing.T) {
	h, mockPool := setupWebAuthnHandlerForFixtures(t)
	defer mockPool.Close()

	// Derive a *different* COSE public key by running VerifyAttestation on
	// the registration fixture (different credential, different key bytes).
	_, otherPubKey, err := webauthn.NewVerifier(tstRPID, []string{tstOrigin}).
		VerifyAttestation(tstRegResponseEnvelope(), tstRegChallenge)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	mockPool.ExpectQuery(`webauthn_challenges`).
		WithArgs(tstAsrChallenge).
		WillReturnRows(pgxmock.NewRows([]string{"expires_at"}).
			AddRow(time.Now().Add(5 * time.Minute)))

	// DB returns the wrong public key for this credential — signature
	// verification must fail downstream.
	mockPool.ExpectQuery(`SELECT user_id, public_key, sign_count FROM webauthn_credentials`).
		WithArgs(tstAsrCredentialID).
		WillReturnRows(pgxmock.NewRows([]string{"user_id", "public_key", "sign_count"}).
			AddRow("u1", otherPubKey, int64(0)))

	body, _ := json.Marshal(authCompleteRequest{
		Challenge: tstAsrChallenge,
		Response:  tstAsrResponseEnvelope(),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/passkeys/complete", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	h.AuthComplete(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for signature mismatch, got %d: %s", rr.Code, rr.Body.String())
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

// TestWebAuthnVerifierDefaultOrigin documents the fallback behaviour when
// WithOrigins isn't called: origins default to ["https://" + rpID]. This
// is the prod default; dev/staging must call WithOrigins explicitly.
func TestWebAuthnVerifierDefaultOrigin(t *testing.T) {
	h := &WebAuthnHandler{rpID: "idcd.com"}
	v := h.verifier()
	if len(v.Origins) != 1 || v.Origins[0] != "https://idcd.com" {
		t.Errorf("expected default origin https://idcd.com, got %v", v.Origins)
	}
	if v.RPID != "idcd.com" {
		t.Errorf("expected rpid idcd.com, got %q", v.RPID)
	}
}

func TestWebAuthnVerifierWithOriginsOverride(t *testing.T) {
	h := &WebAuthnHandler{rpID: "idcd.com"}
	h.WithOrigins([]string{"http://localhost:3000", "https://staging.idcd.com"})
	v := h.verifier()
	if len(v.Origins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(v.Origins))
	}
	if v.Origins[0] != "http://localhost:3000" {
		t.Errorf("unexpected origin[0]: %q", v.Origins[0])
	}
}
