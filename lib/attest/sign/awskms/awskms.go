// Package awskms 是 lib/attest/sign 的 AWS KMS 实现：调用 AWS KMS 的
// Sign / GetPublicKey / DescribeKey 接口，配 idempotency 缓存兜底（D4）。
//
// 与 lib/cert/vault/awskms（envelope encryption / Encrypt+Decrypt）完全
// 独立 — 本包只做 asymmetric sign / GetPublicKey，不复用任何 vault 包代码。
//
// 凭据来源：
//   - 同时设置 AccessKeyID + SecretAccessKey：静态凭据；
//   - 两者均留空：走 AWS SDK 默认 credential chain
//     （IRSA / instance profile / shared config / env）；
//   - 只设其一：返回 ErrInvalidInput（半凭据是配置错误）。
//
// 测试通过 KMSClient interface 注入 mock，不依赖真实 AWS。
package awskms

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	kmssdk "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/kite365/idcd/lib/attest/sign"
)

// KMSClient 抽象本包使用的 AWS KMS 调用面，便于测试注入 mock。
// 生产实现见 realKMSClient（包内私有，封装 aws-sdk-go-v2/service/kms）。
type KMSClient interface {
	// Sign 用 keyID + algorithm 对 digest 做签名；idempotencyKey 通过
	// SDK 的 invocation ID 中间件透传给 AWS（CloudTrail 据此去重）。
	Sign(ctx context.Context, keyID, algorithm string, digest []byte, idempotencyKey string) ([]byte, error)

	// GetPublicKey 返回 keyID 对应的 PEM 公钥（PKIX SubjectPublicKeyInfo）。
	GetPublicKey(ctx context.Context, keyID string) ([]byte, error)

	// DescribeKey 返回 keyID 的版本号。AWS KMS 没有显式 int 版本，
	// realKMSClient 用 ListKeyRotations 数量近似（每次轮换 +1，起点 0）。
	DescribeKey(ctx context.Context, keyID string) (version int, err error)
}

// Config 是构造 AWS KMS Signer / Verifier 所需的全部参数。
type Config struct {
	// Region 是 AWS region，例如 "us-east-1"。必填。
	Region string

	// AccessKeyID / SecretAccessKey 是静态 IAM 凭据。两者必须同时设置
	// 或同时留空。留空时走 SDK 默认 credential chain。
	AccessKeyID     string
	SecretAccessKey string

	// KeyID 是 KMS CMK 的 alias 或 ARN，例如 "alias/verdict-sign" 或
	// "arn:aws:kms:us-east-1:123:key/uuid"。必填。
	KeyID string

	// Algorithm 是签名算法（见 sign 包常量）。默认 ECDSA_SHA_256。
	// 必须与 KMS 端 CMK 的 KeySpec 兼容（ECC_NIST_P256 / RSA_2048 等）。
	Algorithm string
}

// signer 是 sign.Signer 的 AWS KMS 实现。并发安全：client 自身保证；
// idempCache 是 sync.Map。
type signer struct {
	client    KMSClient
	keyID     string
	algorithm string

	// idempCache 是 (idempotencyKey → signature) 进程内缓存；重试同
	// key 时直接返回缓存值，不打 KMS。worker 重启会丢失，外层 WAL 兜底。
	idempCache sync.Map
}

// verifier 是 sign.Verifier 的 AWS KMS 实现。
type verifier struct {
	client    KMSClient
	keyID     string
	algorithm string
}

// New 构造 AWS KMS Signer。任一必填字段缺失返回 ErrInvalidInput；
// 半凭据（只设其一）也返回 ErrInvalidInput。
//
// 网络 / 凭据问题在首次 Sign / GetPublicKey 时才暴露；本函数只做参数
// 校验 + SDK client 初始化（默认 credential chain 也是 lazy）。
func New(cfg Config) (sign.Signer, error) {
	cli, alg, err := buildClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewWithClient(cli, cfg.KeyID, alg), nil
}

// NewVerifier 构造 AWS KMS Verifier。参数校验同 New。
func NewVerifier(cfg Config) (sign.Verifier, error) {
	cli, alg, err := buildClient(cfg)
	if err != nil {
		return nil, err
	}
	return NewVerifierWithClient(cli, cfg.KeyID, alg), nil
}

// NewWithClient 用自定义 KMSClient 构造 Signer，专供测试 / DI。
func NewWithClient(client KMSClient, keyID, algorithm string) sign.Signer {
	return &signer{client: client, keyID: keyID, algorithm: algorithm}
}

