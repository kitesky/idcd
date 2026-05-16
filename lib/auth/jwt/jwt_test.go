package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kite365/idcd/lib/shared/apperr"
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

// ---------------------------------------------------------------------------
// JTI + blocklist behavior.
//
// Goals validated below:
//   - Every Sign emits a non-empty unique JTI (RegisteredClaims.ID).
//   - With a blocklist wired, Refresh revokes the OLD jti immediately so
//     the previous token cannot be replayed.
//   - The blocklist entry TTL never exceeds the original token's
//     remaining ExpiresAt — once the token would naturally expire,
//     there's no need to keep it on the list.
//   - Verify is fail-closed: a blocklist lookup error is treated as
//     "revoked" (Unauthorized), not "allowed".
//   - Without a blocklist, Verify / Refresh behave identically to the
//     pre-blocklist Service (no JTI lookup, no revocation side-effect).
// ---------------------------------------------------------------------------

func TestService_SignEmitsUniqueJTI(t *testing.T) {
	service, err := NewService(Config{SecretKey: testSecret})
	require.NoError(t, err)

	tok1, err := service.Sign("u_test", "s_test", 15*time.Minute)
	require.NoError(t, err)
	tok2, err := service.Sign("u_test", "s_test", 15*time.Minute)
	require.NoError(t, err)

	c1, err := service.Verify(tok1)
	require.NoError(t, err)
	c2, err := service.Verify(tok2)
	require.NoError(t, err)

	assert.NotEmpty(t, c1.ID, "every JWT must carry a JTI for revocation")
	assert.NotEmpty(t, c2.ID)
	assert.NotEqual(t, c1.ID, c2.ID, "two signs must produce distinct JTIs")
}

func TestService_RefreshRevokesOldToken(t *testing.T) {
	bl := NewInMemoryBlocklist()
	service, err := NewServiceWithOptions(Config{SecretKey: testSecret}, WithBlocklist(bl))
	require.NoError(t, err)

	oldToken, err := service.Sign("u_test", "s_test", 15*time.Minute)
	require.NoError(t, err)

	// Old token must verify before refresh.
	oldClaims, err := service.Verify(oldToken)
	require.NoError(t, err)
	require.NotEmpty(t, oldClaims.ID)

	newToken, err := service.Refresh(oldToken, 15*time.Minute)
	require.NoError(t, err)

	// New token verifies; old token is dead.
	newClaims, err := service.Verify(newToken)
	require.NoError(t, err)
	assert.NotEqual(t, oldClaims.ID, newClaims.ID, "refresh must rotate JTI")

	_, err = service.Verify(oldToken)
	require.Error(t, err, "refreshed-away token must no longer verify")
	assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))

	// Blocklist should hold the old JTI.
	revoked, err := bl.IsRevoked(context.Background(), oldClaims.ID)
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestService_RefreshTTLBoundedByOldExp(t *testing.T) {
	bl := NewInMemoryBlocklist()
	service, err := NewServiceWithOptions(Config{SecretKey: testSecret}, WithBlocklist(bl))
	require.NoError(t, err)

	// Short-lived old token: 30s remaining when refreshed.
	oldExpiry := 30 * time.Second
	oldToken, err := service.Sign("u_test", "s_test", oldExpiry)
	require.NoError(t, err)

	oldClaims, err := service.Verify(oldToken)
	require.NoError(t, err)
	oldExpAt := oldClaims.ExpiresAt.Time

	// Refresh — issue a 24h new token but blocklist TTL must be bounded
	// by the OLD token's remaining lifetime, not the new one's.
	_, err = service.Refresh(oldToken, 24*time.Hour)
	require.NoError(t, err)

	// Inspect the in-memory entry expiry directly (test-only access).
	bl.mu.RLock()
	entryExp, ok := bl.entries[oldClaims.ID]
	bl.mu.RUnlock()
	require.True(t, ok, "old JTI must be on the blocklist")

	// Blocklist entry should expire roughly at the old token's exp.
	// Tolerance: 2s (revocation happens slightly after Sign).
	diff := entryExp.Sub(oldExpAt)
	if diff < 0 {
		diff = -diff
	}
	assert.LessOrEqual(t, diff, 2*time.Second,
		"blocklist TTL must be capped at the old token's remaining exp, not the new token's expiry")
}

