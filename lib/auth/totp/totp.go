package totp

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"
)

func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

func GenerateCode(secret string, t time.Time) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	counter := uint64(t.Unix()) / 30
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	code := binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", code%1_000_000), nil
}

// ValidateCode validates a TOTP code against the secret using the system
// clock and a ±1 step window. For new code paths prefer Validator which adds
// clock injection and replay protection (see RFC 6238 §5.2 — codes must be
// single-use within their validity window).
func ValidateCode(secret, code string) (bool, error) {
	return validateCodeAt(secret, code, time.Now())
}

func validateCodeAt(secret, code string, now time.Time) (bool, error) {
	for _, delta := range []int64{-1, 0, 1} {
		t := now.Add(time.Duration(delta) * 30 * time.Second)
		c, err := GenerateCode(secret, t)
		if err != nil {
			return false, err
		}
		if c == code {
			return true, nil
		}
	}
	return false, nil
}

// ReplayStore tracks consumed TOTP codes to prevent reuse within the
// validity window. SetNX semantics: Mark must atomically claim a key and
// report whether it was previously absent.
type ReplayStore interface {
	// Mark reports true if key was newly inserted (=> first use); false if
	// it already exists (=> replay). TTL bounds how long the entry stays
	// blocked; callers should set ≥ 90s to cover the ±1 step window.
	Mark(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Validator validates codes with an injectable clock + optional replay
// protection. The zero value is a working validator using time.Now and no
// replay protection (matches legacy ValidateCode behaviour).
type Validator struct {
	Now    func() time.Time
	Replay ReplayStore
}

func (v *Validator) now() time.Time {
	if v != nil && v.Now != nil {
		return v.Now()
	}
	return time.Now()
}

// Validate verifies code for secret. When userID is non-empty and Replay
// is configured, a successful validation also claims the code so a second
// validation with the same (userID, code) within the window returns
// (false, nil).
func (v *Validator) Validate(ctx context.Context, secret, userID, code string) (bool, error) {
	ok, err := validateCodeAt(secret, code, v.now())
	if err != nil || !ok {
		return ok, err
	}
	if v == nil || v.Replay == nil || userID == "" {
		return true, nil
	}
	// 90s = 3 × 30s step covers the ±1 window we accepted above plus a
	// small safety margin for clock skew between caller + Redis.
	fresh, err := v.Replay.Mark(ctx, UsedCodeKey(userID, code), 90*time.Second)
	if err != nil {
		return false, fmt.Errorf("totp replay store: %w", err)
	}
	if !fresh {
		return false, nil
	}
	return true, nil
}

// UsedCodeKey returns the Redis key for tracking a consumed TOTP code.
// TTL should be 90s (±1 time-step window) to block replays.
func UsedCodeKey(userID, code string) string {
	return "totp:used:" + userID + ":" + code
}

func OTPAuthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func GenerateBackupCodes() ([]string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seen := make(map[string]struct{}, 8)
	codes := make([]string, 0, 8)
	for len(codes) < 8 {
		b := make([]byte, 8)
		for j := range b {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			if err != nil {
				return nil, fmt.Errorf("generate backup code: %w", err)
			}
			b[j] = chars[n.Int64()]
		}
		code := string(b)
		if _, dup := seen[code]; !dup {
			seen[code] = struct{}{}
			codes = append(codes, code)
		}
	}
	return codes, nil
}
