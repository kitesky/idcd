package alikms

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	kmssdk "github.com/alibabacloud-go/kms-20160120/v3/client"
	teasdk "github.com/alibabacloud-go/tea/tea"

	"github.com/kite365/idcd/lib/attest/sign"
)

// ---------- mockKMSClient ----------

type mockKMSClient struct {
	signResult     []byte
	signErr        error
	getPubResult   []byte
	getPubErr      error
	describeResult int
	describeErr    error
	signCalls      atomic.Int64
	getPubCalls    atomic.Int64
	describeCalls  atomic.Int64
	mu             sync.Mutex
	lastIdempotency string
	lastDigest      []byte
	lastSignKeyID   string
	lastSignAlg     string
}

func (m *mockKMSClient) Sign(_ context.Context, keyID, algorithm string, digest []byte, idempotencyKey string) ([]byte, error) {
	m.signCalls.Add(1)
	m.mu.Lock()
	m.lastIdempotency = idempotencyKey
	m.lastDigest = append([]byte(nil), digest...)
	m.lastSignKeyID = keyID
	m.lastSignAlg = algorithm
	m.mu.Unlock()
	if m.signErr != nil {
		return nil, m.signErr
	}
	return append([]byte(nil), m.signResult...), nil
}

func (m *mockKMSClient) GetPublicKey(_ context.Context, _ string) ([]byte, error) {
	m.getPubCalls.Add(1)
	if m.getPubErr != nil {
		return nil, m.getPubErr
	}
	return append([]byte(nil), m.getPubResult...), nil
}

func (m *mockKMSClient) DescribeKey(_ context.Context, _ string) (int, error) {
	m.describeCalls.Add(1)
	if m.describeErr != nil {
		return 0, m.describeErr
	}
	return m.describeResult, nil
}

// ---------- New / NewVerifier 校验 ----------

func TestNewMissingFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"region", Config{AccessKeyID: "a", AccessKeySecret: "s", KeyID: "k"}},
		{"ak", Config{RegionID: "cn-hangzhou", AccessKeySecret: "s", KeyID: "k"}},
		{"sk", Config{RegionID: "cn-hangzhou", AccessKeyID: "a", KeyID: "k"}},
		{"keyid", Config{RegionID: "cn-hangzhou", AccessKeyID: "a", AccessKeySecret: "s"}},
		{"bad alg", Config{RegionID: "cn-hangzhou", AccessKeyID: "a", AccessKeySecret: "s", KeyID: "k", Algorithm: "WAT"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(c.cfg)
			if !errors.Is(err, sign.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			_, err = NewVerifier(c.cfg)
			if !errors.Is(err, sign.ErrInvalidInput) {
				t.Fatalf("verifier err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestNewHappy(t *testing.T) {
	s, err := New(Config{
		RegionID: "cn-hangzhou", AccessKeyID: "a", AccessKeySecret: "s",
		KeyID: "alias/test",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.KeyID() != "alias/test" {
		t.Fatalf("KeyID = %q", s.KeyID())
	}
	if s.Algorithm() != sign.AlgorithmECDSASHA256 {
		t.Fatalf("Algorithm = %q", s.Algorithm())
	}
}

func TestNewVerifierHappy(t *testing.T) {
	v, err := NewVerifier(Config{
		RegionID: "cn-hangzhou", AccessKeyID: "a", AccessKeySecret: "s",
		KeyID: "k", Algorithm: sign.AlgorithmRSAPSSSHA384,
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if v.Algorithm() != sign.AlgorithmRSAPSSSHA384 {
		t.Fatalf("Algorithm = %q", v.Algorithm())
	}
}

// ---------- Sign happy + idempotency ----------

func TestSignHappy(t *testing.T) {
	mock := &mockKMSClient{signResult: []byte("sig-bytes")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	digest := make([]byte, 32)
	sig, err := s.Sign(context.Background(), digest, "idem-1")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !bytes.Equal(sig, []byte("sig-bytes")) {
		t.Fatalf("sig = %q", sig)
	}
	if mock.lastIdempotency != "idem-1" {
		t.Fatalf("lastIdempotency = %q", mock.lastIdempotency)
	}
	if mock.lastSignAlg != sign.AlgorithmECDSASHA256 {
		t.Fatalf("lastSignAlg = %q", mock.lastSignAlg)
	}
	if mock.lastSignKeyID != "k" {
		t.Fatalf("lastSignKeyID = %q", mock.lastSignKeyID)
	}
}

func TestSignIdempotencyCacheHit(t *testing.T) {
	mock := &mockKMSClient{signResult: []byte("sig-bytes")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	digest := make([]byte, 32)
	for i := 0; i < 5; i++ {
		if _, err := s.Sign(context.Background(), digest, "idem"); err != nil {
			t.Fatalf("Sign iter %d: %v", i, err)
		}
	}
	if got := mock.signCalls.Load(); got != 1 {
		t.Fatalf("signCalls = %d, want 1", got)
	}
}

func TestSignReturnedCacheIsCopy(t *testing.T) {
	mock := &mockKMSClient{signResult: []byte{1, 2, 3, 4}}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	digest := make([]byte, 32)
	sig1, err := s.Sign(context.Background(), digest, "idem")
	if err != nil {
		t.Fatalf("Sign1: %v", err)
	}
	for i := range sig1 {
		sig1[i] = 0xff
	}
	sig2, err := s.Sign(context.Background(), digest, "idem")
	if err != nil {
		t.Fatalf("Sign2: %v", err)
	}
	if !bytes.Equal(sig2, []byte{1, 2, 3, 4}) {
		t.Fatalf("sig2 contaminated: %v", sig2)
	}
}

// ---------- Sign 参数校验 ----------

func TestSignEmptyIdempotency(t *testing.T) {
	mock := &mockKMSClient{}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := s.Sign(context.Background(), make([]byte, 32), "")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestSignBadDigestLen(t *testing.T) {
	mock := &mockKMSClient{}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := s.Sign(context.Background(), make([]byte, 16), "idem")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestSignUnsupportedAlgorithmDefense(t *testing.T) {
	mock := &mockKMSClient{}
	s := NewWithClient(mock, "k", "BOGUS")
	_, err := s.Sign(context.Background(), nil, "idem")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// ---------- 错误归一 ----------

func makeSDKErr(status int, code string) *teasdk.SDKError {
	st := status
	c := code
	return &teasdk.SDKError{StatusCode: &st, Code: &c}
}

func TestSignErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		mockErr error
		want    error
	}{
		{"sdk 401", makeSDKErr(401, ""), sign.ErrAuthFailed},
		{"sdk 403", makeSDKErr(403, ""), sign.ErrAuthFailed},
		{"sdk 404", makeSDKErr(404, ""), sign.ErrKeyNotFound},
		{"sdk 500", makeSDKErr(500, ""), sign.ErrUpstreamUnavailable},
		{"sdk 503", makeSDKErr(503, ""), sign.ErrUpstreamUnavailable},
		{"sdk 0", makeSDKErr(0, ""), sign.ErrUpstreamUnavailable},
		{"sdk code Forbidden", makeSDKErr(200, "Forbidden.RAM"), sign.ErrAuthFailed},
		{"sdk code SignatureDoesNotMatch", makeSDKErr(200, "SignatureDoesNotMatch"), sign.ErrAuthFailed},
		{"sdk code AccessDenied", makeSDKErr(200, "AccessDenied"), sign.ErrAuthFailed},
		{"sdk code Unauthorized", makeSDKErr(200, "Unauthorized"), sign.ErrAuthFailed},
		{"sdk code InvalidAccessKeyId", makeSDKErr(200, "InvalidAccessKeyId.NotFound"), sign.ErrAuthFailed},
		{"sdk code KeyNotFound", makeSDKErr(200, "Key.NotFound"), sign.ErrKeyNotFound},
		{"sdk code Disabled", makeSDKErr(200, "Key.Disabled"), sign.ErrKeyNotFound},
		{"sdk code KeyState", makeSDKErr(200, "KeyStateError"), sign.ErrKeyNotFound},
		{"sdk code Throttling", makeSDKErr(200, "Throttling.User"), sign.ErrUpstreamUnavailable},
		{"sdk code ServiceBusy", makeSDKErr(200, "ServiceBusy"), sign.ErrUpstreamUnavailable},
		{"sdk code InternalError", makeSDKErr(200, "InternalError"), sign.ErrUpstreamUnavailable},
		{"sdk code InvalidParameter", makeSDKErr(200, "InvalidParameter.X"), sign.ErrInvalidInput},
		{"sdk code InvalidDigest", makeSDKErr(200, "InvalidDigest.Length"), sign.ErrInvalidInput},
		{"sdk code unknown", makeSDKErr(200, "RandomThing"), sign.ErrUpstreamUnavailable},
		{"str AccessDenied", errors.New("AccessDenied: bad creds"), sign.ErrAuthFailed},
		{"str Forbidden", errors.New("Forbidden"), sign.ErrAuthFailed},
		{"str notfound", errors.New("Key NotFound"), sign.ErrKeyNotFound},
		{"str timeout", errors.New("context deadline / timeout"), sign.ErrUpstreamUnavailable},
		{"str ServiceBusy", errors.New("ServiceBusy"), sign.ErrUpstreamUnavailable},
		{"str unknown", errors.New("totally novel error type"), sign.ErrUpstreamUnavailable},
		{"sentinel passthrough auth", sign.ErrAuthFailed, sign.ErrAuthFailed},
		{"sentinel passthrough notfound", sign.ErrKeyNotFound, sign.ErrKeyNotFound},
		{"sentinel passthrough upstream", sign.ErrUpstreamUnavailable, sign.ErrUpstreamUnavailable},
		{"sentinel passthrough invalid", sign.ErrInvalidInput, sign.ErrInvalidInput},
		{"sentinel passthrough sig invalid", sign.ErrSignatureInvalid, sign.ErrSignatureInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mock := &mockKMSClient{signErr: c.mockErr}
			s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
			_, err := s.Sign(context.Background(), make([]byte, 32), c.name)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestMapErrNil(t *testing.T) {
	if got := mapErr(nil); got != nil {
		t.Fatalf("mapErr(nil) = %v, want nil", got)
	}
}

func TestSDKErrNoStatusOrCode(t *testing.T) {
	// SDKError 没设任何字段，应走最终字符串 fallback。
	err := &teasdk.SDKError{}
	got := mapErr(err)
	if !errors.Is(got, sign.ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", got)
	}
}

// ---------- GetPublicKey / KeyVersion ----------

func TestPublicKeyHappy(t *testing.T) {
	mock := &mockKMSClient{getPubResult: []byte("-----BEGIN PUBLIC KEY-----\nfake\n-----END PUBLIC KEY-----\n")}
	v := NewVerifierWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	pemBytes, err := v.PublicKey(context.Background())
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if !bytes.Contains(pemBytes, []byte("BEGIN PUBLIC KEY")) {
		t.Fatalf("pem missing header: %q", pemBytes)
	}
}

func TestPublicKeyError(t *testing.T) {
	mock := &mockKMSClient{getPubErr: makeSDKErr(404, "Key.NotFound")}
	v := NewVerifierWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := v.PublicKey(context.Background())
	if !errors.Is(err, sign.ErrKeyNotFound) {
		t.Fatalf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestKeyVersionHappy(t *testing.T) {
	mock := &mockKMSClient{describeResult: 12345}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	v, err := s.KeyVersion(context.Background())
	if err != nil {
		t.Fatalf("KeyVersion: %v", err)
	}
	if v != 12345 {
		t.Fatalf("version = %d", v)
	}
}

func TestKeyVersionError(t *testing.T) {
	mock := &mockKMSClient{describeErr: errors.New("Throttling.User")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := s.KeyVersion(context.Background())
	if !errors.Is(err, sign.ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

// ---------- realKMSClient 响应解析 ----------

func TestParseAsymmetricSignResponse(t *testing.T) {
	if _, err := parseAsymmetricSignResponse(nil); err == nil {
		t.Fatal("nil should err")
	}
	if _, err := parseAsymmetricSignResponse(&kmssdk.AsymmetricSignResponse{}); err == nil {
		t.Fatal("nil body should err")
	}
	body := &kmssdk.AsymmetricSignResponseBody{}
	if _, err := parseAsymmetricSignResponse(&kmssdk.AsymmetricSignResponse{Body: body}); err == nil {
		t.Fatal("nil Value should err")
	}
	empty := ""
	body.Value = &empty
	if _, err := parseAsymmetricSignResponse(&kmssdk.AsymmetricSignResponse{Body: body}); err == nil {
		t.Fatal("empty Value should err")
	}
	bad := "not-base64-!!!"
	body.Value = &bad
	if _, err := parseAsymmetricSignResponse(&kmssdk.AsymmetricSignResponse{Body: body}); err == nil {
		t.Fatal("bad base64 should err")
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("sig-bytes"))
	body.Value = &encoded
	sig, err := parseAsymmetricSignResponse(&kmssdk.AsymmetricSignResponse{Body: body})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Equal(sig, []byte("sig-bytes")) {
		t.Fatalf("sig = %q", sig)
	}
}

func TestParseGetPublicKeyResponse(t *testing.T) {
	if _, err := parseGetPublicKeyResponse(nil); err == nil {
		t.Fatal("nil should err")
	}
	if _, err := parseGetPublicKeyResponse(&kmssdk.GetPublicKeyResponse{}); err == nil {
		t.Fatal("nil body should err")
	}
	body := &kmssdk.GetPublicKeyResponseBody{}
	if _, err := parseGetPublicKeyResponse(&kmssdk.GetPublicKeyResponse{Body: body}); err == nil {
		t.Fatal("nil PublicKey should err")
	}
	empty := ""
	body.PublicKey = &empty
	if _, err := parseGetPublicKeyResponse(&kmssdk.GetPublicKeyResponse{Body: body}); err == nil {
		t.Fatal("empty PublicKey should err")
	}
	// 阿里云原始返回会含字面 "\n"；确认还原后含真实换行。
	raw := "-----BEGIN PUBLIC KEY-----\\nfakebody\\n-----END PUBLIC KEY-----\\n"
	body.PublicKey = &raw
	out, err := parseGetPublicKeyResponse(&kmssdk.GetPublicKeyResponse{Body: body})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !bytes.Contains(out, []byte("-----BEGIN PUBLIC KEY-----\n")) {
		t.Fatalf("escapes not un-escaped: %q", out)
	}
	if strings.Contains(string(out), `\n`) {
		t.Fatalf("literal \\n remains: %q", out)
	}
}

func TestParseDescribeKeyVersion(t *testing.T) {
	if v := parseDescribeKeyVersion(nil); v != 0 {
		t.Fatalf("nil = %d", v)
	}
	if v := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{}); v != 0 {
		t.Fatalf("nil body = %d", v)
	}
	body := &kmssdk.DescribeKeyResponseBody{}
	if v := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body}); v != 0 {
		t.Fatalf("nil metadata = %d", v)
	}
	body.KeyMetadata = &kmssdk.DescribeKeyResponseBodyKeyMetadata{}
	if v := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body}); v != 0 {
		t.Fatalf("nil version = %d", v)
	}
	empty := ""
	body.KeyMetadata.PrimaryKeyVersion = &empty
	if v := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body}); v != 0 {
		t.Fatalf("empty version = %d", v)
	}
	verA := "version-aaa"
	body.KeyMetadata.PrimaryKeyVersion = &verA
	va := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body})
	if va == 0 {
		t.Fatalf("nonempty version hashed to 0")
	}
	verB := "version-bbb"
	body.KeyMetadata.PrimaryKeyVersion = &verB
	vb := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body})
	if vb == 0 {
		t.Fatalf("nonempty version hashed to 0")
	}
	if va == vb {
		t.Fatalf("different versions hashed to same value: %d", va)
	}
	// 同 input 二次 hash 必须稳定。
	body.KeyMetadata.PrimaryKeyVersion = &verA
	if got := parseDescribeKeyVersion(&kmssdk.DescribeKeyResponse{Body: body}); got != va {
		t.Fatalf("unstable hash: %d vs %d", va, got)
	}
}

// ---------- realKMSClient SDK fn-pointer wiring ----------

func TestRealKMSClientFunctionPointers(t *testing.T) {
	sigB64 := base64.StdEncoding.EncodeToString([]byte("OK"))
	pubKey := "-----BEGIN PUBLIC KEY-----\\nfakebody\\n-----END PUBLIC KEY-----\\n"
	keyVer := "version-x"
	r := &realKMSClient{
		sdkSign: func(req *kmssdk.AsymmetricSignRequest) (*kmssdk.AsymmetricSignResponse, error) {
			// 校验 SDK 收到的 digest 是 base64 编码的。
			if req.Digest == nil {
				t.Fatalf("Digest nil")
			}
			if _, err := base64.StdEncoding.DecodeString(*req.Digest); err != nil {
				t.Fatalf("Digest not base64: %v", err)
			}
			return &kmssdk.AsymmetricSignResponse{Body: &kmssdk.AsymmetricSignResponseBody{Value: &sigB64}}, nil
		},
		sdkGetPub: func(_ *kmssdk.GetPublicKeyRequest) (*kmssdk.GetPublicKeyResponse, error) {
			return &kmssdk.GetPublicKeyResponse{Body: &kmssdk.GetPublicKeyResponseBody{PublicKey: &pubKey}}, nil
		},
		sdkDescribe: func(_ *kmssdk.DescribeKeyRequest) (*kmssdk.DescribeKeyResponse, error) {
			return &kmssdk.DescribeKeyResponse{Body: &kmssdk.DescribeKeyResponseBody{
				KeyMetadata: &kmssdk.DescribeKeyResponseBodyKeyMetadata{PrimaryKeyVersion: &keyVer},
			}}, nil
		},
	}
	sig, err := r.Sign(context.Background(), "k", "ECDSA_SHA_256", []byte("digest-32-bytes-padding-padding!"), "idem")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !bytes.Equal(sig, []byte("OK")) {
		t.Fatalf("sig = %q", sig)
	}
	pub, err := r.GetPublicKey(context.Background(), "k")
	if err != nil {
		t.Fatalf("GetPublicKey: %v", err)
	}
	if !bytes.Contains(pub, []byte("BEGIN PUBLIC KEY")) {
		t.Fatalf("pub: %q", pub)
	}
	ver, err := r.DescribeKey(context.Background(), "k")
	if err != nil {
		t.Fatalf("DescribeKey: %v", err)
	}
	if ver == 0 {
		t.Fatalf("ver = 0, want non-zero hash")
	}
}

func TestRealKMSClientSignErrPropagation(t *testing.T) {
	r := &realKMSClient{
		sdkSign: func(_ *kmssdk.AsymmetricSignRequest) (*kmssdk.AsymmetricSignResponse, error) {
			return nil, errors.New("network down")
		},
	}
	if _, err := r.Sign(context.Background(), "k", "ECDSA_SHA_256", make([]byte, 32), "idem"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRealKMSClientGetPubErrPropagation(t *testing.T) {
	r := &realKMSClient{
		sdkGetPub: func(_ *kmssdk.GetPublicKeyRequest) (*kmssdk.GetPublicKeyResponse, error) {
			return nil, errors.New("denied")
		},
	}
	if _, err := r.GetPublicKey(context.Background(), "k"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRealKMSClientDescribeErrPropagation(t *testing.T) {
	r := &realKMSClient{
		sdkDescribe: func(_ *kmssdk.DescribeKeyRequest) (*kmssdk.DescribeKeyResponse, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := r.DescribeKey(context.Background(), "k"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRealKMSClientCtxCanceled(t *testing.T) {
	r := &realKMSClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Sign(ctx, "k", "ECDSA_SHA_256", make([]byte, 32), "idem"); err == nil {
		t.Fatalf("Sign should err on canceled ctx")
	}
	if _, err := r.GetPublicKey(ctx, "k"); err == nil {
		t.Fatalf("GetPublicKey should err on canceled ctx")
	}
	if _, err := r.DescribeKey(ctx, "k"); err == nil {
		t.Fatalf("DescribeKey should err on canceled ctx")
	}
}
