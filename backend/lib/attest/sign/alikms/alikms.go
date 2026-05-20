// Package alikms 是 lib/attest/sign 的阿里云 KMS 实现：调用阿里云 KMS 的
// AsymmetricSign / GetPublicKey / DescribeKey 接口，配 idempotency 缓存
// 兜底（D4）。
//
// 与 lib/cert/vault/alikms（envelope encryption / Encrypt+Decrypt）完全
// 独立 — 本包只做 asymmetric sign / GetPublicKey，不复用任何 vault 包代码。
//
// 阿里云 KMS 的 AsymmetricSign API：
//   - Digest 入参是 base64 字符串（在 SDK 层编码）；
//   - 返回 Value 是 base64 字符串（在响应解析层解码）；
//   - 算法常量名称与 AWS KMS 不完全一致，但本包接受同一套 sign 包常量并做映射。
//
// 测试通过 KMSClient interface 注入 mock，不依赖真实阿里云。
package alikms

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	kmssdk "github.com/alibabacloud-go/kms-20160120/v3/client"
	teasdk "github.com/alibabacloud-go/tea/tea"

	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/sign/internal/idempcache"
)

// KMSClient 抽象本包使用的阿里云 KMS 调用面，便于测试注入 mock。
type KMSClient interface {
	// Sign 用 keyID + algorithm 对 digest 做签名。返回原始（已 base64 解码）
	// signature 字节。idempotencyKey 由实现透传到 RequestId / KeyVersionId
	// 等阿里云幂等机制字段。
	Sign(ctx context.Context, keyID, algorithm string, digest []byte, idempotencyKey string) ([]byte, error)

	// GetPublicKey 返回 keyID 对应的 PEM 公钥。
	GetPublicKey(ctx context.Context, keyID string) ([]byte, error)

	// DescribeKey 返回 keyID 的版本号。阿里云 KMS 的 PrimaryKeyVersion
	// 是 UUID 字符串；realKMSClient 用 FNV-32 把它哈希成 int 近似 "整数
	// 版本"（仅用于轮换检测，不是密码学保证）。
	DescribeKey(ctx context.Context, keyID string) (version int, err error)
}

// Config 是构造阿里云 KMS Signer / Verifier 所需的全部参数。
type Config struct {
	// RegionID 是阿里云 region，例如 "cn-hangzhou"。必填。
	RegionID string

	// AccessKeyID / AccessKeySecret 是 RAM 子账号凭据；必须有对应 KMS
	// CMK 的 AsymmetricSign + GetPublicKey + DescribeKey 权限。必填。
	AccessKeyID     string
	AccessKeySecret string

	// KeyID 是 CMK 的 alias 或 ID，例如 "key-cn-hangzhou-xxxxx"。必填。
	KeyID string

	// Algorithm 是签名算法（见 sign 包常量）。默认 ECDSA_SHA_256。
	// 必须与阿里云 CMK 的 KeySpec 兼容（EC_P256 / RSA_2048 等）。
	Algorithm string
}

// signer 是 sign.Signer 的阿里云 KMS 实现。
type signer struct {
	client    KMSClient
	keyID     string
	algorithm string

	// idempCache：同 awskms.signer.idempCache，进程内 (idempotencyKey →
	// signature) bounded 缓存。FIFO 驱逐 + cap=10000，长跑（90 天 KMS 实例
	// 生命周期）下内存恒定；驱逐窗口远大于阿里云 KMS 服务端去重窗口，
	// 不会触发重复签名审计。
	idempCache *idempcache.Cache
}

// verifier 是 sign.Verifier 的阿里云 KMS 实现。
type verifier struct {
	client    KMSClient
	keyID     string
	algorithm string
}

// New 构造阿里云 KMS Signer。任一必填字段缺失返回 ErrInvalidInput。
//
// 网络 / 凭据问题在首次 Sign 时才暴露；本函数只做参数校验 + SDK client
// 初始化。
func New(cfg Config) (sign.Signer, error) {
	cli, alg, err := buildClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewWithClient(cli, cfg.KeyID, alg), nil
}

