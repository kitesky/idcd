package webauthn

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestGenerateChallenge_ReturnsBase64URL(t *testing.T) {
	ch, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ch) < 20 {
		t.Errorf("challenge too short: %q", ch)
	}
	if strings.ContainsAny(ch, "+/=") {
		t.Errorf("challenge is not base64url-encoded (contains padding/std chars): %q", ch)
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
	if _, ok := m["challenge"]; !ok {
		t.Error("missing challenge field in JSON")
	}
	if _, ok := m["rp"]; !ok {
		t.Error("missing rp field in JSON")
	}
	if _, ok := m["user"]; !ok {
		t.Error("missing user field in JSON")
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
}

func TestParseAttestationResponse_ValidInput(t *testing.T) {
	clientData := map[string]any{
		"type":      "webauthn.create",
		"challenge": "testchallenge",
		"origin":    "https://idcd.com",
	}
	clientDataBytes, _ := json.Marshal(clientData)
	clientDataB64 := base64.RawURLEncoding.EncodeToString(clientDataBytes)

	input := map[string]any{
		"id":    "credentialId123",
		"rawId": "credentialId123",
		"response": map[string]any{
			"clientDataJSON":    clientDataB64,
			"attestationObject": "mockAttestationObject",
		},
	}

	credID, pubKey, err := ParseAttestationResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID == "" {
		t.Error("expected non-empty credentialID")
	}
	if pubKey == "" {
		t.Error("expected non-empty publicKey")
	}
}

func TestParseAttestationResponse_WrongType(t *testing.T) {
	clientData := map[string]any{
		"type":      "webauthn.get",
		"challenge": "testchallenge",
	}
	clientDataBytes, _ := json.Marshal(clientData)
	clientDataB64 := base64.RawURLEncoding.EncodeToString(clientDataBytes)

	input := map[string]any{
		"id":    "credentialId123",
		"rawId": "credentialId123",
		"response": map[string]any{
			"clientDataJSON":    clientDataB64,
			"attestationObject": "mock",
		},
	}

	_, _, err := ParseAttestationResponse(input)
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestParseAssertionResponse_ValidInput(t *testing.T) {
	clientData := map[string]any{
		"type":      "webauthn.get",
		"challenge": "testchallenge",
		"origin":    "https://idcd.com",
	}
	clientDataBytes, _ := json.Marshal(clientData)
	clientDataB64 := base64.RawURLEncoding.EncodeToString(clientDataBytes)

	input := map[string]any{
		"id":         "credentialId456",
		"rawId":      "credentialId456",
		"sign_count": float64(5),
		"response": map[string]any{
			"clientDataJSON": clientDataB64,
			"signature":      "mockSignature",
		},
	}

	credID, signCount, err := ParseAssertionResponse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if credID == "" {
		t.Error("expected non-empty credentialID")
	}
	if signCount != 5 {
		t.Errorf("expected signCount=5, got %d", signCount)
	}
}

func TestParseAssertionResponse_MissingCredentialID(t *testing.T) {
	clientData := map[string]any{
		"type":      "webauthn.get",
		"challenge": "ch",
	}
	clientDataBytes, _ := json.Marshal(clientData)
	clientDataB64 := base64.RawURLEncoding.EncodeToString(clientDataBytes)

	input := map[string]any{
		"response": map[string]any{
			"clientDataJSON": clientDataB64,
		},
	}

	_, _, err := ParseAssertionResponse(input)
	if err == nil {
		t.Error("expected error for missing credential id")
	}
}