// NewVerifierWithClient 用自定义 KMSClient 构造 Verifier，专供测试 / DI。
func NewVerifierWithClient(client KMSClient, keyID, algorithm string) sign.Verifier {
	return &verifier{client: client, keyID: keyID, algorithm: algorithm}
}

// buildClient 共享 New / NewVerifier 的配置校验 + SDK 初始化逻辑。
func buildClient(cfg Config) (KMSClient, string, error) {
	if cfg.Region == "" {
		return nil, "", fmt.Errorf("%w: Region is empty", sign.ErrInvalidInput)
	}
	if cfg.KeyID == "" {
		return nil, "", fmt.Errorf("%w: KeyID is empty", sign.ErrInvalidInput)
	}
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, "", fmt.Errorf("%w: AccessKeyID and SecretAccessKey must both be set or both empty", sign.ErrInvalidInput)
	}
	alg := cfg.Algorithm
	if alg == "" {
		alg = sign.AlgorithmECDSASHA256
	}
	if _, err := sign.HashAlgFor(alg); err != nil {
		return nil, "", fmt.Errorf("%w: unsupported Algorithm %q", sign.ErrInvalidInput, alg)
	}

	opts := kmssdk.Options{Region: cfg.Region}
	if cfg.AccessKeyID != "" {
		opts.Credentials = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)
	} else {
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Region),
		)
		if err != nil {
			return nil, "", fmt.Errorf("awskms: load default AWS config: %w", err)
		}
		opts.Credentials = awsCfg.Credentials
	}
	sdkClient := kmssdk.New(opts)
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
		// 防御：构造时已校验过 algorithm，理论不会触发。
		return nil, fmt.Errorf("%w: unsupported Algorithm %q", sign.ErrInvalidInput, s.algorithm)
	}
	if len(digest) != want {
		return nil, fmt.Errorf("%w: digest length %d, want %d for %s", sign.ErrInvalidInput, len(digest), want, s.algorithm)
	}

	// 命中缓存直接返回，不打 KMS。
	if cached, ok := s.idempCache.Load(idempotencyKey); ok {
		return append([]byte(nil), cached.([]byte)...), nil
	}

	sig, err := s.client.Sign(ctx, s.keyID, s.algorithm, digest, idempotencyKey)
	if err != nil {
		return nil, mapErr(err)
	}
	// 用 LoadOrStore 处理同 key 并发 race — 第一个写入者赢，
	// 后续以缓存值为准（KMS sign 是确定性的，理论上 sig 一致；即使
	// 不一致以先到为准也不影响正确性，因为外层 WAL 只会用任一返回值）。
	stored, _ := s.idempCache.LoadOrStore(idempotencyKey, append([]byte(nil), sig...))
	return append([]byte(nil), stored.([]byte)...), nil
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

// expectedDigestLen 是 sign.expectedDigestLen 的本包镜像，避免对 sign
// 包私有符号的依赖。
func expectedDigestLen(algorithm string) int {
	h, err := sign.HashAlgFor(algorithm)
	if err != nil {
		return 0
	}
	return h.Size()
}

// mapErr 把 SDK / mock 抛出的具体错误归一为 sign 包的 sentinel。
// 已是 sentinel 的（mock 直接返回 sign.ErrXxx）原样回传；类型断言识别
// AWS KMS 异常；smithy ResponseError 看 HTTP status 兜底；最后字符串
// 兜底处理测试 mock 常用的简单 error。
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

	// AWS KMS 业务异常（key 被删 / 禁用 / 不可用）。
	var nfe *kmstypes.NotFoundException
	if errors.As(err, &nfe) {
		return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
	}
	var de *kmstypes.DisabledException
	if errors.As(err, &de) {
		return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
	}
	var kue *kmstypes.KeyUnavailableException
	if errors.As(err, &kue) {
		return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
	}
	var ike *kmstypes.InvalidKeyUsageException
	if errors.As(err, &ike) {
		return fmt.Errorf("%w: %v", sign.ErrInvalidInput, err)
	}

	// HTTP 状态码兜底：401/403 → AuthFailed；404 → KeyNotFound；5xx / 0
	// （网络错）→ UpstreamUnavailable。
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		switch sc := respErr.HTTPStatusCode(); {
		case sc == 401 || sc == 403:
			return fmt.Errorf("%w: %v", sign.ErrAuthFailed, err)
		case sc == 404:
			return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
		case sc >= 500 || sc == 0:
			return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
		default:
			return fmt.Errorf("%w: HTTP %d: %v", sign.ErrUpstreamUnavailable, sc, err)
		}
	}

	// 字符串兜底：测试 mock 与 SDK 早期路径会以普通 error 形式抛出
	// "401" / "AccessDenied" 等。
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "accessdenied") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") || strings.Contains(msg, "403"):
		return fmt.Errorf("%w: %v", sign.ErrAuthFailed, err)
	case strings.Contains(msg, "notfound") || strings.Contains(msg, "not found") || strings.Contains(msg, "404"):
		return fmt.Errorf("%w: %v", sign.ErrKeyNotFound, err)
	case strings.Contains(msg, "throttl") ||
		strings.Contains(msg, "503") || strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") || strings.Contains(msg, "504") ||
		strings.Contains(msg, "5xx") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused"):
		return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
	}
	// 兜底归类为 upstream — 调用方据 sentinel 决定是否 retry；
	// 已知错误类型应在上面分支命中，这里走到说明是新类型，从保守
	// 角度按 retryable 处理（KMS audit log 会去重，重复 retry 不会
	// 造成 false positive）。
	return fmt.Errorf("%w: %v", sign.ErrUpstreamUnavailable, err)
}

