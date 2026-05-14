package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
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

func ValidateCode(secret, code string) (bool, error) {
	now := time.Now()
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

// UsedCodeKey returns the Redis key for tracking a consumed TOTP code.
// TTL should be 90s (±1 time-step window) to block replays.
func UsedCodeKey(userID, code string) string {
	return "totp:used:" + userID + ":" + code
}

func OTPAuthURL(issuer, account, secret string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, account, secret, issuer)
}

func GenerateBackupCodes() ([]string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codes := make([]string, 8)
	for i := range codes {
		b := make([]byte, 8)
		for j := range b {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
			if err != nil {
				return nil, fmt.Errorf("generate backup code: %w", err)
			}
			b[j] = chars[n.Int64()]
		}
		codes[i] = string(b)
	}
	return codes, nil
}
