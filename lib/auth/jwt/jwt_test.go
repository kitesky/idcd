package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kite365/idcd/packages/shared/apperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSecret = "this-is-a-very-long-secret-key-for-testing-purposes-that-meets-minimum-length"

	// Test RSA keys (2048-bit, generated for testing only)
	testPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCp1lZ0TTrY4tBG
/8mnKmEbRxruiRldXWDQp7QKNM8t8qIp4nL2s5bnREHQ1lHwBolnZIScz3bqTAbE
mFuozN2imHCYWWjlazLJ1LFiyh72YZifBTw+9Cu5G8zhvUE2Q8M/NqyYpIWN91qg
g+lXK3XZGvPYF3GBbJnJtcZPisnC9tcwsAWmhZHRMSdv+Mykv2+N3H7+mrPzmO9K
6FkHQcUpZz4LPfwrHKb7pV8Y2qlTaLzwjIGUZ02TzNeKZdYOIwPmcGaTEc4PhXGC
tb9ug/hrM2a9K0gUgxQs4Y83MjdoF10/ipDbk+2LbTFjcxSN0/uEZtInJNGgqoeO
3foiZao1AgMBAAECggEACyp/HR7dvVguKtjS1HV+FGnInMGxQ/jpblZ0SQ5/R41R
+ZB8j/kvNO3BKP6KPQ5k9sH+UQP31MAiWscMkazEbpX4ox+PvLOfT3M3JWBWCEtS
2jOhKh9qB33LKtVDRhLO8adB0jhQ2oxWbkK18uf108wIl5609PYjp0YW73BXwwRV
pvMHmIQxeFl2Z2aHbwaIq4Vh3l46BTEimWEdRj87+vX3QHXrkjoXqawnOlIQj2T5
afVH6YgF7rOVHNuGpcDKe4HnUr2N+ea6MC+0DQsS0tE9fpJocMuhPf2cmMFi6C2v
CM4er44nXfgMGcMvNQeX9s2V23coRLsTzrRDJlMDcQKBgQDRUuhmwcHl1E8O9OHm
iSjmwydm0cha7j+i4MTvFRdfr77RgT/azV76JbXCYmc2Z5wySCYjH7FaGmJQA2xS
87NWyNAFsOPsUq9H/ooNvWUwXMDXuM53iIC66rVLAHGtfCfzQBBNSqZrSGMVksZF
9Bchel8VYhH95x5Uxkj0oMC8UQKBgQDPtV4Rrb6jJb/IqpCxSRYR134/PWCuObK6
nKCmPpnnJkYJYae9klefkhTPrTWmjeSlOEn6DrNl/wmMD6X1/krijkRuNqDskYbL
bK3L8C5IFc+ggsG6gdDD9XaaXz5ly0gmqu/rRDvdmoRN4fouQloAh9244AvvnIh3
xrELPBcqpQKBgFfZtOHTdb4wcZG0Ys6vR/Q5eWLkrnLDRP/l16EDuBCXoL0qwpLg
2HihtPvE8s5Zg6tyrlbVaUiIhDRSi3bxApZspymMSMwZE6ligawsjbhTZTfkPvrZ
1jUcZkP5Brypu9aST4UwzFGASt12ATLAs6iAREGkLCrkgc1QfrP0d49RAoGAM4e4
bcRgDlO4L815FjKeohCHRqMwkCjKWZewF25iekE5kOxEVDixOmpgdWFwdQCw3/iG
Cd6JzV0nfjMHpm7PH0PSYFF3PRmhimhM+dJ9eO7IUvb9nwrDw0nrgcLtVQ3Iuacg
3IpSG9lQx42vprhZYdZTQKF89JYuGSEXHUVsLe0CgYAqaO/e11Hfz9f+SZyBZDpy
PvW/8TP5/KU8vrHdRG8lDy094W8c3gRIitGyK7gwTp/AtD7uwGpXIenZolmbQbzS
/lFHc6qDNjLzm//mmg9a6s9X7xC6BhddXNc9XSofZQl5kvxRnHLGxZxZXuxe0j0H
FQrMdOCR6iobFREjCTlTsA==
-----END PRIVATE KEY-----`

	testPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAqdZWdE062OLQRv/Jpyph
G0ca7okZXV1g0Ke0CjTPLfKiKeJy9rOW50RB0NZR8AaJZ2SEnM926kwGxJhbqMzd
ophwmFlo5WsyydSxYsoe9mGYnwU8PvQruRvM4b1BNkPDPzasmKSFjfdaoIPpVyt1
2Rrz2BdxgWyZybXGT4rJwvbXMLAFpoWR0TEnb/jMpL9vjdx+/pqz85jvSuhZB0HF
KWc+Cz38Kxym+6VfGNqpU2i88IyBlGdNk8zXimXWDiMD5nBmkxHOD4VxgrW/boP4
azNmvStIFIMULOGPNzI3aBddP4qQ25Pti20xY3MUjdP7hGbSJyTRoKqHjt36ImWq
NQIDAQAB
-----END PUBLIC KEY-----`
)

