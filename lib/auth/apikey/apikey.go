// Package apikey provides API key generation and verification functionality.
// Uses Argon2id for secure password hashing and nanoid for key generation.
package apikey

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"github.com/kite365/idcd/packages/shared/apperr"
)

const (
	// Key generation parameters
	keyPrefix = "sk_live_"
	keyLength = 32 // 32 random bytes = 256 bits

	// Argon2id parameters (recommended values)
	argonTime     = 3        // number of iterations
	argonMemory   = 64 * 1024 // memory in KB (64 MB)
	argonThreads  = 4        // number of threads
	argonKeyLen   = 32       // length of derived key in bytes
	saltLen       = 16       // length of salt in bytes
)

// Generate creates a new API key pair: plaintext key and its Argon2id hash.
// The plaintext format is "sk_live_{32_random_bytes_base64}".
// Only the hash should be stored in the database.
func Generate() (plaintext, hash string, err error) {
	// Generate random bytes for the key
	keyBytes := make([]byte, keyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", apperr.Internal("failed to generate random key bytes", err)
	}

	// Create plaintext key with prefix
	plaintext = keyPrefix + base64.RawURLEncoding.EncodeToString(keyBytes)

	// Generate hash
	hash, err = Hash(plaintext)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash generated key: %w", err)
	}

	return plaintext, hash, nil
}

// Hash computes the Argon2id hash of a plaintext API key.
func Hash(plaintext string) (string, error) {
	if plaintext == "" {
		return "", apperr.Validation("plaintext is required", "")
	}

	if !strings.HasPrefix(plaintext, keyPrefix) {
		return "", apperr.Validation("invalid API key format", "prefix must be "+keyPrefix)
	}

	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", apperr.Internal("failed to generate salt", err)
	}

	// Derive key using Argon2id
	derivedKey := argon2.IDKey([]byte(plaintext), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Encode salt and derived key for storage
	// Format: base64(salt):base64(derivedKey)
	saltB64 := base64.RawURLEncoding.EncodeToString(salt)
	keyB64 := base64.RawURLEncoding.EncodeToString(derivedKey)

	return fmt.Sprintf("%s:%s", saltB64, keyB64), nil
}

// Verify checks if the provided plaintext API key matches the stored hash.
func Verify(plaintext, hash string) bool {
	if plaintext == "" || hash == "" {
		return false
	}

	if !strings.HasPrefix(plaintext, keyPrefix) {
		return false
	}

	// Parse the hash to extract salt and derived key
	parts := strings.Split(hash, ":")
	if len(parts) != 2 {
		return false
	}

	saltB64, keyB64 := parts[0], parts[1]

	// Decode salt and stored derived key
	salt, err := base64.RawURLEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}

	storedKey, err := base64.RawURLEncoding.DecodeString(keyB64)
	if err != nil {
		return false
	}

	// Derive key from plaintext with the stored salt
	derivedKey := argon2.IDKey([]byte(plaintext), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Compare using constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(derivedKey, storedKey) == 1
}

// ExtractPrefix extracts the displayable prefix from an API key for storage.
// This allows prefix-based lookup without exposing the full key.
// Uses the same logic as idgen.APIKeyPrefix but operates on our generated keys.
func ExtractPrefix(plaintext string) string {
	if !strings.HasPrefix(plaintext, keyPrefix) {
		return plaintext // return as-is for invalid format
	}

	// Show prefix + first 8 characters after prefix
	const prefixDisplayLen = 8
	if len(plaintext) < len(keyPrefix)+prefixDisplayLen {
		return plaintext
	}

	return plaintext[:len(keyPrefix)+prefixDisplayLen]
}