// ---------------- realKMSClient: AWS SDK Go v2 适配 ----------------

// realKMSClient 把 aws-sdk-go-v2/service/kms 适配成 KMSClient interface。
//
// SDK 调用通过 sdkSign / sdkGetPub / sdkDescribe / sdkListRotations 函数
// 指针注入，便于单元测试覆盖响应解析逻辑而不打网络。
type realKMSClient struct {
	sdk              *kmssdk.Client
	sdkSign          func(ctx context.Context, in *kmssdk.SignInput, optFns ...func(*kmssdk.Options)) (*kmssdk.SignOutput, error)
	sdkGetPub        func(ctx context.Context, in *kmssdk.GetPublicKeyInput, optFns ...func(*kmssdk.Options)) (*kmssdk.GetPublicKeyOutput, error)
	sdkDescribe      func(ctx context.Context, in *kmssdk.DescribeKeyInput, optFns ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error)
	sdkListRotations func(ctx context.Context, in *kmssdk.ListKeyRotationsInput, optFns ...func(*kmssdk.Options)) (*kmssdk.ListKeyRotationsOutput, error)
}

func (r *realKMSClient) callSign(ctx context.Context, in *kmssdk.SignInput, optFns ...func(*kmssdk.Options)) (*kmssdk.SignOutput, error) {
	if r.sdkSign != nil {
		return r.sdkSign(ctx, in, optFns...)
	}
	return r.sdk.Sign(ctx, in, optFns...)
}

func (r *realKMSClient) callGetPub(ctx context.Context, in *kmssdk.GetPublicKeyInput, optFns ...func(*kmssdk.Options)) (*kmssdk.GetPublicKeyOutput, error) {
	if r.sdkGetPub != nil {
		return r.sdkGetPub(ctx, in, optFns...)
	}
	return r.sdk.GetPublicKey(ctx, in, optFns...)
}

func (r *realKMSClient) callDescribe(ctx context.Context, in *kmssdk.DescribeKeyInput, optFns ...func(*kmssdk.Options)) (*kmssdk.DescribeKeyOutput, error) {
	if r.sdkDescribe != nil {
		return r.sdkDescribe(ctx, in, optFns...)
	}
	return r.sdk.DescribeKey(ctx, in, optFns...)
}

func (r *realKMSClient) callListRotations(ctx context.Context, in *kmssdk.ListKeyRotationsInput, optFns ...func(*kmssdk.Options)) (*kmssdk.ListKeyRotationsOutput, error) {
	if r.sdkListRotations != nil {
		return r.sdkListRotations(ctx, in, optFns...)
	}
	return r.sdk.ListKeyRotations(ctx, in, optFns...)
}

// Sign 调用 AWS KMS Sign API。idempotencyKey 通过自定义 middleware 注入
// X-Amz-Sdk-Invocation-Id header — CloudTrail 据此去重，避免 worker
// crash 重试导致 audit log 出现重复条目（D4）。
func (r *realKMSClient) Sign(ctx context.Context, keyID, algorithm string, digest []byte, idempotencyKey string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	in := &kmssdk.SignInput{
		KeyId:            aws.String(keyID),
		Message:          digest,
		MessageType:      kmstypes.MessageTypeDigest,
		SigningAlgorithm: kmstypes.SigningAlgorithmSpec(algorithm),
	}
	optFn := func(o *kmssdk.Options) {
		o.APIOptions = append(o.APIOptions, withIdempotencyHeader(idempotencyKey))
	}
	out, err := r.callSign(ctx, in, optFn)
	if err != nil {
		return nil, err
	}
	return parseSignOutput(out)
}

