package webauthn

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Real WebAuthn fixtures borrowed from go-webauthn's protocol test suite.
// They are real registration / assertion responses produced by a hardware
// authenticator (MacOS TouchID against webauthn.io), which means the
// CBOR-encoded attestation object, the COSE public key and the
// SHA-256(authData || clientDataHash) signature are all real and
// cryptographically valid. Using these as our golden vectors gives us
// confidence that the new verifier really does ES256 signature
// verification end-to-end.
//
// Registration fixture (webauthn.create):
//
//	rp.id   = webauthn.io
//	origin  = https://webauthn.io
//	challenge = W8GzFU8pGjhoRbWrLDlamAfq_y4S1CZG1VuoeRLARrE
//
// Assertion fixture (webauthn.get):
//
//	rp.id   = webauthn.io
//	origin  = https://webauthn.io
//	challenge = E4PTcIH_HfX1pC6Sigk1SC9NAlgeztN0439vi8z_c9k
// ---------------------------------------------------------------------------

const (
	regChallenge          = "W8GzFU8pGjhoRbWrLDlamAfq_y4S1CZG1VuoeRLARrE"
	regOrigin             = "https://webauthn.io"
	regRPID               = "webauthn.io"
	regCredentialID       = "6xrtBhJQW6QU4tOaB4rrHaS2Ks0yDDL_q8jDC16DEjZ-VLVf4kCRkvl2xp2D71sTPYns-exsHQHTy3G-zJRK8g"
	regAttestationObjectB64 = "o2NmbXRkbm9uZWdhdHRTdG10oGhhdXRoRGF0YVjEdKbqkhPJnC90siSSsyDPQCYqlMGpUKA5fyklC2CEHvBBAAAAAAAAAAAAAAAAAAAAAAAAAAAAQOsa7QYSUFukFOLTmgeK6x2ktirNMgwy_6vIwwtegxI2flS1X-JAkZL5dsadg-9bEz2J7PnsbB0B08txvsyUSvKlAQIDJiABIVggLKF5xS0_BntttUIrm2Z2tgZ4uQDwllbdIfrrBMABCNciWCDHwin8Zdkr56iSIh0MrB5qZiEzYLQpEOREhMUkY6q4Vw"
	regClientDataJSONB64    = "eyJjaGFsbGVuZ2UiOiJXOEd6RlU4cEdqaG9SYldyTERsYW1BZnFfeTRTMUNaRzFWdW9lUkxBUnJFIiwib3JpZ2luIjoiaHR0cHM6Ly93ZWJhdXRobi5pbyIsInR5cGUiOiJ3ZWJhdXRobi5jcmVhdGUifQ"

	asrChallenge      = "E4PTcIH_HfX1pC6Sigk1SC9NAlgeztN0439vi8z_c9k"
	asrOrigin         = "https://webauthn.io"
	asrRPID           = "webauthn.io"
	asrCredentialID   = "AI7D5q2P0LS-Fal9ZT7CHM2N5BLbUunF92T8b6iYC199bO2kagSuU05-5dZGqb1SP0A0lyTWng"
	asrAuthDataB64    = "dKbqkhPJnC90siSSsyDPQCYqlMGpUKA5fyklC2CEHvBFXJJiGa3OAAI1vMYKZIsLJfHwVQMANwCOw-atj9C0vhWpfWU-whzNjeQS21Lpxfdk_G-omAtffWztpGoErlNOfuXWRqm9Uj9ANJck1p6lAQIDJiABIVggKAhfsdHcBIc0KPgAcRyAIK_-Vi-nCXHkRHPNaCMBZ-4iWCBxB8fGYQSBONi9uvq0gv95dGWlhJrBwCsj_a4LJQKVHQ"
	asrClientDataB64  = "eyJjaGFsbGVuZ2UiOiJFNFBUY0lIX0hmWDFwQzZTaWdrMVNDOU5BbGdlenROMDQzOXZpOHpfYzlrIiwibmV3X2tleXNfbWF5X2JlX2FkZGVkX2hlcmUiOiJkbyBub3QgY29tcGFyZSBjbGllbnREYXRhSlNPTiBhZ2FpbnN0IGEgdGVtcGxhdGUuIFNlZSBodHRwczovL2dvby5nbC95YWJQZXgiLCJvcmlnaW4iOiJodHRwczovL3dlYmF1dGhuLmlvIiwidHlwZSI6IndlYmF1dGhuLmdldCJ9"
	asrSignatureB64   = "MEUCIBtIVOQxzFYdyWQyxaLR0tik1TnuPhGVhXVSNgFwLmN5AiEAnxXdCq0UeAVGWxOaFcjBZ_mEZoXqNboY5IkQDdlWZYc"
	asrUserHandleB64  = "0ToAAAAAAAAAAA"

	// The COSE public key (CBOR) embedded in the assertion's authData,
	// extracted by running ParseAttestationResponse-style decoding on the
	// auth data and then base64url-encoding the credentialPublicKey bytes.
	// We use this directly as the "stored public key" in assertion
	// verification tests.
	asrCredentialPublicKeyB64 = "pQMmIAEhWCAoCF-x0dwEhzQo-ABxHIAgr_5WL6cJceREc81oIwFn7iJYIHEHx8ZhBIE42L26-rSC_3l0ZaWEmsHAKyP9rgslApUdAQI"
)

