// Package jwt provides JWT token signing and verification functionality.
// Supports both HMAC (HS256) and RSA (RS256) signing methods.
package jwt

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kite365/idcd/packages/shared/apperr"
)

// Claims represents the JWT token claims.
type Claims struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// Service provides JWT token operations.
type Service struct {
	secretKey  string
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	method     jwt.SigningMethod
}

// Config holds the JWT configuration.
type Config struct {
	SecretKey  string `yaml:"secret_key"`  // for HMAC signing
	PrivateKey string `yaml:"private_key"` // PEM format RSA private key
	PublicKey  string `yaml:"public_key"`  // PEM format RSA public key
}

// NewService creates a new JWT service with the given config.
// If SecretKey is provided, uses HMAC-SHA256. If RSA keys are provided, uses RSA-SHA256.
func NewService(config Config) (*Service, error) {
	if config.SecretKey != "" {
		if len(config.SecretKey) < 32 {
			return nil, apperr.Validation("JWT secret key must be at least 32 characters", "")
		}
		return &Service{
			secretKey: config.SecretKey,
			method:    jwt.SigningMethodHS256,
		}, nil
	}

	if config.PrivateKey != "" && config.PublicKey != "" {
		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(config.PrivateKey))
		if err != nil {
			return nil, apperr.Validation("invalid RSA private key", err.Error())
		}

		publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(config.PublicKey))
		if err != nil {
			return nil, apperr.Validation("invalid RSA public key", err.Error())
		}

		return &Service{
			privateKey: privateKey,
			publicKey:  publicKey,
			method:     jwt.SigningMethodRS256,
		}, nil
	}

	return nil, apperr.Validation("either secret_key or RSA key pair must be provided", "")
}

// Sign creates a new JWT token with the given user ID, session ID, and expiry duration.
func (s *Service) Sign(userID, sessionID string, expiry time.Duration) (string, error) {
	if userID == "" {
		return "", apperr.Validation("user ID is required", "")
	}
	if sessionID == "" {
		return "", apperr.Validation("session ID is required", "")
	}

	now := time.Now()
	claims := Claims{
		UserID:    userID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
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
func (s *Service) Verify(tokenString string) (*Claims, error) {
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

	return claims, nil
}

// Refresh validates an existing token and issues a new one with updated expiry.
// The old token must be valid (not expired) for refresh to succeed.
func (s *Service) Refresh(tokenString string, newExpiry time.Duration) (string, error) {
	claims, err := s.Verify(tokenString)
	if err != nil {
		return "", fmt.Errorf("refresh token verification failed: %w", err)
	}

	// Issue new token with same user and session but new expiry
	return s.Sign(claims.UserID, claims.SessionID, newExpiry)
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