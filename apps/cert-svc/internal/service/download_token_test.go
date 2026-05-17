package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newDownloadHarness(t *testing.T, opts ...DownloadOption) (*DownloadTokenManager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	m := NewDownloadTokenManager(rdb, []byte("test-secret-32-bytes-or-so--xxxx"), opts...)
	return m, mr
}

func TestDownloadToken_IssueConsumeRoundTrip(t *testing.T) {
	m, _ := newDownloadHarness(t)
	ctx := context.Background()
	want := DownloadTokenPayload{CertID: 9, AccountID: 42, Format: "pem"}

	token, expiresAt, err := m.Issue(ctx, want)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.True(t, strings.Contains(token, "."), "token must be <payload>.<hmac>")
	require.WithinDuration(t, time.Now().Add(DefaultDownloadTokenTTL), expiresAt, time.Second)

	got, err := m.Consume(ctx, token)
	require.NoError(t, err)
	require.Equal(t, want.CertID, got.CertID)
	require.Equal(t, want.AccountID, got.AccountID)
	require.Equal(t, want.Format, got.Format)
	require.NotEmpty(t, got.Nonce)
	require.NotZero(t, got.IssuedAt)
}

func TestDownloadToken_DoubleConsumeFails(t *testing.T) {
	m, _ := newDownloadHarness(t)
	ctx := context.Background()
	token, _, err := m.Issue(ctx, DownloadTokenPayload{CertID: 9, AccountID: 42, Format: "pem"})
	require.NoError(t, err)

	_, err = m.Consume(ctx, token)
	require.NoError(t, err)

	_, err = m.Consume(ctx, token)
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestDownloadToken_BadHMACFails(t *testing.T) {
	m, _ := newDownloadHarness(t)
	ctx := context.Background()
	token, _, err := m.Issue(ctx, DownloadTokenPayload{CertID: 9, AccountID: 42, Format: "pem"})
	require.NoError(t, err)

	// Flip the signature segment.
	parts := strings.SplitN(token, ".", 2)
	require.Len(t, parts, 2)
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte("definitely-not-the-mac"))
	_, err = m.Consume(ctx, tampered)
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestDownloadToken_TamperedPayloadFails(t *testing.T) {
	// Mutating the payload but keeping the original HMAC must fail because
	// hmacSign is over the (now-changed) payload bytes.
	m, _ := newDownloadHarness(t)
	ctx := context.Background()
	token, _, err := m.Issue(ctx, DownloadTokenPayload{CertID: 9, AccountID: 42, Format: "pem"})
	require.NoError(t, err)

	parts := strings.SplitN(token, ".", 2)
	require.Len(t, parts, 2)
	// Re-encode a different payload but keep the old MAC.
	raw, _ := base64.RawURLEncoding.DecodeString(parts[0])
	swapped := strings.Replace(string(raw), `"cert_id":9`, `"cert_id":7`, 1)
	tampered := base64.RawURLEncoding.EncodeToString([]byte(swapped)) + "." + parts[1]
	_, err = m.Consume(ctx, tampered)
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestDownloadToken_ExpiredFails(t *testing.T) {
	m, mr := newDownloadHarness(t, WithDownloadTTL(2*time.Second))
	ctx := context.Background()
	token, _, err := m.Issue(ctx, DownloadTokenPayload{CertID: 9, AccountID: 42, Format: "pem"})
	require.NoError(t, err)

	// miniredis FastForward expires TTL'd keys without sleeping.
	mr.FastForward(3 * time.Second)

	_, err = m.Consume(ctx, token)
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestDownloadToken_MalformedShapeFails(t *testing.T) {
	m, _ := newDownloadHarness(t)
	cases := []string{
		"",
		"no-dot-here",
		".only-mac",
		"only-payload.",
		"!!!.!!!",
	}
	for _, tc := range cases {
		_, err := m.Consume(context.Background(), tc)
		require.ErrorIs(t, err, ErrTokenInvalid, "token=%q", tc)
	}
}

func TestDownloadToken_TTLAccessor(t *testing.T) {
	m, _ := newDownloadHarness(t, WithDownloadTTL(90*time.Second))
	require.Equal(t, 90*time.Second, m.TTL())
}

func TestDownloadToken_DefaultTTL(t *testing.T) {
	m, _ := newDownloadHarness(t)
	require.Equal(t, DefaultDownloadTokenTTL, m.TTL())
}

func TestDownloadToken_Issue_NoRedisFails(t *testing.T) {
	m := NewDownloadTokenManager(nil, []byte("x"))
	_, _, err := m.Issue(context.Background(), DownloadTokenPayload{CertID: 1, AccountID: 1, Format: "pem"})
	require.Error(t, err)
}

func TestDownloadToken_Issue_EmptySecretFails(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	m := NewDownloadTokenManager(rdb, nil)
	_, _, err := m.Issue(context.Background(), DownloadTokenPayload{CertID: 1, AccountID: 1, Format: "pem"})
	require.Error(t, err)
}

func TestDownloadToken_Issue_RandFailureFails(t *testing.T) {
	m, _ := newDownloadHarness(t)
	m.randRead = func(_ []byte) (int, error) { return 0, errors.New("boom") }
	_, _, err := m.Issue(context.Background(), DownloadTokenPayload{CertID: 1, AccountID: 1, Format: "pem"})
	require.Error(t, err)
}

func TestDownloadToken_Consume_NilRedisFails(t *testing.T) {
	m := NewDownloadTokenManager(nil, []byte("x"))
	_, err := m.Consume(context.Background(), "a.b")
	require.ErrorIs(t, err, ErrTokenInvalid)
}

func TestDownloadToken_Consume_RejectsZeroIDs(t *testing.T) {
	// Forge a payload with cert_id=0, sign with the real secret, write to
	// Redis manually — Consume must still reject because zero IDs would
	// hand any caller a wildcard download URL.
	m, mr := newDownloadHarness(t)
	ctx := context.Background()

	// Issue a real token, then DEL + replace the marker so Consume's Redis
	// check succeeds but the payload validation kicks in. We achieve the
	// same outcome by minting through Issue with the legitimate values,
	// then mutating the *token* but not the Redis key. Easier path:
	// hand-build a payload with zero ID and sign it.
	payload := []byte(`{"cert_id":0,"account_id":42,"format":"pem","issued_at":1,"nonce":"deadbeef"}`)
	p64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := hmacSign(m.secret, p64)
	token := p64 + "." + base64.RawURLEncoding.EncodeToString(sig)

	// Pre-stamp the Redis presence record so Consume's DEL would succeed
	// if validation weren't present — proves the guard rejects independently.
	require.NoError(t, mr.Set("cert:dl:deadbeef", string(payload)))

	_, err := m.Consume(ctx, token)
	require.ErrorIs(t, err, ErrTokenInvalid)
}
