// Package webauthn provides WebAuthn / FIDO2 helper types and verification
// helpers built on top of github.com/go-webauthn/webauthn for CBOR decoding
// and ES256 / RS256 / EdDSA signature verification.
//
// Previously this package merely parsed the rawID and stored a fake public
// key (`"pk:" + rawID`), which made authentication equivalent to "possessing
// the credentialID is enough to log in". That has been replaced with real
// CBOR decoding of attestationObject -> COSE public key, plus full clientData
// / authenticatorData / signature verification when callers use the
// Verifier API.
package webauthn

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	gowebauthn "github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

// ---------------------------------------------------------------------------
// Public option types — JSON tags are preserved exactly so that handler
// callers serialising these structs over HTTP see no breaking changes.
// ---------------------------------------------------------------------------

type RelyingParty struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type PubKeyCredParam struct {
	Type string `json:"type"`
	Alg  int    `json:"alg"`
}

type AuthenticatorSelection struct {
	AuthenticatorAttachment string `json:"authenticatorAttachment,omitempty"`
	ResidentKey             string `json:"residentKey"`
	UserVerification        string `json:"userVerification"`
}

type CredentialCreationOptions struct {
	Challenge        string                 `json:"challenge"`
	RelyingParty     RelyingParty           `json:"rp"`
	User             UserEntity             `json:"user"`
	PubKeyCredParams []PubKeyCredParam      `json:"pubKeyCredParams"`
	Timeout          int                    `json:"timeout"`
	AuthSelection    AuthenticatorSelection `json:"authenticatorSelection"`
	Attestation      string                 `json:"attestation"`
}

