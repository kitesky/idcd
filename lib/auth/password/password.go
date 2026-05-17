// Package password provides secure password hashing and verification using Argon2id.
// This package follows the same security standards as the apikey package.
//
// Storage format (PHC string):
//
//	$argon2id$v=19$m=<memory>,t=<time>,p=<parallelism>$<salt-b64>$<hash-b64>
//
// where memory is in KiB, time is iterations, parallelism is threads,
// and salt/hash are base64 (std, unpadded). This is the standard PHC format
// used by reference Argon2 implementations so we can interop with other tools
// and tune parameters without forcing a full-table rehash on every adjustment.
//
// Legacy format (still accepted by Verify for backward compatibility):
//
//	<salt-b64-rawurl>:<hash-b64-rawurl>
//
// Older hashes were stored as colon-separated raw-URL base64 without algorithm
// parameters. Verify falls back to the legacy code path with the package's
// default Argon2id parameters when the stored hash does not start with
// "$argon2id$". Call NeedsRehash on the stored hash after a successful login
// to opportunistically upgrade legacy hashes (and re-cost ones that don't
// match the current parameter set) to the current PHC format.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/kite365/idcd/lib/shared/apperr"
	"golang.org/x/crypto/argon2"
)

// Params controls Argon2id cost parameters. Bump these when hardware allows
// without breaking older hashes — existing stored hashes still verify with
// their own embedded parameters and NeedsRehash will flag them for upgrade.
type Params struct {
	Memory      uint32 // memory cost in KiB (e.g. 64*1024 = 64 MiB)
	Time        uint32 // iteration count
	Parallelism uint8  // number of parallel lanes / threads
	SaltLen     uint32 // salt length in bytes
	KeyLen      uint32 // derived key length in bytes
}

// DefaultParams returns the current recommended Argon2id parameters.
// Update this when revising password security posture; callers should call
// NeedsRehash(stored, DefaultParams()) after login and rehash with the new
// params if it returns true.
func DefaultParams() Params {
	return Params{
		Memory:      64 * 1024, // 64 MiB
		Time:        3,
		Parallelism: 4,
		SaltLen:     16,
		KeyLen:      32,
	}
}

const (
	// PHC prefix identifying our Argon2id format. Verify uses this to route
	// between the new PHC parser and the legacy "salt:key" decoder.
	phcPrefix    = "$argon2id$"
	argon2Ver    = 19 // golang.org/x/crypto/argon2 implements RFC 9106 / v=19
	argon2VerStr = "v=19"

	// Password validation constraints.
	//
	// Argon2 has no input-length limit (unlike bcrypt's 72-byte cap), so we
	// only need an upper bound to prevent DoS via gigantic inputs. 1024 bytes
	// is well above any realistic passphrase and keeps hashing latency bounded.
	minLength = 8
	maxLength = 1024
)

// ErrInvalidHash is returned when a stored hash cannot be parsed.
var ErrInvalidHash = errors.New("password: invalid hash format")

// Stable ASCII error codes used as the Message of *apperr.Error values
// returned by ValidatePassword. These are NOT user-facing strings — the API
// layer maps them to localized text via the i18n catalog. Keep them stable;
// changing a code is a breaking change for any consumer that maps it.
const (
	CodeTooShort        = "password.too_short"
	CodeTooLong         = "password.too_long"
	CodeMissingClasses  = "password.missing_letter_or_digit"
	CodeSameAsEmail     = "password.same_as_email_prefix"
	CodeRequired        = "password.required"
	CodeInvalidParams   = "password.invalid_argon2_params"
	CodeSaltGenFailed   = "password.salt_generation_failed"
)

// ValidatePassword checks if a password meets security requirements.
//
// Error contract: returns an *apperr.Error with Code=CodeValidation and
// Message set to a stable ASCII identifier (one of the Code* constants in
// this package). The Detail field carries any structured parameters needed
// for rendering — e.g. the minimum length for CodeTooShort — so the API
// layer's i18n template can substitute them.
//
// Callers MUST NOT pattern-match on the error's Error() string; use
// apperr.AsError(err).Message to read the code.
func ValidatePassword(password, email string) error {
	if len(password) < minLength {
		return apperr.Validation(CodeTooShort, strconv.Itoa(minLength))
	}

	if len(password) > maxLength {
		return apperr.Validation(CodeTooLong, strconv.Itoa(maxLength))
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
		return apperr.Validation(CodeMissingClasses, "")
	}

	// Check if password is same as email prefix (before @)
	if email != "" {
		emailPrefix := strings.Split(email, "@")[0]
		if strings.EqualFold(password, emailPrefix) {
			return apperr.Validation(CodeSameAsEmail, "")
		}
	}

	return nil
}

// Hash computes the Argon2id hash of a plaintext password using the current
// DefaultParams and returns it in PHC string form.
func Hash(password string) (string, error) {
	return HashWithParams(password, DefaultParams())
}