func (r *realKMSClient) GetPublicKey(ctx context.Context, keyID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	in := &kmssdk.GetPublicKeyInput{KeyId: aws.String(keyID)}
	out, err := r.callGetPub(ctx, in)
	if err != nil {
		return nil, err
	}
	return parseGetPublicKeyOutput(out)
}

func (r *realKMSClient) DescribeKey(ctx context.Context, keyID string) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	// AWS KMS 没有显式整数 "key version" 概念；DescribeKey 返回元数据，
	// 真正的轮换信息在 ListKeyRotations。这里用 ListKeyRotations 数量
	// 作为单调递增版本号（每次轮换 +1，起点 0）。
	//
	// 仍保留 DescribeKey 调用：仅用于校验 key 存在 / 启用状态，触发
	// NotFoundException / DisabledException 的标准错误链。
	if _, err := r.callDescribe(ctx, &kmssdk.DescribeKeyInput{KeyId: aws.String(keyID)}); err != nil {
		return 0, err
	}
	out, err := r.callListRotations(ctx, &kmssdk.ListKeyRotationsInput{KeyId: aws.String(keyID)})
	if err != nil {
		return 0, err
	}
	if out == nil {
		return 0, errors.New("awskms: ListKeyRotations returned nil output")
	}
	return len(out.Rotations), nil
}

// parseSignOutput 解 SDK Sign 响应。提取为独立函数便于单元测试。
func parseSignOutput(out *kmssdk.SignOutput) ([]byte, error) {
	if out == nil {
		return nil, errors.New("awskms: Sign returned nil output")
	}
	if len(out.Signature) == 0 {
		return nil, errors.New("awskms: Sign returned empty Signature")
	}
	sig := make([]byte, len(out.Signature))
	copy(sig, out.Signature)
	return sig, nil
}

// parseGetPublicKeyOutput 解 SDK GetPublicKey 响应。AWS 返回的
// PublicKey 是 DER 编码的 PKIX 字节流；这里包成 PEM 让上层 stdlib
// 直接 pem.Decode + x509.ParsePKIXPublicKey 处理。
func parseGetPublicKeyOutput(out *kmssdk.GetPublicKeyOutput) ([]byte, error) {
	if out == nil {
		return nil, errors.New("awskms: GetPublicKey returned nil output")
	}
	if len(out.PublicKey) == 0 {
		return nil, errors.New("awskms: GetPublicKey returned empty PublicKey")
	}
	return derToPEM(out.PublicKey), nil
}

// derToPEM 把 PKIX DER 公钥包成 PEM "PUBLIC KEY" 块。等价于 stdlib
// pem.EncodeToMemory({Type:"PUBLIC KEY", Bytes:der})，手写避免在响应
// 解析热路径上引 encoding/pem 大依赖。
func derToPEM(der []byte) []byte {
	const header = "-----BEGIN PUBLIC KEY-----\n"
	const footer = "-----END PUBLIC KEY-----\n"
	const lineLen = 64
	b64 := []byte(base64.StdEncoding.EncodeToString(der))
	out := make([]byte, 0, len(header)+len(b64)+len(b64)/lineLen+len(footer))
	out = append(out, header...)
	for i := 0; i < len(b64); i += lineLen {
		end := i + lineLen
		if end > len(b64) {
			end = len(b64)
		}
		out = append(out, b64[i:end]...)
		out = append(out, '\n')
	}
	out = append(out, footer...)
	return out
}

// withIdempotencyHeader 返回一个 smithy middleware，在 Build 阶段把
// idempotencyKey 写到 X-Amz-Sdk-Invocation-Id header 上。AWS SDK Go v2
// 默认 retry middleware 会用同一 invocation ID 在重试时复用，CloudTrail
// 据此可以识别同一逻辑请求的多次物理 HTTP 调用。
//
// 这里手动设置该 header 是为了让 worker crash 后用相同 idempotencyKey
// 在新进程中也能命中 KMS 后端的去重窗口（不同 SDK invocation 但相同
// header 值）。
func withIdempotencyHeader(idempotencyKey string) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(
			middleware.BuildMiddlewareFunc("awskmsSetIdempotencyHeader",
				func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
					if req, ok := in.Request.(*smithyhttp.Request); ok {
						req.Header.Set("X-Amz-Sdk-Invocation-Id", idempotencyKey)
					}
					return next.HandleBuild(ctx, in)
				}),
			middleware.After,
		)
	}
}