func TestService_RefreshTTLCappedAt24h(t *testing.T) {
	// A pathologically long-lived JWT (>24h remaining) must still produce
	// a blocklist TTL no longer than the internal 24h cap.
	bl := NewInMemoryBlocklist()
	service, err := NewServiceWithOptions(Config{SecretKey: testSecret}, WithBlocklist(bl))
	require.NoError(t, err)

	oldToken, err := service.Sign("u_test", "s_test", 7*24*time.Hour) // 7 days
	require.NoError(t, err)
	oldClaims, err := service.Verify(oldToken)
	require.NoError(t, err)

	_, err = service.Refresh(oldToken, time.Minute)
	require.NoError(t, err)

	bl.mu.RLock()
	entryExp, ok := bl.entries[oldClaims.ID]
	bl.mu.RUnlock()
	require.True(t, ok)

	// Cap is 24h. Allow a couple seconds of test wiggle.
	maxExpected := time.Now().Add(24*time.Hour + 2*time.Second)
	assert.True(t, entryExp.Before(maxExpected),
		"blocklist TTL must be capped at 24h for long-lived tokens; got entryExp=%s, now=%s",
		entryExp, time.Now())
}

func TestService_VerifyFailClosed(t *testing.T) {
	// When the blocklist is unreachable, Verify MUST reject the token.
	// Letting a leaked token survive a Redis outage is the bug we're
	// guarding against.
	service, err := NewServiceWithOptions(Config{SecretKey: testSecret}, WithBlocklist(errBlocklist{}))
	require.NoError(t, err)

	token, err := service.Sign("u_test", "s_test", 15*time.Minute)
	require.NoError(t, err)

	_, err = service.Verify(token)
	require.Error(t, err, "blocklist lookup error must be treated as revoked (fail-closed)")
	assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
}

func TestService_NoBlocklist_LegacyBehavior(t *testing.T) {
	// Without WithBlocklist, the Service must behave exactly like
	// pre-blocklist code — no revocation, no errors on refresh-twice.
	service, err := NewService(Config{SecretKey: testSecret})
	require.NoError(t, err)

	oldToken, err := service.Sign("u_test", "s_test", 15*time.Minute)
	require.NoError(t, err)

	newToken, err := service.Refresh(oldToken, 15*time.Minute)
	require.NoError(t, err)

	// Both tokens still verify (legacy behavior — the bug we're fixing
	// for production use, but the no-blocklist path must keep it for
	// backwards compatibility with callers that haven't opted in).
	_, err = service.Verify(oldToken)
	require.NoError(t, err, "no-blocklist Service must keep legacy refresh behavior")
	_, err = service.Verify(newToken)
	require.NoError(t, err)
}

func TestService_RevokeToken(t *testing.T) {
	bl := NewInMemoryBlocklist()
	service, err := NewServiceWithOptions(Config{SecretKey: testSecret}, WithBlocklist(bl))
	require.NoError(t, err)

	t.Run("revokes a valid token", func(t *testing.T) {
		token, err := service.Sign("u_test", "s_test", 15*time.Minute)
		require.NoError(t, err)

		ctx := context.Background()
		require.NoError(t, service.RevokeToken(ctx, token))

		_, err = service.Verify(token)
		require.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})

	t.Run("rejects unparseable token", func(t *testing.T) {
		err := service.RevokeToken(context.Background(), "garbage")
		require.Error(t, err)
		assert.True(t, apperr.Is(err, apperr.CodeUnauthorized))
	})

	t.Run("no-op without blocklist", func(t *testing.T) {
		plainService, err := NewService(Config{SecretKey: testSecret})
		require.NoError(t, err)
		token, err := plainService.Sign("u_test", "s_test", 15*time.Minute)
		require.NoError(t, err)
		require.NoError(t, plainService.RevokeToken(context.Background(), token))
	})
}

func TestBlocklistTTL(t *testing.T) {
	now := time.Now()

	t.Run("nil exp falls back to 24h", func(t *testing.T) {
		got := blocklistTTL(nil, now)
		assert.Equal(t, 24*time.Hour, got)
	})

	t.Run("already-expired returns 0", func(t *testing.T) {
		got := blocklistTTL(jwt.NewNumericDate(now.Add(-time.Second)), now)
		assert.Equal(t, time.Duration(0), got)
	})

	t.Run("within 24h returns remaining", func(t *testing.T) {
		got := blocklistTTL(jwt.NewNumericDate(now.Add(10*time.Minute)), now)
		// Tolerance for now() drift between call sites.
		assert.InDelta(t, (10 * time.Minute).Seconds(), got.Seconds(), 1.0)
	})

	t.Run("beyond 24h is capped at 24h", func(t *testing.T) {
		got := blocklistTTL(jwt.NewNumericDate(now.Add(48*time.Hour)), now)
		assert.Equal(t, 24*time.Hour, got)
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