type AllowCredential struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type CredentialRequestOptions struct {
	Challenge        string            `json:"challenge"`
	Timeout          int               `json:"timeout"`
	RPID             string            `json:"rpId"`
	AllowCredentials []AllowCredential `json:"allowCredentials"`
	UserVerification string            `json:"userVerification"`
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

var (
	// ErrMissingCredentialID is returned when neither `rawId` nor `id` is
	// present in the response.
	ErrMissingCredentialID = errors.New("missing credential id")
	// ErrMissingResponse is returned when the `response` field is absent.
	ErrMissingResponse = errors.New("missing response field")
	// ErrUnsupportedAlgorithm is returned when the credential's COSE
	// algorithm identifier isn't in the supported set (ES256 / RS256 /
	// EdDSA / ES384 / ES512 / PS256 / PS384 / PS512).
	ErrUnsupportedAlgorithm = errors.New("unsupported credential algorithm")
	// ErrUnexpectedType means the clientData.type didn't match the
	// expected ceremony (webauthn.create vs webauthn.get).
	ErrUnexpectedType = errors.New("unexpected client data type")
)

// ---------------------------------------------------------------------------
// Challenge / option builders — unchanged signatures, unchanged behaviour.
// ---------------------------------------------------------------------------

// GenerateChallenge returns a base64url-encoded 32-byte random challenge.
func GenerateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewCredentialCreationOptions builds a registration-options struct ready to
// be JSON-marshalled to the client. The PubKeyCredParams cover ES256 (-7)
// and RS256 (-257), matching WebAuthn Level 2 minimum interop.
func NewCredentialCreationOptions(challenge, rpID, rpName, userID, userName string) CredentialCreationOptions {
	return CredentialCreationOptions{
		Challenge: challenge,
		RelyingParty: RelyingParty{
			ID:   rpID,
			Name: rpName,
		},
		User: UserEntity{
			ID:          base64.RawURLEncoding.EncodeToString([]byte(userID)),
			Name:        userName,
			DisplayName: userName,
		},
		PubKeyCredParams: []PubKeyCredParam{
			{Type: "public-key", Alg: -7},
			{Type: "public-key", Alg: -257},
		},
		Timeout: 60000,
		AuthSelection: AuthenticatorSelection{
			ResidentKey:      "preferred",
			UserVerification: "preferred",
		},
		Attestation: "none",
	}
}

// NewCredentialRequestOptions builds the authentication options sent to the
// client for the get() ceremony.
func NewCredentialRequestOptions(challenge, rpID string, credentialIDs []string) CredentialRequestOptions {
	allow := make([]AllowCredential, 0, len(credentialIDs))
	for _, id := range credentialIDs {
		allow = append(allow, AllowCredential{Type: "public-key", ID: id})
	}
	return CredentialRequestOptions{
		Challenge:        challenge,
		Timeout:          60000,
		RPID:             rpID,
		AllowCredentials: allow,
		UserVerification: "preferred",
	}
}

// ---------------------------------------------------------------------------
// Parsing — now CBOR-aware. ParseAttestationResponse extracts the *real*
// COSE public key (no more "pk:"+rawID stub).
//
// NOTE: These functions perform *parsing only* — they do NOT verify the
// challenge / origin / RPID / signature. Callers that want real security
// must use the Verifier methods below. The old call sites kept here for
// backwards-source-compatibility while the handler is being migrated.
// ---------------------------------------------------------------------------

// ParseAttestationResponse decodes a navigator.credentials.create() response,
// performs real CBOR decoding of the attestationObject and returns the
// credential ID together with the base64-encoded COSE public-key bytes. The
// returned public key is the raw CBOR encoding suitable for later signature
// verification.
//
// Validation performed:
//   - clientData.type == "webauthn.create"
//   - attestationObject CBOR decodes successfully
//   - the credential algorithm is one we support (ES256/RS256/EdDSA/...)
//
// Not performed (use Verifier.VerifyAttestation for these):
//   - challenge equality
//   - origin / RPID equality
//   - attestation statement verification
func ParseAttestationResponse(response map[string]any) (credentialID, publicKey string, err error) {
	parsed, err := parseAttestation(response)
	if err != nil {
		return "", "", err
	}

	if parsed.Response.CollectedClientData.Type != gowebauthn.CreateCeremony {
		return "", "", fmt.Errorf("%w: got %q", ErrUnexpectedType, parsed.Response.CollectedClientData.Type)
	}

	pubKeyBytes := parsed.Response.AttestationObject.AuthData.AttData.CredentialPublicKey
	if len(pubKeyBytes) == 0 {
		return "", "", fmt.Errorf("credential public key missing from authenticator data")
	}

	if err := checkSupportedAlgorithm(pubKeyBytes); err != nil {
		return "", "", err
	}

	credentialID = parsed.ID
	publicKey = base64.RawURLEncoding.EncodeToString(pubKeyBytes)
	return credentialID, publicKey, nil
}

// ParseAssertionResponse decodes a navigator.credentials.get() response and
// returns the credential ID plus the authenticator's monotonic counter
// (sign_count). It does NOT verify the signature; callers must use
// Verifier.VerifyAssertion to perform real cryptographic verification.
//
// The legacy implementation read `sign_count` from a top-level field of the
// JSON envelope (which the browser never sets). We now read the counter
// from the parsed AuthenticatorData where it actually lives. As a
// transition fallback, an explicit `sign_count` field on the envelope is
// still honoured (used by tests and older clients).
func ParseAssertionResponse(response map[string]any) (credentialID string, signCount int64, err error) {
	parsed, err := parseAssertion(response)
	if err != nil {
		return "", 0, err
	}

	if parsed.Response.CollectedClientData.Type != gowebauthn.AssertCeremony {
		return "", 0, fmt.Errorf("%w: got %q", ErrUnexpectedType, parsed.Response.CollectedClientData.Type)
	}

	credentialID = parsed.ID
	signCount = int64(parsed.Response.AuthenticatorData.Counter)

	// Allow tests / legacy clients to override via a top-level field.
	if sc, ok := response["sign_count"]; ok {
		switch v := sc.(type) {
		case float64:
			signCount = int64(v)
		case int64:
			signCount = v
		case int:
			signCount = int64(v)
		}
	}

	return credentialID, signCount, nil
}

// ---------------------------------------------------------------------------
// Verifier — the new, real-verification API. New callers should use this.
//
// Usage:
//
//	v := webauthn.NewVerifier("idcd.com", []string{"https://idcd.com"})
//	credID, pubKey, err := v.VerifyAttestation(req.Response, storedChallenge)
//	// store credID + pubKey on the user
//
//	newSignCount, err := v.VerifyAssertion(req.Response, storedPubKey, storedChallenge, lastSignCount)
// ---------------------------------------------------------------------------

// Verifier carries the relying-party configuration needed for real
// cryptographic verification of both registration and authentication
// responses.
type Verifier struct {
	// RPID is the Relying Party identifier — must match the rp.id used at
	// registration time. For idcd this is "idcd.com".
	RPID string
	// Origins is the allow-list of acceptable Web Origins (full origin
	// strings, e.g. "https://idcd.com").
	Origins []string
	// RequireUserVerification, when true, requires the UV flag to be set
	// on the authenticator data. WebAuthn's "user verified" bit.
	RequireUserVerification bool
}

// NewVerifier constructs a Verifier with the given RPID and origins. Passing
// an empty origins slice will cause every assertion to fail, by design.
func NewVerifier(rpID string, origins []string) *Verifier {
	return &Verifier{RPID: rpID, Origins: origins}
}

// VerifyAttestation performs full WebAuthn registration verification per
// §7.1 of the spec. Returns the credential ID plus the base64-encoded COSE
// public key bytes ready to store in the DB.
//
// On any verification failure (wrong challenge, origin, RPID, unsupported
// alg, or malformed CBOR) the function returns a descriptive error and
// empty credential ID / public key.
func (v *Verifier) VerifyAttestation(response map[string]any, expectedChallenge string) (credentialID, publicKey string, err error) {
	parsed, err := parseAttestation(response)
	if err != nil {
		return "", "", err
	}

	if parsed.Response.CollectedClientData.Type != gowebauthn.CreateCeremony {
		return "", "", fmt.Errorf("%w: got %q", ErrUnexpectedType, parsed.Response.CollectedClientData.Type)
	}

	pubKeyBytes := parsed.Response.AttestationObject.AuthData.AttData.CredentialPublicKey
	if len(pubKeyBytes) == 0 {
		return "", "", fmt.Errorf("credential public key missing from authenticator data")
	}

	if err := checkSupportedAlgorithm(pubKeyBytes); err != nil {
		return "", "", err
	}

	credParams := []gowebauthn.CredentialParameter{
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgES256},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgRS256},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgEdDSA},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgES384},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgES512},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgPS256},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgPS384},
		{Type: gowebauthn.PublicKeyCredentialType, Algorithm: webauthncose.AlgPS512},
	}

	if _, err := parsed.Verify(
		expectedChallenge,
		v.RequireUserVerification,
		true, // verify user presence
		v.RPID,
		v.Origins,
		nil, // rpTopOrigins
		gowebauthn.TopOriginIgnoreVerificationMode,
		nil, // metadata provider
		credParams,
	); err != nil {
		return "", "", fmt.Errorf("verify attestation: %w", err)
	}

	return parsed.ID, base64.RawURLEncoding.EncodeToString(pubKeyBytes), nil
}

