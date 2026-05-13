// Package password provides secure password hashing and verification using Argon2id.
// This package follows the same security standards as the apikey package.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
	"github.com/kite365/idcd/lib/shared/apperr"
)

const (
	// Argon2id parameters (recommended values for password hashing)
	argonTime     = 3        // number of iterations
	argonMemory   = 64 * 1024 // memory in KB (64 MB)
	argonThreads  = 4        // number of threads
	argonKeyLen   = 32       // length of derived key in bytes
	saltLen       = 16       // length of salt in bytes

	// Password validation constraints
	minLength = 8
	maxLength = 72 // bcrypt limit, we follow similar constraint
)

// ValidationError represents password validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidatePassword checks if a password meets security requirements.
func ValidatePassword(password, email string) error {
	if len(password) < minLength {
		return &ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("密码长度必须至少%d字符", minLength),
		}
	}

	if len(password) > maxLength {
		return &ValidationError{
			Field:   "password",
			Message: fmt.Sprintf("密码长度不能超过%d字符", maxLength),
		}
	}

	// Check for letter + number requirement
	hasLetter := false
	hasDigit := false

	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return &ValidationError{
			Field:   "password",
			Message: "密码必须包含字母和数字",
		}
	}

	// Check if password is same as email prefix (before @)
	if email != "" {
		emailPrefix := strings.Split(email, "@")[0]
		if strings.EqualFold(password, emailPrefix) {
			return &ValidationError{
				Field:   "password",
				Message: "密码不能与邮箱前缀相同",
			}
		}
	}

	return nil
}

// Hash computes the Argon2id hash of a plaintext password.
func Hash(password string) (string, error) {
	if password == "" {
		return "", apperr.Validation("password is required", "")
	}

	// Generate random salt
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", apperr.Internal("failed to generate salt", err)
	}

	// Derive key using Argon2id
	derivedKey := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Encode salt and derived key for storage
	// Format: base64(salt):base64(derivedKey)
	saltB64 := base64.RawURLEncoding.EncodeToString(salt)
	keyB64 := base64.RawURLEncoding.EncodeToString(derivedKey)

	return fmt.Sprintf("%s:%s", saltB64, keyB64), nil
}

// Verify checks if the provided plaintext password matches the stored hash.
func Verify(password, hash string) bool {
	if password == "" || hash == "" {
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
	derivedKey := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Compare using constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(derivedKey, storedKey) == 1
}