// HashWithParams hashes a password with the supplied Argon2id parameters.
// This is exposed primarily for tests and for callers that want to upgrade
// existing hashes to a stronger parameter set.
func HashWithParams(password string, p Params) (string, error) {
	if password == "" {
		return "", apperr.Validation(CodeRequired, "")
	}
	if p.SaltLen == 0 || p.KeyLen == 0 || p.Time == 0 || p.Memory == 0 || p.Parallelism == 0 {
		return "", apperr.Internal(CodeInvalidParams, nil)
	}

	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", apperr.Internal(CodeSaltGenFailed, err)
	}

	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Parallelism, p.KeyLen)

	return encodePHC(p, salt, key), nil
}

// Verify checks if the provided plaintext password matches the stored hash.
// It accepts both the new PHC format and the legacy "salt:key" format so that
// existing user passwords keep working through the transition.
func Verify(password, stored string) bool {
	if password == "" || stored == "" {
		return false
	}

	if strings.HasPrefix(stored, phcPrefix) {
		params, salt, key, err := decodePHC(stored)
		if err != nil {
			return false
		}
		derived := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Parallelism, uint32(len(key)))
		return subtle.ConstantTimeCompare(derived, key) == 1
	}

	return verifyLegacy(password, stored)
}

// verifyLegacy handles the pre-PHC storage format: base64rawurl(salt) ":" base64rawurl(key).
// Legacy hashes are assumed to have been produced with DefaultParams() at the
// time they were written; we use today's defaults because the historical
// constants matched these values. Use NeedsRehash to upgrade them at first
// successful login.
func verifyLegacy(password, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}

	salt, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	key, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	p := DefaultParams()
	derived := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Parallelism, uint32(len(key)))
	return subtle.ConstantTimeCompare(derived, key) == 1
}

// NeedsRehash reports whether the stored password hash should be re-hashed
// with the given parameter set. It returns true when:
//   - the stored hash is in the legacy (non-PHC) format
//   - the stored hash is malformed
//   - any Argon2id parameter (m/t/p) differs from current
//   - the stored key length differs from current
//
// Call this after a successful Verify; if it returns true, rehash the
// plaintext with HashWithParams(plain, current) and write the result back
// to the user record.
func NeedsRehash(stored string, current Params) bool {
	if !strings.HasPrefix(stored, phcPrefix) {
		// Legacy or empty — always upgrade.
		return true
	}
	params, _, key, err := decodePHC(stored)
	if err != nil {
		return true
	}
	if params.Memory != current.Memory ||
		params.Time != current.Time ||
		params.Parallelism != current.Parallelism {
		return true
	}
	if uint32(len(key)) != current.KeyLen {
		return true
	}
	return false
}

// encodePHC formats a (params, salt, key) tuple as a standard Argon2id PHC string.
func encodePHC(p Params, salt, key []byte) string {
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	keyB64 := base64.RawStdEncoding.EncodeToString(key)
	return fmt.Sprintf("$argon2id$%s$m=%d,t=%d,p=%d$%s$%s",
		argon2VerStr, p.Memory, p.Time, p.Parallelism, saltB64, keyB64)
}

// decodePHC parses a PHC string of the form produced by encodePHC and returns
// the embedded parameters, salt, and key. It rejects unknown algorithms,
// unsupported versions, malformed parameter sections, or bad base64.
func decodePHC(s string) (Params, []byte, []byte, error) {
	// Format: $argon2id$v=19$m=<m>,t=<t>,p=<p>$<saltB64>$<keyB64>
	// Splitting on "$" yields ["", "argon2id", "v=19", "m=...,t=...,p=...", "salt", "key"].
	parts := strings.Split(s, "$")
	if len(parts) != 6 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}

	// Version.
	if !strings.HasPrefix(parts[2], "v=") {
		return Params{}, nil, nil, ErrInvalidHash
	}
	ver, err := strconv.Atoi(parts[2][2:])
	if err != nil || ver != argon2Ver {
		return Params{}, nil, nil, ErrInvalidHash
	}

	// Parameters: m=<m>,t=<t>,p=<p>
	paramFields := strings.Split(parts[3], ",")
	if len(paramFields) != 3 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	var p Params
	for _, f := range paramFields {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			return Params{}, nil, nil, ErrInvalidHash
		}
		switch kv[0] {
		case "m":
			v, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil || v == 0 {
				return Params{}, nil, nil, ErrInvalidHash
			}
			p.Memory = uint32(v)
		case "t":
			v, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil || v == 0 {
				return Params{}, nil, nil, ErrInvalidHash
			}
			p.Time = uint32(v)
		case "p":
			v, err := strconv.ParseUint(kv[1], 10, 8)
			if err != nil || v == 0 {
				return Params{}, nil, nil, ErrInvalidHash
			}
			p.Parallelism = uint8(v)
		default:
			return Params{}, nil, nil, ErrInvalidHash
		}
	}
	if p.Memory == 0 || p.Time == 0 || p.Parallelism == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	p.SaltLen = uint32(len(salt))
	p.KeyLen = uint32(len(key))

	return p, salt, key, nil
}