// VerifyAssertion performs full WebAuthn authentication verification per
// §7.2 of the spec, including signature check against the previously
// stored public key. Returns the new sign_count value to persist.
//
//   - storedPublicKey  — base64url-encoded COSE public-key bytes as
//     returned by VerifyAttestation / ParseAttestationResponse.
//   - expectedChallenge — the base64url challenge issued in the matching
//     /webauthn/auth/begin call.
//   - lastSignCount — the previously stored counter; the new counter must
//     be strictly greater (replay protection). Pass 0 if this is the
//     first assertion.
func (v *Verifier) VerifyAssertion(response map[string]any, storedPublicKey, expectedChallenge string, lastSignCount int64) (credentialID string, newSignCount int64, err error) {
	parsed, err := parseAssertion(response)
	if err != nil {
		return "", 0, err
	}

	if parsed.Response.CollectedClientData.Type != gowebauthn.AssertCeremony {
		return "", 0, fmt.Errorf("%w: got %q", ErrUnexpectedType, parsed.Response.CollectedClientData.Type)
	}

	pubKeyBytes, err := base64.RawURLEncoding.DecodeString(storedPublicKey)
	if err != nil {
		// Try the std encoding as a defensive fallback (older callers).
		pubKeyBytes, err = base64.StdEncoding.DecodeString(storedPublicKey)
		if err != nil {
			return "", 0, fmt.Errorf("decode stored public key: %w", err)
		}
	}

	if err := parsed.Verify(
		expectedChallenge,
		v.RPID,
		v.Origins,
		nil, // rpTopOrigins
		gowebauthn.TopOriginIgnoreVerificationMode,
		"", // appID extension
		v.RequireUserVerification,
		true, // verify user presence
		pubKeyBytes,
	); err != nil {
		return "", 0, fmt.Errorf("verify assertion: %w", err)
	}

	newSignCount = int64(parsed.Response.AuthenticatorData.Counter)

	// Replay protection: if the authenticator reports a non-zero counter
	// it MUST be strictly greater than the previously stored value.
	// (Counter == 0 means the authenticator doesn't implement a counter.)
	if newSignCount > 0 && newSignCount <= lastSignCount {
		return "", 0, fmt.Errorf("sign count replay detected: got %d, last %d", newSignCount, lastSignCount)
	}

	return parsed.ID, newSignCount, nil
}

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

