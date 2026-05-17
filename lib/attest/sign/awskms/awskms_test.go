package awskms

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	kmssdk "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/kite365/idcd/lib/attest/sign"
)

// ---------- mockKMSClient ----------
//
// 全程模拟 AWS KMS：Sign 返回固定字节流 + 命中预设错误注入；GetPublicKey
// 返回预设 PEM；DescribeKey 返回预设版本号。所有调用计数原子记录便于断言
// idempotency 缓存命中后是否真的少调。

type mockKMSClient struct {
	signResult       []byte
	signErr          error
	getPubResult     []byte
	getPubErr        error
	describeResult   int
	describeErr      error
	signCalls        atomic.Int64
	getPubCalls      atomic.Int64
	describeCalls    atomic.Int64
	lastIdempotency  string
	lastDigest       []byte
	lastSignKeyID    string
	lastSignAlg      string
	mu               sync.Mutex // 仅保护 lastXxx 字段（计数走 atomic）
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
		{"region empty", Config{KeyID: "k"}},
		{"keyid empty", Config{Region: "us-east-1"}},
		{"half cred 1", Config{Region: "us-east-1", KeyID: "k", AccessKeyID: "a"}},
		{"half cred 2", Config{Region: "us-east-1", KeyID: "k", SecretAccessKey: "s"}},
		{"bad alg", Config{Region: "us-east-1", KeyID: "k", Algorithm: "WAT"}},
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

func TestNewDefaults(t *testing.T) {
	// 留空 AccessKeyID + SecretAccessKey 走默认 credential chain；
	// LoadDefaultConfig 在测试环境通常不报错（lazy 取凭据）。
	s, err := New(Config{Region: "us-east-1", KeyID: "alias/test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.KeyID() != "alias/test" {
		t.Fatalf("KeyID = %q, want alias/test", s.KeyID())
	}
	if s.Algorithm() != sign.AlgorithmECDSASHA256 {
		t.Fatalf("Algorithm = %q, want %q", s.Algorithm(), sign.AlgorithmECDSASHA256)
	}
}

func TestNewStaticCredentials(t *testing.T) {
	s, err := New(Config{
		Region: "us-east-1", KeyID: "k", AccessKeyID: "a", SecretAccessKey: "s",
		Algorithm: sign.AlgorithmRSAPSSSHA384,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s.Algorithm() != sign.AlgorithmRSAPSSSHA384 {
		t.Fatalf("Algorithm = %q", s.Algorithm())
	}
}

func TestNewVerifierStaticCredentials(t *testing.T) {
	v, err := NewVerifier(Config{
		Region: "us-east-1", KeyID: "k", AccessKeyID: "a", SecretAccessKey: "s",
	})
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	if v.KeyID() != "k" {
		t.Fatalf("KeyID = %q", v.KeyID())
	}
	if v.Algorithm() != sign.AlgorithmECDSASHA256 {
		t.Fatalf("Algorithm = %q", v.Algorithm())
	}
}

// ---------- Sign happy + idempotency ----------

func TestSignHappy(t *testing.T) {
	mock := &mockKMSClient{signResult: []byte("sig-bytes")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	digest := make([]byte, 32) // SHA-256 digest len
	sig, err := s.Sign(context.Background(), digest, "idem-1")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !bytes.Equal(sig, []byte("sig-bytes")) {
		t.Fatalf("sig = %q", sig)
	}
	if got := mock.signCalls.Load(); got != 1 {
		t.Fatalf("signCalls = %d, want 1", got)
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
		sig, err := s.Sign(context.Background(), digest, "idem-stable")
		if err != nil {
			t.Fatalf("Sign iter %d: %v", i, err)
		}
		if !bytes.Equal(sig, []byte("sig-bytes")) {
			t.Fatalf("sig iter %d = %q", i, sig)
		}
	}
	if got := mock.signCalls.Load(); got != 1 {
		t.Fatalf("signCalls = %d, want 1 (idempotency cache should suppress)", got)
	}
}

func TestSignIdempotencyDifferentKeysAllPassThrough(t *testing.T) {
	mock := &mockKMSClient{signResult: []byte("sig-bytes")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	digest := make([]byte, 32)
	for i, key := range []string{"a", "b", "c"} {
		if _, err := s.Sign(context.Background(), digest, key); err != nil {
			t.Fatalf("Sign iter %d: %v", i, err)
		}
	}
	if got := mock.signCalls.Load(); got != 3 {
		t.Fatalf("signCalls = %d, want 3", got)
	}
}

func TestSignReturnedCacheIsCopy(t *testing.T) {
	// 修改返回的 sig 不应该污染缓存。
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
	if mock.signCalls.Load() != 0 {
		t.Fatalf("should not call KMS on bad input")
	}
}

func TestSignBadDigestLen(t *testing.T) {
	cases := []struct {
		alg     string
		digest  []byte
	}{
		{sign.AlgorithmECDSASHA256, make([]byte, 31)},
		{sign.AlgorithmECDSASHA256, make([]byte, 33)},
		{sign.AlgorithmECDSASHA384, make([]byte, 32)}, // wrong for SHA-384
		{sign.AlgorithmECDSASHA512, make([]byte, 32)},
		{sign.AlgorithmECDSASHA256, nil},
	}
	for _, c := range cases {
		t.Run(c.alg, func(t *testing.T) {
			mock := &mockKMSClient{}
			s := NewWithClient(mock, "k", c.alg)
			_, err := s.Sign(context.Background(), c.digest, "idem")
			if !errors.Is(err, sign.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if mock.signCalls.Load() != 0 {
				t.Fatalf("should not call KMS on bad digest len")
			}
		})
	}
}

func TestSignUnsupportedAlgorithmDefense(t *testing.T) {
	// NewWithClient 不校验算法（DI 路径），直接构造 bad algorithm signer
	// 验证 Sign 内的防御性 expectedDigestLen == 0 分支。
	mock := &mockKMSClient{}
	s := NewWithClient(mock, "k", "BOGUS")
	_, err := s.Sign(context.Background(), nil, "idem")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// ---------- Sign 错误归一 ----------

func TestSignErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		mockErr error
		want    error
	}{
		{"NotFoundException", &kmstypes.NotFoundException{}, sign.ErrKeyNotFound},
		{"DisabledException", &kmstypes.DisabledException{}, sign.ErrKeyNotFound},
		{"KeyUnavailableException", &kmstypes.KeyUnavailableException{}, sign.ErrKeyNotFound},
		{"InvalidKeyUsageException", &kmstypes.InvalidKeyUsageException{}, sign.ErrInvalidInput},
		{"http 401", makeHTTPRespErr(401), sign.ErrAuthFailed},
		{"http 403", makeHTTPRespErr(403), sign.ErrAuthFailed},
		{"http 404", makeHTTPRespErr(404), sign.ErrKeyNotFound},
		{"http 500", makeHTTPRespErr(500), sign.ErrUpstreamUnavailable},
		{"http 503", makeHTTPRespErr(503), sign.ErrUpstreamUnavailable},
		{"http 0 (network)", makeHTTPRespErr(0), sign.ErrUpstreamUnavailable},
		{"http 418 (teapot, unmapped)", makeHTTPRespErr(418), sign.ErrUpstreamUnavailable},
		{"str AccessDenied", errors.New("AccessDenied: bad creds"), sign.ErrAuthFailed},
		{"str unauthorized", errors.New("unauthorized"), sign.ErrAuthFailed},
		{"str notfound", errors.New("KeyNotFound"), sign.ErrKeyNotFound},
		{"str timeout", errors.New("context deadline / timeout"), sign.ErrUpstreamUnavailable},
		{"str throttle", errors.New("ThrottlingException"), sign.ErrUpstreamUnavailable},
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
			// 用唯一 idempotency 避免缓存命中干扰
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

// makeHTTPRespErr 构造一个 smithy ResponseError，HTTPStatusCode == status。
// status == 0 时不附 Response，模拟纯网络错。
func makeHTTPRespErr(status int) error {
	var resp *smithyhttp.Response
	if status != 0 {
		resp = &smithyhttp.Response{Response: &http.Response{StatusCode: status}}
	} else {
		// status 0：仍需 Response 非 nil 才能调 HTTPStatusCode；构造一个 0 status。
		resp = &smithyhttp.Response{Response: &http.Response{StatusCode: 0}}
	}
	return &smithyhttp.ResponseError{
		Response: resp,
		Err:      errors.New("simulated http error"),
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
	mock := &mockKMSClient{getPubErr: &kmstypes.NotFoundException{}}
	v := NewVerifierWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := v.PublicKey(context.Background())
	if !errors.Is(err, sign.ErrKeyNotFound) {
		t.Fatalf("err = %v, want ErrKeyNotFound", err)
	}
}

func TestKeyVersionHappy(t *testing.T) {
	mock := &mockKMSClient{describeResult: 3}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	v, err := s.KeyVersion(context.Background())
	if err != nil {
		t.Fatalf("KeyVersion: %v", err)
	}
	if v != 3 {
		t.Fatalf("version = %d, want 3", v)
	}
}

func TestKeyVersionError(t *testing.T) {
	mock := &mockKMSClient{describeErr: errors.New("ThrottlingException: rate exceeded")}
	s := NewWithClient(mock, "k", sign.AlgorithmECDSASHA256)
	_, err := s.KeyVersion(context.Background())
	if !errors.Is(err, sign.ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

// ---------- realKMSClient 响应解析 ----------

func TestParseSignOutput(t *testing.T) {
	if _, err := parseSignOutput(nil); err == nil {
		t.Fatal("parseSignOutput(nil) should err")
	}
	if _, err := parseSignOutput(&kmssdk.SignOutput{}); err == nil {
		t.Fatal("empty Signature should err")
	}
	out := &kmssdk.SignOutput{Signature: []byte{1, 2, 3}}
	sig, err := parseSignOutput(out)
	if err != nil {
		t.Fatalf("parseSignOutput: %v", err)
	}
	if !bytes.Equal(sig, []byte{1, 2, 3}) {
		t.Fatalf("sig = %v", sig)
	}
	// 应该是拷贝，不能与 out.Signature 共享底层数组。
	sig[0] = 0xff
	if out.Signature[0] != 1 {
		t.Fatalf("parseSignOutput should copy bytes")
	}
}

func TestParseGetPublicKeyOutput(t *testing.T) {
	if _, err := parseGetPublicKeyOutput(nil); err == nil {
		t.Fatal("parseGetPublicKeyOutput(nil) should err")
	}
	if _, err := parseGetPublicKeyOutput(&kmssdk.GetPublicKeyOutput{}); err == nil {
		t.Fatal("empty PublicKey should err")
	}

	// 用真实的 PKIX DER 公钥校验 PEM 输出可被 stdlib parse。
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pemOut, err := parseGetPublicKeyOutput(&kmssdk.GetPublicKeyOutput{PublicKey: der})
	if err != nil {
		t.Fatalf("parseGetPublicKeyOutput: %v", err)
	}
	if !strings.HasPrefix(string(pemOut), "-----BEGIN PUBLIC KEY-----\n") {
		t.Fatalf("pem missing header: %q", pemOut)
	}
	block, _ := pem.Decode(pemOut)
	if block == nil {
		t.Fatalf("pem.Decode failed")
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		t.Fatalf("ParsePKIXPublicKey on round-trip: %v", err)
	}
}

func TestDerToPEMLineWrap(t *testing.T) {
	// 大于 64 字节的输入触发换行循环。
	der := make([]byte, 200)
	for i := range der {
		der[i] = byte(i)
	}
	pem := derToPEM(der)
	lines := strings.Split(string(pem), "\n")
	// header + N body lines + footer + trailing ""
	if len(lines) < 5 {
		t.Fatalf("too few lines: %d", len(lines))
	}
	if lines[0] != "-----BEGIN PUBLIC KEY-----" {
		t.Fatalf("header: %q", lines[0])
	}
	if lines[len(lines)-2] != "-----END PUBLIC KEY-----" {
		t.Fatalf("footer: %q", lines[len(lines)-2])
	}
	// 中间每行 <= 64 字符。
	for i, l := range lines[1 : len(lines)-2] {
		if len(l) > 64 {
			t.Fatalf("line %d too long: %d chars", i+1, len(l))
		}
	}
}

// ---------- realKMSClient SDK fn-pointer wiring ----------
//
// realKMSClient 的 callXxx 调用通过 sdkSign / sdkGetPub / sdkDescribe /
// sdkListRotations 函数指针注入，便于覆盖 SDK 适配层的非网络逻辑。
// 这里跑一遍 happy path 确认函数指针确实被使用。

func TestRealKMSClientFunctionPointers(t *testing.T) {
	r := &realKMSClient{
		sdkSign: func(_ context.Context, in *kmssdk.SignInput, _ ...func(*kmssdk.Options)) (*kmssdk.SignOutput, error) {
			return &kmssdk.SignOutput{Signature: []byte("OK")}, nil
		},
		sdkGetPub: func(_ context.Context, _ *kmssdk.GetPublicKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.GetPublicKeyOutput, error) {
			return &kmssdk.GetPublicKeyOutput{PublicKey: []byte{0xde, 0xad}}, nil
		},
		sdkDescribe: func(_ context.Context, _ *kmssdk.DescribeKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error) {
			return &kmssdk.DescribeKeyOutput{}, nil
		},
		sdkListRotations: func(_ context.Context, _ *kmssdk.ListKeyRotationsInput, _ ...func(*kmssdk.Options)) (*kmssdk.ListKeyRotationsOutput, error) {
			return &kmssdk.ListKeyRotationsOutput{Rotations: make([]kmstypes.RotationsListEntry, 7)}, nil
		},
	}
	sig, err := r.Sign(context.Background(), "k", "ECDSA_SHA_256", []byte("d"), "idem")
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
	if ver != 7 {
		t.Fatalf("ver = %d, want 7", ver)
	}
}

func TestRealKMSClientDescribeKeyNilListOutput(t *testing.T) {
	r := &realKMSClient{
		sdkDescribe: func(_ context.Context, _ *kmssdk.DescribeKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error) {
			return &kmssdk.DescribeKeyOutput{}, nil
		},
		sdkListRotations: func(_ context.Context, _ *kmssdk.ListKeyRotationsInput, _ ...func(*kmssdk.Options)) (*kmssdk.ListKeyRotationsOutput, error) {
			return nil, nil
		},
	}
	if _, err := r.DescribeKey(context.Background(), "k"); err == nil {
		t.Fatalf("expected error on nil ListKeyRotations output")
	}
}

func TestRealKMSClientDescribeKeyErrPropagation(t *testing.T) {
	r := &realKMSClient{
		sdkDescribe: func(_ context.Context, _ *kmssdk.DescribeKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error) {
			return nil, &kmstypes.NotFoundException{}
		},
	}
	if _, err := r.DescribeKey(context.Background(), "k"); err == nil {
		t.Fatalf("expected error")
	}

	r2 := &realKMSClient{
		sdkDescribe: func(_ context.Context, _ *kmssdk.DescribeKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error) {
			return &kmssdk.DescribeKeyOutput{}, nil
		},
		sdkListRotations: func(_ context.Context, _ *kmssdk.ListKeyRotationsInput, _ ...func(*kmssdk.Options)) (*kmssdk.ListKeyRotationsOutput, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := r2.DescribeKey(context.Background(), "k"); err == nil {
		t.Fatalf("expected error on list rotations failure")
	}
}

func TestRealKMSClientCtxCanceled(t *testing.T) {
	r := &realKMSClient{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Sign(ctx, "k", "ECDSA_SHA_256", []byte("d"), "idem"); err == nil {
		t.Fatalf("Sign should err on canceled ctx")
	}
	if _, err := r.GetPublicKey(ctx, "k"); err == nil {
		t.Fatalf("GetPublicKey should err on canceled ctx")
	}
	if _, err := r.DescribeKey(ctx, "k"); err == nil {
		t.Fatalf("DescribeKey should err on canceled ctx")
	}
}

func TestWithIdempotencyHeader(t *testing.T) {
	// 注册 middleware 到 stack；不实际发请求，仅验证 stack 接受且 ID 正确。
	fn := withIdempotencyHeader("idem-xyz")
	stack := middleware.NewStack("test", smithyhttp.NewStackRequest)
	if err := fn(stack); err != nil {
		t.Fatalf("install middleware: %v", err)
	}
	ids := stack.Build.List()
	found := false
	for _, id := range ids {
		if id == "awskmsSetIdempotencyHeader" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("middleware not registered; got %v", ids)
	}
}