// NewVerifier 构造阿里云 KMS Verifier。参数校验同 New。
func NewVerifier(cfg Config) (sign.Verifier, error) {
	cli, alg, err := buildClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewVerifierWithClient(cli, cfg.KeyID, alg), nil
}

// NewWithClient 用自定义 KMSClient 构造 Signer，专供测试 / DI。
func NewWithClient(client KMSClient, keyID, algorithm string) sign.Signer {
	return &signer{
		client:     client,
		keyID:      keyID,
		algorithm:  algorithm,
		idempCache: idempcache.New(0),
	}
}

// NewVerifierWithClient 用自定义 KMSClient 构造 Verifier，专供测试 / DI。
func NewVerifierWithClient(client KMSClient, keyID, algorithm string) sign.Verifier {
	return &verifier{client: client, keyID: keyID, algorithm: algorithm}
}

func buildClient(cfg Config) (KMSClient, string, error) {
	if cfg.RegionID == "" {
		return nil, "", fmt.Errorf("%w: RegionID is empty", sign.ErrInvalidInput)
	}
	if cfg.AccessKeyID == "" {
		return nil, "", fmt.Errorf("%w: AccessKeyID is empty", sign.ErrInvalidInput)
	}
	if cfg.AccessKeySecret == "" {
		return nil, "", fmt.Errorf("%w: AccessKeySecret is empty", sign.ErrInvalidInput)
	}
	if cfg.KeyID == "" {
		return nil, "", fmt.Errorf("%w: KeyID is empty", sign.ErrInvalidInput)
	}
	alg := cfg.Algorithm
	if alg == "" {
		alg = sign.AlgorithmECDSASHA256
	}
	if _, err := sign.HashAlgFor(alg); err != nil {
		return nil, "", fmt.Errorf("%w: unsupported Algorithm %q", sign.ErrInvalidInput, alg)
	}

	regionID := cfg.RegionID
	ak := cfg.AccessKeyID
	sk := cfg.AccessKeySecret
	endpoint := fmt.Sprintf("kms.%s.aliyuncs.com", regionID)
	sdkClient, err := kmssdk.NewClient(&openapiutil.Config{
		AccessKeyId:     &ak,
		AccessKeySecret: &sk,
		RegionId:        &regionID,
		Endpoint:        &endpoint,
	})
	if err != nil {
		return nil, "", fmt.Errorf("alikms: init SDK client: %w", err)
	}
	return &realKMSClient{sdk: sdkClient}, alg, nil
}

func (s *signer) KeyID() string     { return s.keyID }
func (s *signer) Algorithm() string { return s.algorithm }

func (s *signer) KeyVersion(ctx context.Context) (int, error) {
	v, err := s.client.DescribeKey(ctx, s.keyID)
	if err != nil {
		return 0, mapErr(err)
	}
	return v, nil
}

func (s *signer) Sign(ctx context.Context, digest []byte, idempotencyKey string) ([]byte, error) {
	if idempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotencyKey is empty", sign.ErrInvalidInput)
	}
	want := expectedDigestLen(s.algorithm)
	if want == 0 {
		return nil, fmt.Errorf("%w: unsupported Algorithm %q", sign.ErrInvalidInput, s.algorithm)
	}
	if len(digest) != want {
		return nil, fmt.Errorf("%w: digest length %d, want %d for %s", sign.ErrInvalidInput, len(digest), want, s.algorithm)
	}

	if cached, ok := s.idempCache.Load(idempotencyKey); ok {
		return cached, nil
	}

	sig, err := s.client.Sign(ctx, s.keyID, s.algorithm, digest, idempotencyKey)
	if err != nil {
		return nil, mapErr(err)
	}
	stored, _ := s.idempCache.LoadOrStore(idempotencyKey, sig)
	return stored, nil
}

func (v *verifier) KeyID() string     { return v.keyID }
func (v *verifier) Algorithm() string { return v.algorithm }

func (v *verifier) PublicKey(ctx context.Context) ([]byte, error) {
	pem, err := v.client.GetPublicKey(ctx, v.keyID)
	if err != nil {
		return nil, mapErr(err)
	}
	return pem, nil
}

// expectedDigestLen 镜像 sign 包私有同名函数。
func expectedDigestLen(algorithm string) int {
	h, err := sign.HashAlgFor(algorithm)
	if err != nil {
		return 0
	}
	return h.Size()
}

