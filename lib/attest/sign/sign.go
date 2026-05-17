// Package sign 定义 S2 Evidence/Attestation 用的 KMS 签名 / 验签抽象，
// 并提供 AWS KMS 与阿里云 KMS 两个适配器。
//
// 设计目标（D4 / D6 锁定）：
//
//   - Verdict 报告每生成一份调用一次 KMS Sign。worker crash 重试时同一
//     idempotencyKey 必须返回相同 signature，避免 KMS audit log 产生重复
//     条目。本包通过两层保证：进程内 sync.Map idempotency 缓存（Sign 前
//     先查）+ 上层 attestation_record WAL 跳过已 success 的 step（外层
//     保证；与本包独立）。
//
//   - Self-Verify Worker 必须用 GetPublicKey 拿到公钥后本地用 crypto/ecdsa
//     或 crypto/rsa 验签，不调 KMS 的 Verify API — 这样保证签 / 验两条
//     路径独立，KMS 端同进程故障不会同时影响签与验（D6 独立性边界）。
//
//   - sentinel error 用 errors.Is 判定；上层调用方据此决定 retry / 报警 / 失败。
package sign

import (
	"context"
	"crypto"
	"errors"
)

// Signer 是异步签名接口。所有 KMS 适配器必须满足。
//
// 关键约束：实现必须把 idempotencyKey 透传给 KMS 后端的幂等机制（AWS KMS
// 用 X-Amz-Sdk-Invocation-Id；阿里云 KMS 用 RequestId）。Worker crash 重试
// 时同一 idempotencyKey 必返回相同 signature。
//
// 即使 KMS 端的幂等窗口失效，本包内的 sync.Map 缓存也会在进程生命周期内
// 兜底（前提：worker 没有重启）。两层叠加 + 上层 attestation_record WAL
// （worker 重启后通过 WAL 跳过已 success 的 step）共同保证 D4。
type Signer interface {
	// KeyID 返回稳定的 KMS key 标识（AWS ARN / 阿里云 alias 或 ID）。
	// 用于审计 log + verdict_report.signature_key_id 列。
	KeyID() string

	// KeyVersion 返回当前 sign key 的版本号（KMS 端 versioning 概念）。
	// 用于 verdict_report.signature_key_version 列；轮换检测。
	//
	// AWS KMS 没有显式的整数版本号，realKMSClient 用 ListKeyRotations
	// 数量近似（每轮换一次 +1）；阿里云 KMS 用 PrimaryKeyVersion 字符串
	// 的稳定哈希（FNV-32）近似。两者均仅用于 "轮换检测"，不是密码学
	// 保证。
	KeyVersion(ctx context.Context) (int, error)

	// Algorithm 返回签名算法（如 "ECDSA_SHA_256" / "RSA_PSS_SHA_256"）。
	// Verifier 必须用同一算法验签。
	Algorithm() string

	// Sign 用 KMS 对 digest 做签名。
	//
	// digest 必须是 hashAlg(payload) 的输出（KMS 端 MessageType=DIGEST，
	// hashAlg 必须匹配 Algorithm() 实现的算法；见 HashAlgFor）。digest
	// 长度由算法决定：SHA-256=32 / SHA-384=48 / SHA-512=64 字节。
	//
	// idempotencyKey 必须由调用方稳定生成（推荐
	// hex(sha256(report_id || ":" || action))）。空字符串视为
	// ErrInvalidInput（强制约定，避免 worker 漏传导致 audit log 重复）。
	Sign(ctx context.Context, digest []byte, idempotencyKey string) (signature []byte, err error)
}

// Verifier 是与 Signer 配对的验签接口；典型实现是从 KMS GetPublicKey
// 拿到 PEM 公钥，本地用 crypto/ecdsa 或 crypto/rsa 验签。
//
// 仅暴露 PublicKey() 一个 "数据" 方法 — 故意不暴露 Verify() 来强制
// 调用方走本地验签路径（D6 独立性）。
type Verifier interface {
	KeyID() string
	Algorithm() string

	// PublicKey 返回 PEM 编码的公钥（PKIX SubjectPublicKeyInfo 结构）。
	// Self-Verify Worker 把它 pem.Decode + x509.ParsePKIXPublicKey 后
	// 用 stdlib crypto 包验签。
	PublicKey(ctx context.Context) ([]byte, error)
}

// Sentinel errors。上层用 errors.Is 判定后决定行为：
//
//   - ErrUpstreamUnavailable: 网络 / 5xx / throttle → 退避 retry。
//   - ErrAuthFailed:          凭据失效 → P0 告警；不 retry。
//   - ErrInvalidInput:        digest 长度错 / 算法不支持 → 调用方 bug，不 retry。
//   - ErrKeyNotFound:         CMK 被删 / 禁用 → P0 告警；不 retry。
//   - ErrSignatureInvalid:    验签失败 → 调用方据上下文决定（攻击 vs bug）。
var (
	ErrUpstreamUnavailable = errors.New("sign: kms upstream unavailable")
	ErrAuthFailed          = errors.New("sign: kms authentication failed")
	ErrInvalidInput        = errors.New("sign: invalid digest length / algorithm")
	ErrKeyNotFound         = errors.New("sign: kms key not found / disabled")
	ErrSignatureInvalid    = errors.New("sign: signature failed verification")
)

// 已支持的签名算法常量。新增算法时同步 HashAlgFor 与各适配器的 SDK 映射。
const (
	AlgorithmECDSASHA256    = "ECDSA_SHA_256"
	AlgorithmECDSASHA384    = "ECDSA_SHA_384"
	AlgorithmECDSASHA512    = "ECDSA_SHA_512"
	AlgorithmRSAPSSSHA256   = "RSA_PSS_SHA_256"
	AlgorithmRSAPSSSHA384   = "RSA_PSS_SHA_384"
	AlgorithmRSAPSSSHA512   = "RSA_PSS_SHA_512"
	AlgorithmRSAPKCS1SHA256 = "RSASSA_PKCS1_V1_5_SHA_256"
)

// HashAlgFor 返回 algorithm 对应的 crypto.Hash，方便上层算 digest。
// 未知算法返回 ErrInvalidInput。
//
// 注意：返回的 crypto.Hash 是 stdlib 注册的算法 ID；调用方需要先 import
// "crypto/sha256" / "crypto/sha512" 等子包让 hash.Available() 为 true，
// 否则 hash.New() 会 panic。
func HashAlgFor(algorithm string) (crypto.Hash, error) {
	switch algorithm {
	case AlgorithmECDSASHA256, AlgorithmRSAPSSSHA256, AlgorithmRSAPKCS1SHA256:
		return crypto.SHA256, nil
	case AlgorithmECDSASHA384, AlgorithmRSAPSSSHA384:
		return crypto.SHA384, nil
	case AlgorithmECDSASHA512, AlgorithmRSAPSSSHA512:
		return crypto.SHA512, nil
	default:
		return 0, ErrInvalidInput
	}
}

// expectedDigestLen 返回 algorithm 对应 digest 字节数。未知算法返回 0。
// 内部工具：各适配器 Sign 前用它校验 digest 长度。
func expectedDigestLen(algorithm string) int {
	h, err := HashAlgFor(algorithm)
	if err != nil {
		return 0
	}
	return h.Size()
}
