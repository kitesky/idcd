// Package service — DownloadTokenManager backs the W5 “one-shot URL”
// flow for cert downloads (see docs/prd/20-free-cert.md §10.1).
//
// The handler issues a token at POST /v1/cert/certs/{id}/download and
// the caller follows up with GET /v1/cert/dl/{token}. The token is a
// JWS-style "<payloadB64>.<hmacB64>" string and the payload’s Nonce
// indexes a Redis key holding the same payload with EX=TTL. Consume
// atomically deletes the key, so a token cannot be replayed.
//
// HMAC-SHA256 prevents forgery (attacker can't mint a token without the
// secret); Redis presence prevents replay (each nonce burns once).
package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Default time-to-live for a freshly minted download token.
const DefaultDownloadTokenTTL = 5 * time.Minute

// Redis key prefix for the per-nonce presence record. Indexed only by
// the nonce — the (signed) payload lives in the token itself, so Redis
// stores the same JSON payload purely as a single-use marker we can DEL
// atomically.
const downloadTokenKeyPrefix = "cert:dl:"

// ErrTokenInvalid is the single sentinel Consume surfaces for every
// failure path (bad shape / bad HMAC / expired / already consumed).
// Collapsing onto one error keeps the public handler from leaking which
// of those four conditions tripped.
var ErrTokenInvalid = errors.New("download token invalid or expired")

// DownloadTokenPayload is the data we sign into the token AND store in
// Redis. CertID + AccountID let the GET handler re-fetch the cert and
// re-check ownership; Format + Password drive the encoding path; IssuedAt
// is informational; Nonce is the Redis-key suffix.
type DownloadTokenPayload struct {
	CertID    int64  `json:"cert_id"`
	AccountID int64  `json:"account_id"`
	Format    string `json:"format"`
	Password  string `json:"password,omitempty"`
	IssuedAt  int64  `json:"issued_at"`
	Nonce     string `json:"nonce"`
}

// DownloadTokenManager mints and consumes one-shot download tokens.
// Safe for concurrent use; all state lives in Redis.
type DownloadTokenManager struct {
	rdb    *redis.Client
	secret []byte
	ttl    time.Duration
	// now and randRead are seams the tests poke for deterministic output.
	now      func() time.Time
	randRead func([]byte) (int, error)
}

// DownloadOption tweaks construction; new ones land here so callers don't
// have to grow the constructor signature.
type DownloadOption func(*DownloadTokenManager)

// WithDownloadTTL overrides the default 5-minute lifetime.
func WithDownloadTTL(d time.Duration) DownloadOption {
	return func(m *DownloadTokenManager) {
		if d > 0 {
			m.ttl = d
		}
	}
}

// NewDownloadTokenManager constructs a manager. secret must be non-empty;
// the cert-svc boot path is responsible for failing fast when it is.
func NewDownloadTokenManager(rdb *redis.Client, secret []byte, opts ...DownloadOption) *DownloadTokenManager {
	m := &DownloadTokenManager{
		rdb:      rdb,
		secret:   append([]byte(nil), secret...),
		ttl:      DefaultDownloadTokenTTL,
		now:      time.Now,
		randRead: rand.Read,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// TTL exposes the configured lifetime so handlers can echo expires_at
// back to the caller without recomputing.
func (m *DownloadTokenManager) TTL() time.Duration { return m.ttl }

// Issue mints a token for payload, writes the Redis presence marker and
// returns the token + absolute expiry. payload.Nonce / payload.IssuedAt
// are overwritten with fresh values — the caller's input is ignored.
func (m *DownloadTokenManager) Issue(ctx context.Context, payload DownloadTokenPayload) (string, time.Time, error) {
	if m.rdb == nil {
		return "", time.Time{}, errors.New("download token manager: redis not configured")
	}
	if len(m.secret) == 0 {
		return "", time.Time{}, errors.New("download token manager: empty hmac secret")
	}

	// 16-byte nonce is enough entropy to defeat birthday collisions inside
	// a 5-minute window even at 10^6 issues/sec; hex keeps the value safe
	// for use as a Redis key without further escaping.
	nb := make([]byte, 16)
	if _, err := m.randRead(nb); err != nil {
		return "", time.Time{}, err
	}
	payload.Nonce = hex.EncodeToString(nb)
	payload.IssuedAt = m.now().Unix()

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", time.Time{}, err
	}

	// Redis presence key. SETNX guards against the (vanishingly unlikely)
	// nonce collision: if a key already exists we refuse to reuse it
	// rather than silently overwriting a live token.
	key := downloadTokenKeyPrefix + payload.Nonce
	ok, err := m.rdb.SetNX(ctx, key, raw, m.ttl).Result()
	if err != nil {
		return "", time.Time{}, err
	}
	if !ok {
		return "", time.Time{}, errors.New("download token manager: nonce collision")
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmacSign(m.secret, payloadB64)
	token := payloadB64 + "." + base64.RawURLEncoding.EncodeToString(mac)

	expiresAt := m.now().Add(m.ttl)
	return token, expiresAt, nil
}

// Consume validates token, atomically deletes the Redis marker and
// returns the embedded payload. Any failure → ErrTokenInvalid.
func (m *DownloadTokenManager) Consume(ctx context.Context, token string) (DownloadTokenPayload, error) {
	if m.rdb == nil {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	dot := strings.IndexByte(token, '.')
	if dot <= 0 || dot == len(token)-1 {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	payloadB64, sigB64 := token[:dot], token[dot+1:]

	gotSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	wantSig := hmacSign(m.secret, payloadB64)
	// hmac.Equal is constant-time; using plain == would leak a timing
	// channel an attacker could grind against to forge tokens.
	if !hmac.Equal(gotSig, wantSig) {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}

	raw, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	var p DownloadTokenPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	if p.Nonce == "" || p.CertID <= 0 || p.AccountID <= 0 {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}

	// Atomic single-use: DEL returns 1 iff the key existed. If it returns
	// 0 the token was already consumed (or expired) and we refuse.
	n, err := m.rdb.Del(ctx, downloadTokenKeyPrefix+p.Nonce).Result()
	if err != nil {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	if n == 0 {
		return DownloadTokenPayload{}, ErrTokenInvalid
	}
	return p, nil
}

// hmacSign computes HMAC-SHA256(secret, msg). Pulled into a helper so
// Issue and Consume share the exact bytes-on-the-wire shape.
func hmacSign(secret []byte, msg string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(msg))
	return h.Sum(nil)
}