// mapErr 把 SDK / mock 错误归一为 sign 包 sentinel。识别 tea.SDKError
// 的 StatusCode / Code 字段；同时支持已是 sentinel 的透传与字符串兜底。
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	// 已是 sentinel 直接透传。
	for _, sentinel := range []error{
		sign.ErrUpstreamUnavailable,
		sign.ErrAuthFailed,
		sign.ErrInvalidInput,
		sign.ErrKeyNotFound,
		sign.ErrSignatureInvalid,
	} {
		if errors.Is(err, sentinel) {
			return err
		}
	}

	// tea.SDKError 是阿里云 SDK 的标准错误类型。
	var sdkErr *teasdk.SDKError
	if errors.As(err, &sdkErr) {
		if sdkErr.StatusCode != nil {
			sc := *sdkErr.StatusCode
			switch {
			case sc == 401 || sc == 403:
				return fmt.Errorf("%w: %v", sign.ErrAuthFailed, err)
			case sc == 404:
				return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
			case sc >= 500 || sc == 0:
				return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
			}
		}
		if sdkErr.Code != nil {
			code := strings.ToLower(*sdkErr.Code)
			switch {
			case strings.Contains(code, "forbidden") || strings.Contains(code, "unauthorized") || strings.Contains(code, "accessdenied") || strings.Contains(code, "invalidaccesskeyid") || strings.Contains(code, "signaturedoesnotmatch"):
				return fmt.Errorf("%w: %v", sign.ErrAuthFailed, err)
			case strings.Contains(code, "notfound") || strings.Contains(code, "keystate") || strings.Contains(code, "disabled"):
				return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
			case strings.Contains(code, "throttling") || strings.Contains(code, "servicebusy") || strings.Contains(code, "internalerror"):
				return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
			case strings.Contains(code, "invalidparameter") || strings.Contains(code, "invalidargument") || strings.Contains(code, "invaliddigest"):
				return fmt.Errorf("%w: %v", sign.ErrInvalidInput, err)
			}
		}
	}

	// 字符串兜底（mock / 旧 SDK 路径）。
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "accessdenied") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "401") || strings.Contains(msg, "403"):
		return fmt.Errorf("%w: %v", sign.ErrAuthFailed, err)
	case strings.Contains(msg, "notfound") || strings.Contains(msg, "not found") || strings.Contains(msg, "404"):
		return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
	case strings.Contains(msg, "throttl") ||
		strings.Contains(msg, "503") || strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") || strings.Contains(msg, "504") ||
		strings.Contains(msg, "5xx") || strings.Contains(msg, "servicebusy") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused"):
		return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
	}
	return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
}

// ---------------- realKMSClient: 阿里云 SDK 适配 ----------------

// realKMSClient 把 alibabacloud-go/kms-20160120/v3 SDK 适配成 KMSClient。
//
// SDK 调用通过 sdkSign / sdkGetPub / sdkDescribe 函数指针注入，便于测试
// 覆盖响应解析路径而不打网络。
type realKMSClient struct {
	sdk         *kmssdk.Client
	sdkSign     func(*kmssdk.AsymmetricSignRequest) (*kmssdk.AsymmetricSignResponse, error)
	sdkGetPub   func(*kmssdk.GetPublicKeyRequest) (*kmssdk.GetPublicKeyResponse, error)
	sdkDescribe func(*kmssdk.DescribeKeyRequest) (*kmssdk.DescribeKeyResponse, error)
}

func (r *realKMSClient) callSign(req *kmssdk.AsymmetricSignRequest) (*kmssdk.AsymmetricSignResponse, error) {
	if r.sdkSign != nil {
		return r.sdkSign(req)
	}
	return r.sdk.AsymmetricSign(req)
}

func (r *realKMSClient) callGetPub(req *kmssdk.GetPublicKeyRequest) (*kmssdk.GetPublicKeyResponse, error) {
	if r.sdkGetPub != nil {
		return r.sdkGetPub(req)
	}
	return r.sdk.GetPublicKey(req)
}

