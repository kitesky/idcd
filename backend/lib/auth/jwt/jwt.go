// Package jwt provides JWT token signing and verification functionality.
// Supports both HMAC (HS256) and RSA (RS256) signing methods.
//
// Revocation (jti blocklist):
//
// When constructed via NewServiceWithOptions(..., WithBlocklist(bl)), the
// Service writes a JTI ("jti" RegisteredClaim) on every Sign and consults
// the blocklist on every Verify. Refresh rotates the JTI and revokes the
// old one for the remainder of its original lifetime, so a leaked token
// is killed the moment its owner refreshes (or an admin calls Revoke).
//
// Blocklist lookups are fail-closed: if the blocklist returns an error
// (Redis down, etc.), Verify treats the token as revoked. This trades
// brief availability hits for "leaked token still works" risk; the latter
// is unacceptable for an attestation product.
//
// When no blocklist is wired (legacy NewService), Sign still emits a JTI
// for forward-compat but Verify/Refresh never touch a blocklist.
package jwt

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/kite365/idcd/lib/shared/apperr"
)

// Claims represents the JWT token claims.
//
// Locale is an optional short locale code (e.g. "cn" / "en") matching the
// shared i18n registry. It is omitted from the wire format when empty so
// older tokens issued before the field existed remain backwards compatible
// — Verify won't reject them, and Sign won't emit the claim unless an
// explicit locale is supplied via SignWithLocale.
//
// The JTI ("jti") lives on RegisteredClaims and is set automatically by
// Sign / SignWithLocale; it's the key used by JTIBlocklist for revocation.
type Claims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Locale    string `json:"locale,omitempty"`
	jwt.RegisteredClaims
}

// Service provides JWT token operations.
type Service struct {
	secretKey  string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	method     jwt.SigningMethod

	// Optional. When non-nil, Sign writes a JTI, Refresh revokes the
	// previous JTI, and Verify rejects tokens whose JTI is on the list.
	blocklist JTIBlocklist
}

// Config holds the JWT configuration.
type Config struct {
	SecretKey  string `yaml:"secret_key"`  // for HMAC signing
	PrivateKey string `yaml:"private_key"` // PEM format RSA private key
	PublicKey  string `yaml:"public_key"`  // PEM format RSA public key
}

// Option configures a Service at construction time. Use with
// NewServiceWithOptions; NewService remains for callers that don't need
// any options (it's equivalent to NewServiceWithOptions(cfg)).
type Option func(*Service)

// WithBlocklist wires a JTIBlocklist into the Service. When set, every
// successful Sign records its JTI, Refresh revokes the old JTI for the
// remainder of its lifetime, and Verify rejects tokens whose JTI is on
// the list (fail-closed on lookup error).
func WithBlocklist(bl JTIBlocklist) Option {
	return func(s *Service) { s.blocklist = bl }
}

// NewService creates a new JWT service with the given config.
// If SecretKey is provided, uses HMAC-SHA256. If RSA keys are provided, uses RSA-SHA256.
//
// NewService is preserved for backward compatibility; callers that want a
// JTI blocklist should use NewServiceWithOptions(cfg, WithBlocklist(...)).
func NewService(config Config) (*Service, error) {
	return NewServiceWithOptions(config)
}

// NewServiceWithOptions is NewService with functional options. The most
// common option today is WithBlocklist.
func NewServiceWithOptions(config Config, opts ...Option) (*Service, error) {
	var svc *Service
	switch {
	case config.SecretKey != "":
		if len(config.SecretKey) < 32 {
			return nil, apperr.Validation("JWT secret key must be at least 32 characters", "")
		}
		svc = &Service{
			secretKey: config.SecretKey,
			method:    jwt.SigningMethodHS256,
		}
	case config.PrivateKey != "" && config.PublicKey != "":
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKey))
		if err != nil {
			return nil, apperr.Validation("invalid RSA private key", err.Error())
		}
		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
		if err != nil {
			return nil, apperr.Validation("invalid RSA public key", err.Error())
		}
		svc = &Service{
			privateKey: privateKey,
			publicKey:  publicKey,
			method:     jwt.SigningMethodRS256,
		}
	default:
		return nil, apperr.Validation("either secret_key or RSA key pair must be provided", "")
	}

	for _, opt := range opts {
		opt(svc)
	}
	return svc, nil
}