// regResponse returns a fresh map[string]any that the handler would receive
// for the registration ceremony. Tests may mutate fields on it to construct
// negative cases.
func regResponse() map[string]any {
	return map[string]any{
		"id":    regCredentialID,
		"rawId": regCredentialID,
		"type":  "public-key",
		"response": map[string]any{
			"attestationObject": regAttestationObjectB64,
			"clientDataJSON":    regClientDataJSONB64,
		},
	}
}

// asrResponse returns a fresh map[string]any for the authentication
// ceremony.
func asrResponse() map[string]any {
	return map[string]any{
		"id":    asrCredentialID,
		"rawId": asrCredentialID,
		"type":  "public-key",
		"response": map[string]any{
			"authenticatorData": asrAuthDataB64,
			"clientDataJSON":    asrClientDataB64,
			"signature":         asrSignatureB64,
			"userHandle":        asrUserHandleB64,
		},
		"clientExtensionResults": map[string]any{
			"appID": "example.com",
		},
	}
}

// reencodeClientData base64url-encodes the given clientData map after
// JSON-marshalling. Used to forge invalid clientData payloads in negative
// tests.
func reencodeClientData(t *testing.T, fields map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal client data: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// ---------------------------------------------------------------------------
// Helpers / option builders.
// ---------------------------------------------------------------------------

func TestGenerateChallenge_ReturnsBase64URL(t *testing.T) {
	ch, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ch) < 20 {
		t.Errorf("challenge too short: %q", ch)
	}
	if strings.ContainsAny(ch, "+/=") {
		t.Errorf("challenge not base64url-encoded (contains padding/std chars): %q", ch)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(ch)
	if err != nil {
		t.Fatalf("failed to decode challenge as base64url: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(decoded))
	}
}

func TestGenerateChallenge_IsUnique(t *testing.T) {
	c1, _ := GenerateChallenge()
	c2, _ := GenerateChallenge()
	if c1 == c2 {
		t.Error("two challenges should not be equal")
	}
}

func TestNewCredentialCreationOptions_Fields(t *testing.T) {
	ch := "testchallenge"
	opts := NewCredentialCreationOptions(ch, "idcd.com", "idcd", "u_123", "alice@example.com")

	if opts.Challenge != ch {
		t.Errorf("expected challenge %q, got %q", ch, opts.Challenge)
	}
	if opts.RelyingParty.ID != "idcd.com" {
		t.Errorf("expected rp.id idcd.com, got %q", opts.RelyingParty.ID)
	}
	if opts.RelyingParty.Name != "idcd" {
		t.Errorf("expected rp.name idcd, got %q", opts.RelyingParty.Name)
	}
	if opts.User.Name != "alice@example.com" {
		t.Errorf("expected user.name alice@example.com, got %q", opts.User.Name)
	}
	if len(opts.PubKeyCredParams) == 0 {
		t.Error("expected at least one pubKeyCredParam")
	}
	if opts.Timeout <= 0 {
		t.Error("expected positive timeout")
	}
	if opts.AuthSelection.UserVerification == "" {
		t.Error("expected non-empty userVerification")
	}
	if opts.Attestation != "none" {
		t.Errorf("expected attestation=none, got %q", opts.Attestation)
	}
}

func TestNewCredentialCreationOptions_JSONRoundtrip(t *testing.T) {
	opts := NewCredentialCreationOptions("ch", "example.com", "Example", "uid", "user")
	b, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"challenge", "rp", "user", "pubKeyCredParams", "timeout", "authenticatorSelection", "attestation"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %q field in JSON", k)
		}
	}
}