func TestNewService(t *testing.T) {
	t.Run("with HMAC secret", func(t *testing.T) {
		config := Config{SecretKey: testSecret}
		service, err := NewService(config)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, testSecret, service.secretKey)
	})

	t.Run("with RSA keys", func(t *testing.T) {
		config := Config{
			PrivateKey: testPrivateKey,
			PublicKey:  testPublicKey,
		}
		service, err := NewService(config)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.NotNil(t, service.privateKey)
		assert.NotNil(t, service.publicKey)
	})

	t.Run("with short secret", func(t *testing.T) {
		config := Config{SecretKey: "short"}
		_, err := NewService(config)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("with no keys", func(t *testing.T) {
		config := Config{}
		_, err := NewService(config)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("with invalid RSA private key", func(t *testing.T) {
		config := Config{
			PrivateKey: "invalid-key",
			PublicKey:  testPublicKey,
		}
		_, err := NewService(config)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestService_Sign(t *testing.T) {
	config := Config{SecretKey: testSecret}
	service, err := NewService(config)
	require.NoError(t, err)

	userID := "u_testuser123"
	sessionID := "s_testsession456"
	expiry := 15 * time.Minute

	t.Run("successful signing", func(t *testing.T) {
		token, err := service.Sign(userID, sessionID, expiry)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
		assert.Contains(t, token, ".")
	})

	t.Run("empty user ID", func(t *testing.T) {
		_, err := service.Sign("", sessionID, expiry)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("empty session ID", func(t *testing.T) {
		_, err := service.Sign(userID, "", expiry)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})
}

func TestService_Verify(t *testing.T) {
	config := Config{SecretKey: testSecret}
	service, err := NewService(config)
	require.NoError(t, err)

	userID := "u_testuser123"
	sessionID := "s_testsession456"
	expiry := 15 * time.Minute

	t.Run("valid token", func(t *testing.T) {
		token, err := service.Sign(userID, sessionID, expiry)
		require.NoError(t, err)

		claims, err := service.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, sessionID, claims.SessionID)
		assert.True(t, claims.ExpiresAt.After(time.Now()))
	})

	t.Run("expired token", func(t *testing.T) {
		// Sign with very short expiry
		token, err := service.Sign(userID, sessionID, time.Millisecond)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(2 * time.Millisecond)

		_, err = service.Verify(token)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := service.Verify("")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := service.Verify("invalid.token.here")
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeValidation))
	})

	t.Run("token signed with different key", func(t *testing.T) {
		// Create another service with different key
		otherConfig := Config{SecretKey: "another-very-long-secret-key-for-testing-purposes-that-meets-minimum-length"}
		otherService, err := NewService(otherConfig)
		require.NoError(t, err)

		token, err := otherService.Sign(userID, sessionID, expiry)
		require.NoError(t, err)

		// Try to verify with original service
		_, err = service.Verify(token)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})
}

func TestService_Refresh(t *testing.T) {
	config := Config{SecretKey: testSecret}
	service, err := NewService(config)
	require.NoError(t, err)

	userID := "u_testuser123"
	sessionID := "s_testsession456"
	expiry := 15 * time.Minute

	t.Run("valid refresh", func(t *testing.T) {
		originalToken, err := service.Sign(userID, sessionID, expiry)
		require.NoError(t, err)

		newExpiry := 30 * time.Minute
		refreshedToken, err := service.Refresh(originalToken, newExpiry)
		require.NoError(t, err)
		assert.NotEmpty(t, refreshedToken)
		assert.NotEqual(t, originalToken, refreshedToken)

		// Verify refreshed token has same user/session but new expiry
		claims, err := service.Verify(refreshedToken)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, sessionID, claims.SessionID)
	})

	t.Run("refresh expired token", func(t *testing.T) {
		expiredToken, err := service.Sign(userID, sessionID, time.Millisecond)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(2 * time.Millisecond)

		_, err = service.Refresh(expiredToken, 30*time.Minute)
		assert.Error(t, err)
	})

	t.Run("refresh invalid token", func(t *testing.T) {
		_, err := service.Refresh("invalid.token.here", 30*time.Minute)
		assert.Error(t, err)
	})
}

func TestService_RSA(t *testing.T) {
	config := Config{
		PrivateKey: testPrivateKey,
		PublicKey:  testPublicKey,
	}
	service, err := NewService(config)
	require.NoError(t, err)

	userID := "u_testuser123"
	sessionID := "s_testsession456"
	expiry := 15 * time.Minute

	t.Run("RSA sign and verify", func(t *testing.T) {
		token, err := service.Sign(userID, sessionID, expiry)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := service.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, sessionID, claims.SessionID)
	})

	t.Run("RSA refresh", func(t *testing.T) {
		originalToken, err := service.Sign(userID, sessionID, expiry)
		require.NoError(t, err)

		refreshedToken, err := service.Refresh(originalToken, 30*time.Minute)
		require.NoError(t, err)
		assert.NotEqual(t, originalToken, refreshedToken)

		claims, err := service.Verify(refreshedToken)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, sessionID, claims.SessionID)
	})
}

func TestService_ErrorCases(t *testing.T) {
	config := Config{SecretKey: testSecret}
	service, err := NewService(config)
	require.NoError(t, err)

	t.Run("token with wrong signing method", func(t *testing.T) {
		// Create a token with RS256 but verify with HMAC service
		rsaConfig := Config{PrivateKey: testPrivateKey, PublicKey: testPublicKey}
		rsaService, err := NewService(rsaConfig)
		require.NoError(t, err)

		rsaToken, err := rsaService.Sign("u_test", "s_test", 15*time.Minute)
		require.NoError(t, err)

		// Try to verify RSA token with HMAC service
		_, err = service.Verify(rsaToken)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})

	t.Run("keyFunc with wrong token method for HMAC", func(t *testing.T) {
		// Test keyFunc error handling for HMAC when token has wrong method
		token := &jwt.Token{
			Method: jwt.SigningMethodRS256, // RSA method for HMAC service
			Header: map[string]interface{}{"alg": "RS256"},
		}
		_, err := service.keyFunc(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})

	t.Run("keyFunc with wrong token method for RSA", func(t *testing.T) {
		rsaConfig := Config{PrivateKey: testPrivateKey, PublicKey: testPublicKey}
		rsaService, err := NewService(rsaConfig)
		require.NoError(t, err)

		// Test keyFunc error handling for RSA when token has wrong method
		token := &jwt.Token{
			Method: jwt.SigningMethodHS256, // HMAC method for RSA service
			Header: map[string]interface{}{"alg": "HS256"},
		}
		_, err = rsaService.keyFunc(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected signing method")
	})

	t.Run("sign with zero expiry", func(t *testing.T) {
		// Test edge case with zero expiry duration
		_, err := service.Sign("u_test", "s_test", 0)
		// Should still work since we don't validate positive expiry in Sign
		require.NoError(t, err)
	})

	t.Run("verify with malformed token parts", func(t *testing.T) {
		// Test various malformed tokens
		malformedTokens := []string{
			"not.a.token",
			"header.payload", // missing signature
			"", // empty
			"just-one-string",
		}

		for _, malformed := range malformedTokens {
			_, err := service.Verify(malformed)
			assert.Error(t, err)
		}
	})
}

func TestClaims_IsValid(t *testing.T) {
	config := Config{SecretKey: testSecret}
	service, err := NewService(config)
	require.NoError(t, err)

	t.Run("future NotBefore time", func(t *testing.T) {
		userID := "u_test"
		sessionID := "s_test"
		expiry := 15 * time.Minute

		// Manually create a token with future NotBefore
		claims := Claims{
			UserID:    userID,
			SessionID: sessionID,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(time.Hour)), // 1 hour in future
			},
		}

		futureToken := jwt.NewWithClaims(service.method, claims)
		signedToken, err := futureToken.SignedString([]byte(service.secretKey))
		require.NoError(t, err)

		_, err = service.Verify(signedToken)
		assert.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})
}