func (r *realKMSClient) callDescribe(req *kmssdk.DescribeKeyRequest) (*kmssdk.DescribeKeyResponse, error) {
	if r.sdkDescribe != nil {
		return r.sdkDescribe(req)
	}
	return r.sdk.DescribeKey(req)
}

// Sign 调用阿里云 KMS AsymmetricSign API。digest 用 base64 编码后传给
// SDK；返回的 signature 也是 base64 解码后的原始字节。
//
// idempotencyKey 在阿里云 SDK 上没有直接对应字段（AsymmetricSignRequest
// 没有 ClientToken）。我们在 SDK 层把它写入请求的 KeyVersionId 字段是
// 不安全的（会改变签名密钥版本）；最佳实践是依赖 idcd 进程内 sync.Map
// 缓存 + 上层 WAL 完成 D4 等价语义。本实现把 idempotencyKey 透传给
// realKMSClient 但目前不消费（mock 仍能断言收到了正确值）；如未来阿里
// 云 KMS 添加 ClientToken 字段或我们 fork SDK 增加 header 注入再启用。
func (r *realKMSClient) Sign(ctx context.Context, keyID, algorithm string, digest []byte, idempotencyKey string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	_ = idempotencyKey // 见上文注释；进程内 sync.Map 已兜底。
	digestB64 := base64.StdEncoding.EncodeToString(digest)
	keyIDCopy := keyID
	algCopy := algorithm
	req := &kmssdk.AsymmetricSignRequest{
		KeyId:     &keyIDCopy,
		Algorithm: &algCopy,
		Digest:    &digestB64,
	}
	resp, err := r.callSign(req)
	if err != nil {
		return nil, err
	}
	return parseAsymmetricSignResponse(resp)
}

func (r *realKMSClient) GetPublicKey(ctx context.Context, keyID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keyIDCopy := keyID
	req := &kmssdk.GetPublicKeyRequest{KeyId: &keyIDCopy}
	resp, err := r.callGetPub(req)
	if err != nil {
		return nil, err
	}
	return parseGetPublicKeyResponse(resp)
}

func (r *realKMSClient) DescribeKey(ctx context.Context, keyID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	keyIDCopy := keyID
	resp, err := r.callDescribe(&kmssdk.DescribeKeyRequest{KeyId: &keyIDCopy})
	if err != nil {
		return 0, err
	}
	return parseDescribeKeyVersion(resp), nil
}

// parseAsymmetricSignResponse 解 SDK AsymmetricSign 响应：base64 解码 Value。
func parseAsymmetricSignResponse(resp *kmssdk.AsymmetricSignResponse) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("alikms: AsymmetricSign returned empty body")
	}
	if resp.Body.Value == nil || *resp.Body.Value == "" {
		return nil, errors.New("alikms: AsymmetricSign returned empty Value")
	}
	sig, err := base64.StdEncoding.DecodeString(*resp.Body.Value)
	if err != nil {
		return nil, fmt.Errorf("alikms: decode Value base64: %w", err)
	}
	return sig, nil
}

// parseGetPublicKeyResponse 解 SDK GetPublicKey 响应：PublicKey 已经是
// PEM 字符串（阿里云不返回裸 DER），但会包含转义的 \n；做一次还原。
func parseGetPublicKeyResponse(resp *kmssdk.GetPublicKeyResponse) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("alikms: GetPublicKey returned empty body")
	}
	if resp.Body.PublicKey == nil || *resp.Body.PublicKey == "" {
		return nil, errors.New("alikms: GetPublicKey returned empty PublicKey")
	}
	// 阿里云有时返回字面 "\\n"，按需还原。
	pem := strings.ReplaceAll(*resp.Body.PublicKey, "\\n", "\n")
	return []byte(pem), nil
}

// parseDescribeKeyVersion 把 DescribeKey 响应里的 PrimaryKeyVersion（UUID
// 字符串）哈希成稳定 int — 用于轮换检测，不需要密码学安全。
func parseDescribeKeyVersion(resp *kmssdk.DescribeKeyResponse) int {
	if resp == nil || resp.Body == nil || resp.Body.KeyMetadata == nil {
		return 0
	}
	v := resp.Body.KeyMetadata.PrimaryKeyVersion
	if v == nil || *v == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(*v))
	return int(h.Sum32())
}