func TestNewCredentialRequestOptions_Fields(t *testing.T) {
	opts := NewCredentialRequestOptions("ch2", "idcd.com", []string{"credA", "credB"})
	if opts.Challenge != "ch2" {
		t.Errorf("expected challenge ch2, got %q", opts.Challenge)
	}
	if opts.RPID != "idcd.com" {
		t.Errorf("expected rpId idcd.com, got %q", opts.RPID)
	}
	if len(opts.AllowCredentials) != 2 {
		t.Errorf("expected 2 allowCredentials, got %d", len(opts.AllowCredentials))
	}
	if opts.AllowCredentials[0].Type != "public-key" {
		t.Errorf("expected type=public-key, got %q", opts.AllowCredentials[0].Type)
	}
}

// ---------------------------------------------------------------------------
// ParseAttestationResponse — real CBOR parsing & public-key extraction.
// ---------------------------------------------------------------------------

func TestParseAttestationResponse_ValidVector(t *testing.T) {
	credID, pubKey, err := ParseAttestationResponse(regResponse())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID != regCredentialID {
		t.Errorf("expected credID %q, got %q", regCredentialID, credID)
	}
	// The previous implementation returned base64url("pk:"+rawID) which
	// would start with the base64 of "pk:" (`cGs6`). The real key bytes
	// start with the COSE map tag (`pQ` in raw-url-base64 for a 5-entry
	// map). Assert we didn't regress.
	if strings.HasPrefix(pubKey, "cGs6") {
		t.Errorf("public key looks like the old fake 'pk:'+rawID stub: %q", pubKey)
	}
	// Decode and ensure it's non-trivial CBOR (starts with major type 5
	// map, which encodes as 0xa0 ... 0xbf — i.e. byte starts 0xa or 0xb).
	raw, err := base64.RawURLEncoding.DecodeString(pubKey)
	if err != nil {
		t.Fatalf("public key not base64url: %v", err)
	}
	if len(raw) < 30 {
		t.Errorf("public key suspiciously short: %d bytes", len(raw))
	}
	if (raw[0] & 0xe0) != 0xa0 {
		t.Errorf("public key first byte 0x%02x is not a CBOR map tag", raw[0])
	}
}

func TestParseAttestationResponse_WrongCeremonyType(t *testing.T) {
	resp := regResponse()
	r := resp["response"].(map[string]any)
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.get",
		"challenge": regChallenge,
		"origin":    regOrigin,
	})

	_, _, err := ParseAttestationResponse(resp)
	if err == nil {
		t.Fatal("expected error for wrong ceremony type, got nil")
	}
	if !errors.Is(err, ErrUnexpectedType) {
		t.Errorf("expected ErrUnexpectedType, got %v", err)
	}
}

func TestParseAttestationResponse_MissingCredentialID(t *testing.T) {
	resp := regResponse()
	delete(resp, "id")
	delete(resp, "rawId")

	_, _, err := ParseAttestationResponse(resp)
	if err == nil {
		t.Fatal("expected error for missing credential id")
	}
}

func TestParseAttestationResponse_MissingResponse(t *testing.T) {
	resp := regResponse()
	delete(resp, "response")

	_, _, err := ParseAttestationResponse(resp)
	if err == nil {
		t.Fatal("expected error for missing response field")
	}
}

func TestParseAttestationResponse_GarbageAttestationObject(t *testing.T) {
	resp := regResponse()
	r := resp["response"].(map[string]any)
	r["attestationObject"] = base64.RawURLEncoding.EncodeToString([]byte("not-cbor"))

	_, _, err := ParseAttestationResponse(resp)
	if err == nil {
		t.Fatal("expected error for garbage attestationObject")
	}
}

func TestParseAttestationResponse_NilResponse(t *testing.T) {
	_, _, err := ParseAttestationResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

// ---------------------------------------------------------------------------
// ParseAssertionResponse — real CBOR-aware counter parsing.
// ---------------------------------------------------------------------------

func TestParseAssertionResponse_ValidVector(t *testing.T) {
	credID, signCount, err := ParseAssertionResponse(asrResponse())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID != asrCredentialID {
		t.Errorf("expected credID %q, got %q", asrCredentialID, credID)
	}
	// The fixture's AuthenticatorData.Counter is 1553097241.
	if signCount != 1553097241 {
		t.Errorf("expected signCount=1553097241 (from authData), got %d", signCount)
	}
}

func TestParseAssertionResponse_LegacySignCountOverride(t *testing.T) {
	resp := asrResponse()
	resp["sign_count"] = float64(99)

	_, signCount, err := ParseAssertionResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if signCount != 99 {
		t.Errorf("expected legacy override signCount=99, got %d", signCount)
	}
}

func TestParseAssertionResponse_WrongCeremonyType(t *testing.T) {
	resp := asrResponse()
	r := resp["response"].(map[string]any)
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.create",
		"challenge": asrChallenge,
		"origin":    asrOrigin,
	})

	_, _, err := ParseAssertionResponse(resp)
	if err == nil {
		t.Fatal("expected error for wrong ceremony type, got nil")
	}
	if !errors.Is(err, ErrUnexpectedType) {
		t.Errorf("expected ErrUnexpectedType, got %v", err)
	}
}