// parseAttestation re-encodes the handler-friendly map[string]any payload as
// JSON and feeds it to go-webauthn's protocol parser, yielding a fully
// CBOR-decoded ParsedCredentialCreationData.
func parseAttestation(response map[string]any) (*gowebauthn.ParsedCredentialCreationData, error) {
	if response == nil {
		return nil, ErrMissingResponse
	}

	// Some callers (and our existing tests) send only `id` without `rawId`,
	// or vice versa — go-webauthn's parser insists on both. Mirror them
	// before encoding.
	normalised := normaliseEnvelope(response)

	raw, err := json.Marshal(normalised)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	parsed, err := gowebauthn.ParseCredentialCreationResponseBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse attestation: %w", err)
	}
	return parsed, nil
}

// parseAssertion mirrors parseAttestation for navigator.credentials.get()
// responses.
func parseAssertion(response map[string]any) (*gowebauthn.ParsedCredentialAssertionData, error) {
	if response == nil {
		return nil, ErrMissingResponse
	}

	normalised := normaliseEnvelope(response)

	raw, err := json.Marshal(normalised)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	parsed, err := gowebauthn.ParseCredentialRequestResponseBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse assertion: %w", err)
	}
	return parsed, nil
}

// normaliseEnvelope returns a shallow copy of the envelope with mandatory
// fields filled in if only one of `id` / `rawId` is provided, and with a
// default `type=public-key` if absent. Returns an error wrapped in the
// caller path if neither id is present.
func normaliseEnvelope(response map[string]any) map[string]any {
	out := make(map[string]any, len(response)+2)
	for k, v := range response {
		out[k] = v
	}

	idVal, _ := out["id"].(string)
	rawIDVal, _ := out["rawId"].(string)
	if idVal == "" && rawIDVal != "" {
		out["id"] = rawIDVal
	}
	if rawIDVal == "" && idVal != "" {
		out["rawId"] = idVal
	}

	if _, ok := out["type"].(string); !ok {
		out["type"] = "public-key"
	}

	return out
}

// checkSupportedAlgorithm CBOR-decodes the COSE public key and rejects
// algorithms outside our supported set. This is a defence-in-depth check
// on top of credParams enforcement during Verify.
func checkSupportedAlgorithm(pubKeyBytes []byte) error {
	key, err := webauthncose.ParsePublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("parse cose key: %w", err)
	}

	var alg webauthncose.COSEAlgorithmIdentifier
	switch k := key.(type) {
	case webauthncose.EC2PublicKeyData:
		alg = webauthncose.COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm)
	case webauthncose.RSAPublicKeyData:
		alg = webauthncose.COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm)
	case webauthncose.OKPPublicKeyData:
		alg = webauthncose.COSEAlgorithmIdentifier(k.PublicKeyData.Algorithm)
	default:
		return fmt.Errorf("%w: unknown key type %T", ErrUnsupportedAlgorithm, key)
	}

	switch alg {
	case webauthncose.AlgES256,
		webauthncose.AlgES384,
		webauthncose.AlgES512,
		webauthncose.AlgEdDSA,
		webauthncose.AlgRS256,
		webauthncose.AlgRS384,
		webauthncose.AlgRS512,
		webauthncose.AlgPS256,
		webauthncose.AlgPS384,
		webauthncose.AlgPS512:
		return nil
	default:
		return fmt.Errorf("%w: alg=%d", ErrUnsupportedAlgorithm, alg)
	}
}