// Sign creates a new JWT token with the given user ID, session ID, and expiry duration.
//
// Sign does not embed a locale claim; use SignWithLocale when the caller
// has a user locale to carry over (e.g. immediately after a successful
// login or refresh).
func (s *Service) Sign(userID, sessionID string, expiry time.Duration) (string, error) {
	return s.SignWithLocale(userID, sessionID, "", expiry)
}

// SignWithLocale is the same as Sign but additionally embeds a "locale"
// claim into the token. Pass "" to skip the claim (equivalent to Sign).
//
// The locale string is stored verbatim — validation against the shared
// registry is the caller's responsibility (typically the handler that owns
// user.locale at signing time).
func (s *Service) SignWithLocale(userID, sessionID, locale string, expiry time.Duration) (string, error) {
	if userID == "" {
		return "", apperr.Validation("user ID is required", "")
	}
	if sessionID == "" {
		return "", apperr.Validation("session ID is required", "")
	}
	if expiry <= 0 {
		// Refuse to mint a token that's already expired. Zero / negative
		// expiry usually means a caller forgot to populate the config —
		// fail loudly rather than silently issue an unusable token (which
		// in turn would surface as cryptic 401s downstream).
		return "", apperr.Validation("expiry must be positive", "")
	}

	now := time.Now()
	claims := Claims{
		UserID:    userID,
		SessionID: sessionID,
		Locale:    locale,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(s.method, claims)

	var signedToken string
	var err error

	switch s.method {
	case jwt.SigningMethodHS256:
		signedToken, err = token.SignedString([]byte(s.secretKey))
	case jwt.SigningMethodRS256:
		signedToken, err = token.SignedString(s.privateKey)
	default:
		return "", apperr.Internal("unsupported signing method", nil)
	}

	if err != nil {
		return "", apperr.Internal("failed to sign JWT token", err)
	}

	return signedToken, nil
}

// Verify validates and parses a JWT token, returning the claims.
//
// Verify uses context.Background() for any blocklist lookup; prefer
// VerifyCtx in request paths so cancellation propagates. When the
// Service has no blocklist (NewService without WithBlocklist), Verify
// behaves exactly as before.
func (s *Service) Verify(tokenString string) (*Claims, error) {
	return s.VerifyCtx(context.Background(), tokenString)
}

// VerifyCtx is Verify with an explicit context for blocklist lookups.
//
// Fail-closed: if the blocklist returns an error, the token is treated
// as revoked and ErrTokenRevoked is wrapped into an Unauthorized apperr.
func (s *Service) VerifyCtx(ctx context.Context, tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, apperr.Validation("token is required", "")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, s.keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, apperr.Unauthorized("token has expired")
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, apperr.Unauthorized("token not valid yet")
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, apperr.Validation("malformed token", err.Error())
		}
		return nil, apperr.Unauthorized("invalid token")
	}

	if !token.Valid {
		return nil, apperr.Unauthorized("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, apperr.Internal("failed to parse token claims", nil)
	}

	// Blocklist check happens AFTER signature + exp + nbf. We only
	// consult the blocklist when the JWT itself is otherwise valid; a
	// missing JTI (legacy token issued before this feature) is treated
	// as not-revoked — there's nothing to look up.
	if s.blocklist != nil && claims.ID != "" {
		revoked, lookupErr := s.blocklist.IsRevoked(ctx, claims.ID)
		if lookupErr != nil {
			// Fail-closed: a leaked token must not survive a Redis
			// outage. Brief availability hit is the lesser evil.
			return nil, apperr.Unauthorized("token revoked")
		}
		if revoked {
			return nil, apperr.Unauthorized("token revoked")
		}
	}

	return claims, nil
}