func TestParseAssertionResponse_MissingCredentialID(t *testing.T) {
	resp := asrResponse()
	delete(resp, "id")
	delete(resp, "rawId")

	_, _, err := ParseAssertionResponse(resp)
	if err == nil {
		t.Fatal("expected error for missing credential id")
	}
}

func TestParseAssertionResponse_NilResponse(t *testing.T) {
	_, _, err := ParseAssertionResponse(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

// ---------------------------------------------------------------------------
// Verifier.VerifyAttestation — the real registration verification path.
// ---------------------------------------------------------------------------

func newRegVerifier() *Verifier {
	return NewVerifier(regRPID, []string{regOrigin})
}

func TestVerifyAttestation_HappyPath(t *testing.T) {
	v := newRegVerifier()
	credID, pubKey, err := v.VerifyAttestation(regResponse(), regChallenge)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID != regCredentialID {
		t.Errorf("expected credID %q, got %q", regCredentialID, credID)
	}
	if pubKey == "" {
		t.Error("expected non-empty public key")
	}
	// Make sure we can decode the returned public key and that it parses as
	// COSE — that's exactly what the assertion verification will need.
	if _, err := base64.RawURLEncoding.DecodeString(pubKey); err != nil {
		t.Errorf("returned pubKey is not base64url: %v", err)
	}
}

func TestVerifyAttestation_WrongChallenge(t *testing.T) {
	v := newRegVerifier()
	_, _, err := v.VerifyAttestation(regResponse(), "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if err == nil {
		t.Fatal("expected error for wrong challenge")
	}
}

func TestVerifyAttestation_WrongOrigin(t *testing.T) {
	v := NewVerifier(regRPID, []string{"https://attacker.example.com"})
	_, _, err := v.VerifyAttestation(regResponse(), regChallenge)
	if err == nil {
		t.Fatal("expected error for wrong origin")
	}
}

func TestVerifyAttestation_WrongRPID(t *testing.T) {
	v := NewVerifier("attacker.example.com", []string{regOrigin})
	_, _, err := v.VerifyAttestation(regResponse(), regChallenge)
	if err == nil {
		t.Fatal("expected error for wrong RPID")
	}
}

func TestVerifyAttestation_TamperedClientData(t *testing.T) {
	resp := regResponse()
	r := resp["response"].(map[string]any)
	// Re-sign-with-wrong-fields the client data: keep the right type but
	// change the challenge.
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.create",
		"challenge": "tampered_challenge_does_not_match",
		"origin":    regOrigin,
	})

	v := newRegVerifier()
	_, _, err := v.VerifyAttestation(resp, regChallenge)
	if err == nil {
		t.Fatal("expected error for tampered clientDataJSON")
	}
}

func TestVerifyAttestation_MalformedClientDataJSON(t *testing.T) {
	resp := regResponse()
	r := resp["response"].(map[string]any)
	r["clientDataJSON"] = base64.RawURLEncoding.EncodeToString([]byte("not-json"))

	v := newRegVerifier()
	_, _, err := v.VerifyAttestation(resp, regChallenge)
	if err == nil {
		t.Fatal("expected error for malformed clientDataJSON")
	}
}

// ---------------------------------------------------------------------------
// Verifier.VerifyAssertion — the real signature-check authentication path.
// ---------------------------------------------------------------------------

func newAsrVerifier() *Verifier {
	return NewVerifier(asrRPID, []string{asrOrigin})
}

func TestVerifyAssertion_HappyPath(t *testing.T) {
	v := newAsrVerifier()
	credID, signCount, err := v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID != asrCredentialID {
		t.Errorf("expected credID %q, got %q", asrCredentialID, credID)
	}
	if signCount != 1553097241 {
		t.Errorf("expected signCount=1553097241, got %d", signCount)
	}
}

