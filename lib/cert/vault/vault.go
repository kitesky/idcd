// Package vault 提供证书私钥与 DNS API 凭据的加密 / 解密能力。
//
// S1 实现：envmaster 用环境变量主密钥 + AES-256-GCM 信封加密（适合 MVP，不抗主机失窃）。
// S2 计划：替换为阿里云 KMS / AWS KMS，Vault 接口保持不变。
package vault

import (
	"context"
	"errors"
)

// KeyAlg 标识被加密的原始私钥算法。解密后调用方据此决定如何 parse / 编码 PEM。
type KeyAlg string

const (
	KeyAlgECDSAP256 KeyAlg = "ecdsa-p256"
	KeyAlgRSA2048   KeyAlg = "rsa-2048"
)

// EncryptedKey 加密后的私钥及其元数据。落库时序列化为 JSON / protobuf 均可（本接口不规定）。
type EncryptedKey struct {
	KeyID      string // Vault 主密钥 ID，用于审计 / 防误用错主密钥解密
	Algorithm  string // 始终 "AES-256-GCM"
	Nonce      []byte // 12 字节
	Ciphertext []byte // GCM 输出（含 auth tag）
	Alg        KeyAlg // 原始私钥算法（解密后用作生成 PEM header）
}

// EncryptedBlob 用于 DNS 凭据等任意 byte slice。结构同 EncryptedKey 但无算法字段。
type EncryptedBlob struct {
	KeyID      string
	Algorithm  string
	Nonce      []byte
	Ciphertext []byte
}

// Vault 是私钥与凭据加密的统一接口。S1 由 envmaster 实现，S2 切真 KMS 时保持接口不变。
type Vault interface {
	// KeyID 返回当前 Vault 持有的主密钥 ID（稳定，便于审计 / 轮换识别）。
	KeyID() string

	// GenerateKey 生成新私钥并立即加密；返回明文 PEM（**仅本次调用返回，调用方负责传走或销毁**）和落库用的 EncryptedKey。
	GenerateKey(ctx context.Context, alg KeyAlg) (plainPEM []byte, encrypted EncryptedKey, err error)

	// EncryptKey 加密既有 PEM 私钥（调用方场景：用户上传 CSR / 私钥迁移）。
	EncryptKey(ctx context.Context, plainPEM []byte) (EncryptedKey, error)

	// DecryptKey 解密返回 PEM。KeyID 不匹配应返回 ErrKeyIDMismatch。
	DecryptKey(ctx context.Context, ek EncryptedKey) (plainPEM []byte, err error)

	// EncryptBlob / DecryptBlob 用于 DNS API 凭据等任意 byte。
	EncryptBlob(ctx context.Context, plaintext []byte) (EncryptedBlob, error)
	DecryptBlob(ctx context.Context, eb EncryptedBlob) (plaintext []byte, err error)
}

var (
	ErrKeyIDMismatch     = errors.New("vault: key id mismatch")
	ErrInvalidCiphertext = errors.New("vault: invalid ciphertext or tampered")
	ErrMasterKeyMissing  = errors.New("vault: master key missing or invalid")
	ErrUnsupportedAlg    = errors.New("vault: unsupported key algorithm")
)