// Refresh validates an existing token and issues a new one with updated expiry.
// The old token must be valid (not expired) for refresh to succeed.
//
// When a blocklist is wired, the old token's JTI is revoked for the
// remainder of its original lifetime BEFORE the new token is returned,
// so a caller that holds the old token can no longer use it. The
// blocklist TTL is capped at the old token's remaining ExpiresAt;
// after that point the token would fail the exp check anyway.
//
// Refresh uses context.Background(); prefer RefreshCtx in request paths.
func (s *Service) Refresh(tokenString string, newExpiry time.Duration) (string, error) {
	return s.RefreshCtx(context.Background(), tokenString, newExpiry)
}

// RefreshCtx is Refresh with an explicit context for blocklist operations.
func (s *Service) RefreshCtx(ctx context.Context, tokenString string, newExpiry time.Duration) (string, error) {
	claims, err := s.VerifyCtx(ctx, tokenString)
	if err != nil {
		return "", fmt.Errorf("refresh token verification failed: %w", err)
	}

	// Revoke the old JTI before issuing the new token. We bound the TTL
	// by the OLD token's remaining lifetime — once it would naturally
	// expire there's no need to keep it on the blocklist. If revocation
	// fails we surface the error rather than returning a new token while
	// the old one stays usable; that would defeat the purpose.
	if s.blocklist != nil && claims.ID != "" {
		ttl := blocklistTTL(claims.ExpiresAt, time.Now())
		if ttl > 0 {
			if err := s.blocklist.Revoke(ctx, claims.ID, ttl); err != nil {
				return "", fmt.Errorf("refresh blocklist revoke failed: %w", err)
			}
		}
	}

	// Issue new token with same user, session, and locale but new expiry.
	// SignWithLocale handles the empty-locale case (omits the claim) and
	// generates a fresh JTI so the new token has its own identity.
	return s.SignWithLocale(claims.UserID, claims.SessionID, claims.Locale, newExpiry)
}

// RevokeToken parses a token (without checking the blocklist) and writes
// its JTI to the blocklist for the rest of the token's natural lifetime.
// Useful for explicit logout flows.
//
// Returns nil when no blocklist is wired (caller must check) — this is
// intentional so feature-gated callers can fail open during rollout.
// Returns an Unauthorized error if the token is unparseable / expired.
func (s *Service) RevokeToken(ctx context.Context, tokenString string) error {
	if s.blocklist == nil {
		return nil
	}
	// Parse without invoking the blocklist (otherwise an already-revoked
	// token would refuse to be revoked-again, which is fine but spammy).
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, s.keyFunc)
	if err != nil || !token.Valid {
		return apperr.Unauthorized("cannot revoke invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || claims.ID == "" {
		return apperr.Unauthorized("token has no JTI")
	}
	ttl := blocklistTTL(claims.ExpiresAt, time.Now())
	if ttl <= 0 {
		// Already expired; nothing to revoke.
		return nil
	}
	if err := s.blocklist.Revoke(ctx, claims.ID, ttl); err != nil {
		return fmt.Errorf("blocklist revoke failed: %w", err)
	}
	return nil
}

// blocklistTTL returns how long a JTI should sit on the blocklist —
// the lesser of the token's remaining lifetime and an internal sanity
// cap (24h). Negative results mean "don't bother".
func blocklistTTL(exp *jwt.NumericDate, now time.Time) time.Duration {
	if exp == nil {
		// No exp claim — fall back to a conservative 24h cap.
		return 24 * time.Hour
	}
	remaining := exp.Time.Sub(now)
	if remaining <= 0 {
		return 0
	}
	if remaining > 24*time.Hour {
		// JWT access tokens shouldn't have multi-day lifetimes; if
		// one does, cap the blocklist entry at 24h to keep Redis
		// pressure bounded. Adjust if long-lived JWTs become a use case.
		return 24 * time.Hour
	}
	return remaining
}

// keyFunc returns the appropriate key for token validation based on signing method.
func (s *Service) keyFunc(token *jwt.Token) (any, error) {
	switch s.method {
	case jwt.SigningMethodHS256:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	case jwt.SigningMethodRS256:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	default:
		return nil, fmt.Errorf("unsupported signing method: %v", s.method)
	}
}