func TestVerifyAssertion_WrongChallenge(t *testing.T) {
	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, "wrong_challenge_AAAAAAAAAAAAAAAAAAAAA", 0)
	if err == nil {
		t.Fatal("expected error for wrong challenge")
	}
}

func TestVerifyAssertion_WrongOrigin(t *testing.T) {
	v := NewVerifier(asrRPID, []string{"https://attacker.example.com"})
	_, _, err := v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for wrong origin")
	}
}

func TestVerifyAssertion_WrongRPID(t *testing.T) {
	v := NewVerifier("attacker.example.com", []string{asrOrigin})
	_, _, err := v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for wrong RPID")
	}
}

func TestVerifyAssertion_WrongPublicKey(t *testing.T) {
	v := newAsrVerifier()
	// Swap in the *registration* fixture's public key — different
	// credential, so the signature must fail to verify.
	regResp := regResponse()
	regCredID, regPubKey, err := ParseAttestationResponse(regResp)
	if err != nil || regCredID == "" {
		t.Fatalf("setup parse failed: %v", err)
	}

	_, _, err = v.VerifyAssertion(asrResponse(), regPubKey, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for wrong public key (signature mismatch)")
	}
}

func TestVerifyAssertion_TamperedSignature(t *testing.T) {
	resp := asrResponse()
	r := resp["response"].(map[string]any)
	// Flip the last byte of the signature.
	sig, _ := base64.RawURLEncoding.DecodeString(asrSignatureB64)
	sig[len(sig)-1] ^= 0xFF
	r["signature"] = base64.RawURLEncoding.EncodeToString(sig)

	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(resp, asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for tampered signature")
	}
}

func TestVerifyAssertion_TamperedAuthData(t *testing.T) {
	resp := asrResponse()
	r := resp["response"].(map[string]any)
	authData, _ := base64.RawURLEncoding.DecodeString(asrAuthDataB64)
	// Bump the counter bytes (offset 33-36 in authData) — but leave the
	// signature alone. The signature was produced over the original
	// authData so the verify must fail.
	authData[33] ^= 0xFF
	r["authenticatorData"] = base64.RawURLEncoding.EncodeToString(authData)

	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(resp, asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for tampered authenticator data")
	}
}

func TestVerifyAssertion_TamperedClientData(t *testing.T) {
	resp := asrResponse()
	r := resp["response"].(map[string]any)
	// Re-encode clientData with a different challenge — origin & type still
	// look correct so the early Verify checks may pass, but the signature
	// is now over different bytes and must fail.
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.get",
		"challenge": asrChallenge,
		"origin":    asrOrigin,
		"extra":     "tamper",
	})

	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(resp, asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for tampered clientDataJSON")
	}
}

func TestVerifyAssertion_ReplayProtection(t *testing.T) {
	v := newAsrVerifier()
	// First call with lastSignCount=0 should succeed and return 1553097241.
	_, sc, err := v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, 0)
	if err != nil {
		t.Fatalf("first verify unexpected error: %v", err)
	}

	// Replay: claim we've already seen `sc` (or higher) — must reject.
	_, _, err = v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, sc)
	if err == nil {
		t.Fatal("expected replay rejection when lastSignCount == newSignCount")
	}
	if !strings.Contains(err.Error(), "replay") {
		t.Errorf("expected error mentioning replay, got %v", err)
	}

	_, _, err = v.VerifyAssertion(asrResponse(), asrCredentialPublicKeyB64, asrChallenge, sc+10)
	if err == nil {
		t.Fatal("expected replay rejection when lastSignCount > newSignCount")
	}
}

func TestVerifyAssertion_MalformedPublicKey(t *testing.T) {
	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(asrResponse(), "not-valid-base64!!!", asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for non-base64 public key")
	}
}

func TestVerifyAssertion_NilResponse(t *testing.T) {
	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(nil, asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestVerifyAssertion_WrongCeremonyType(t *testing.T) {
	resp := asrResponse()
	r := resp["response"].(map[string]any)
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.create",
		"challenge": asrChallenge,
		"origin":    asrOrigin,
	})

	v := newAsrVerifier()
	_, _, err := v.VerifyAssertion(resp, asrCredentialPublicKeyB64, asrChallenge, 0)
	if err == nil {
		t.Fatal("expected error for wrong ceremony type")
	}
	if !errors.Is(err, ErrUnexpectedType) {
		t.Errorf("expected ErrUnexpectedType, got %v", err)
	}
}

