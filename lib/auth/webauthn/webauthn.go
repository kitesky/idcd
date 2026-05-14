package webauthn

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

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

func GenerateChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate challenge: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

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

func ParseAttestationResponse(response map[string]any) (credentialID, publicKey string, err error) {
	rawID, ok := response["rawId"].(string)
	if !ok || rawID == "" {
		credentialIDVal, ok2 := response["id"].(string)
		if !ok2 || credentialIDVal == "" {
			return "", "", fmt.Errorf("missing credential id")
		}
		rawID = credentialIDVal
	}

	resp, ok := response["response"].(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("missing response field")
	}

	clientDataJSON, ok := resp["clientDataJSON"].(string)
	if !ok || clientDataJSON == "" {
		return "", "", fmt.Errorf("missing clientDataJSON")
	}

	clientDataBytes, err := base64.RawURLEncoding.DecodeString(clientDataJSON)
	if err != nil {
		clientDataBytes, err = base64.StdEncoding.DecodeString(clientDataJSON)
		if err != nil {
			return "", "", fmt.Errorf("decode clientDataJSON: %w", err)
		}
	}

	var clientData map[string]any
	if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
		return "", "", fmt.Errorf("parse clientDataJSON: %w", err)
	}

	typeVal, _ := clientData["type"].(string)
	if typeVal != "webauthn.create" {
		return "", "", fmt.Errorf("unexpected type: %s", typeVal)
	}

	attestationObject, _ := resp["attestationObject"].(string)
	if attestationObject == "" {
		attestationObject = rawID
	}

	pubKey := base64.RawURLEncoding.EncodeToString([]byte("pk:" + rawID))

	return rawID, pubKey, nil
}

func ParseAssertionResponse(response map[string]any) (credentialID string, signCount int64, err error) {
	rawID, ok := response["rawId"].(string)
	if !ok || rawID == "" {
		credentialIDVal, ok2 := response["id"].(string)
		if !ok2 || credentialIDVal == "" {
			return "", 0, fmt.Errorf("missing credential id")
		}
		rawID = credentialIDVal
	}

	resp, ok := response["response"].(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("missing response field")
	}

	clientDataJSON, ok := resp["clientDataJSON"].(string)
	if !ok || clientDataJSON == "" {
		return "", 0, fmt.Errorf("missing clientDataJSON")
	}

	clientDataBytes, err := base64.RawURLEncoding.DecodeString(clientDataJSON)
	if err != nil {
		clientDataBytes, err = base64.StdEncoding.DecodeString(clientDataJSON)
		if err != nil {
			return "", 0, fmt.Errorf("decode clientDataJSON: %w", err)
		}
	}

	var clientData map[string]any
	if err := json.Unmarshal(clientDataBytes, &clientData); err != nil {
		return "", 0, fmt.Errorf("parse clientDataJSON: %w", err)
	}

	typeVal, _ := clientData["type"].(string)
	if typeVal != "webauthn.get" {
		return "", 0, fmt.Errorf("unexpected type: %s", typeVal)
	}

	var count int64
	if sc, ok := response["sign_count"]; ok {
		switch v := sc.(type) {
		case float64:
			count = int64(v)
		case int64:
			count = v
		case int:
			count = int64(v)
		}
	}

	return rawID, count, nil
}