func TestVerifyAssertion_StdBase64PublicKey(t *testing.T) {
	// Some legacy callers may have stored the public key in standard
	// base64 (with padding). VerifyAssertion should accept either form.
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(asrCredentialPublicKeyB64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	stdEncoded := base64.StdEncoding.EncodeToString(pubKeyBytes)

	v := newAsrVerifier()
	credID, _, err := v.VerifyAssertion(asrResponse(), stdEncoded, asrChallenge, 0)
	if err != nil {
		t.Fatalf("std-encoded pubKey should be accepted, got: %v", err)
	}
	if credID != asrCredentialID {
		t.Errorf("expected credID %q, got %q", asrCredentialID, credID)
	}
}

func TestVerifyAttestation_NilResponse(t *testing.T) {
	v := newRegVerifier()
	_, _, err := v.VerifyAttestation(nil, regChallenge)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestVerifyAttestation_WrongCeremonyType(t *testing.T) {
	resp := regResponse()
	r := resp["response"].(map[string]any)
	r["clientDataJSON"] = reencodeClientData(t, map[string]any{
		"type":      "webauthn.get",
		"challenge": regChallenge,
		"origin":    regOrigin,
	})

	v := newRegVerifier()
	_, _, err := v.VerifyAttestation(resp, regChallenge)
	if err == nil {
		t.Fatal("expected error for wrong ceremony type")
	}
	if !errors.Is(err, ErrUnexpectedType) {
		t.Errorf("expected ErrUnexpectedType, got %v", err)
	}
}

func TestParseAssertionResponse_SignCountIntTypes(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
		want int64
	}{
		{"int64", int64(7), 7},
		{"int", int(8), 8},
		{"float64", float64(9), 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := asrResponse()
			resp["sign_count"] = tc.v
			_, sc, err := ParseAssertionResponse(resp)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc != tc.want {
				t.Errorf("expected sc=%d, got %d", tc.want, sc)
			}
		})
	}
}

func TestNormaliseEnvelope_AddsTypeWhenMissing(t *testing.T) {
	resp := regResponse()
	delete(resp, "type")
	credID, _, err := ParseAttestationResponse(resp)
	if err != nil {
		t.Fatalf("envelope without type should default to public-key: %v", err)
	}
	if credID != regCredentialID {
		t.Errorf("expected credID %q, got %q", regCredentialID, credID)
	}
}

// ---------------------------------------------------------------------------
// Algorithm whitelist.
// ---------------------------------------------------------------------------

func TestCheckSupportedAlgorithm_RejectsUnknownAlg(t *testing.T) {
	// Hand-build a COSE EC2 key with an alg not in our allow-list (alg=-99
	// is not a defined identifier and ParsePublicKey will reject this).
	// We can't easily forge a structurally-valid alg=-99 COSE key without
	// the library's blessing, so use a manifestly invalid byte string and
	// assert we get an error.
	err := checkSupportedAlgorithm([]byte{0xff, 0xff, 0xff})
	if err == nil {
		t.Fatal("expected error for garbage COSE bytes")
	}
}

func TestCheckSupportedAlgorithm_AcceptsES256(t *testing.T) {
	// Re-use the real ES256 public key from the registration fixture.
	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(asrCredentialPublicKeyB64)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := checkSupportedAlgorithm(pubKeyBytes); err != nil {
		t.Fatalf("expected ES256 to be supported, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// normaliseEnvelope quirks: tests fill in either id or rawId only.
// ---------------------------------------------------------------------------

func TestParseAttestationResponse_OnlyRawIDPresent(t *testing.T) {
	resp := regResponse()
	delete(resp, "id")

	credID, _, err := ParseAttestationResponse(resp)
	if err != nil {
		t.Fatalf("expected ParseAttestationResponse to tolerate id-from-rawId, got %v", err)
	}
	if credID != regCredentialID {
		t.Errorf("expected credID %q, got %q", regCredentialID, credID)
	}
}

func TestParseAttestationResponse_OnlyIDPresent(t *testing.T) {
	resp := regResponse()
	delete(resp, "rawId")

	credID, _, err := ParseAttestationResponse(resp)
	if err != nil {
		t.Fatalf("expected ParseAttestationResponse to tolerate rawId-from-id, got %v", err)
	}
	if credID != regCredentialID {
		t.Errorf("expected credID %q, got %q", regCredentialID, credID)
	}
}
