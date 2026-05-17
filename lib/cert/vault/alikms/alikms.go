// Package alikms 是 Vault 的 S2 实现：阿里云 KMS（D-FC-04 国内主路径）。
//
// 信封加密（envelope encryption）模型：
//
//  1. 向 KMS 调 GenerateDataKey 取一个 256-bit DEK，返回 plaintext DEK
//     + 由主密钥 CMK 加密过的 encrypted_DEK；
//  2. 本地用 plaintext DEK + AES-256-GCM 加密实际明文，立即销毁 plaintext DEK；
//  3. 落库存 (encrypted_DEK, nonce, ciphertext)；
//  4. 解密时用 encrypted_DEK 调 KMS Decrypt 拿回 plaintext DEK，本地解 AES-GCM。
//
// 由于 vault.EncryptedKey / EncryptedBlob 没有专门的 encrypted_DEK 字段，
// 这里把 encrypted_DEK 序列化进 Ciphertext 头部（4 字节大端长度前缀 + encrypted_DEK
// + AES-GCM 输出）。该序列化仅 alikms 包内可解；不同 vault 实现之间的密文本来就
// 不应跨实现解密，对 envmaster 也无影响。
//
// 测试通过 KMSClient interface 注入 mock，不依赖真实阿里云。
package alikms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"

	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	kmssdk "github.com/alibabacloud-go/kms-20160120/v3/client"

	"github.com/kite365/idcd/lib/cert/vault"
)

const (
	algorithm         = "AES-256-GCM"
	pemTypePrivateKey = "PRIVATE KEY"
	dekKeySpec        = "AES_256"
	dekLen            = 32 // AES-256 DEK
	nonceLen          = 12 // GCM standard
	dekLenPrefixBytes = 4  // uint32 BE
	// maxEncDEKLen 是 Ciphertext 头部 encrypted_DEK 长度上限。阿里云 KMS
	// 的 CiphertextBlob 通常 < 1 KiB；这里给 64 KiB 余量足够，同时拒绝
	// 异常巨大的头部，防止恶意构造耗资源。
	maxEncDEKLen = 64 * 1024
)

// KMSClient 抽象出本包使用的 KMS 调用，便于测试注入 mock。生产实现见
// realKMSClient（包内私有，封装阿里云 V2 SDK）。
type KMSClient interface {
	// GenerateDataKey 向 KMS 申请 AES-256 DEK：返回 plaintext DEK
	// 和被 keyID 主密钥加密后的 encrypted_DEK。
	GenerateDataKey(ctx context.Context, keyID string, keySpec string) (plaintext []byte, encryptedDEK []byte, err error)

	// Decrypt 用 keyID 主密钥解密 encrypted_DEK，返回 plaintext DEK。
	Decrypt(ctx context.Context, encryptedDEK []byte) (plaintext []byte, err error)
}

// Config 是构造阿里云 KMS Vault 所需的全部参数。所有字段必填。
type Config struct {
	// RegionID 是阿里云 KMS region，例如 "cn-hangzhou"。
	RegionID string

	// AccessKeyID / AccessKeySecret 是 RAM 子账号凭据；
	// 必须有对应 KMS CMK 的 GenerateDataKey + Decrypt 权限。
	AccessKeyID     string
	AccessKeySecret string

	// KeyID 是 CMK 的 alias 或 ID，例如 "alias/cert-master" 或
	// "key-cn-hangzhou-xxxxx"。同时作为 vault.KeyID() 返回值。
	KeyID string
}

// alikmsVault 是 vault.Vault 的阿里云 KMS 实现。并发安全：
// 内部不持有可变状态，client 实现自行保证并发安全（V2 SDK 是的）。
type alikmsVault struct {
	client KMSClient
	keyID  string
}

// New 构造阿里云 KMS Vault。任一必填字段缺失返回 ErrMasterKeyMissing。
//
// 网络/凭据问题在首次 GenerateKey / EncryptKey 调用时才会暴露；
// 本函数仅做参数校验和 SDK 客户端初始化。
func New(cfg Config) (vault.Vault, error) {
	if cfg.RegionID == "" {
		return nil, fmt.Errorf("%w: RegionID is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("%w: AccessKeyID is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.AccessKeySecret == "" {
		return nil, fmt.Errorf("%w: AccessKeySecret is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.KeyID == "" {
		return nil, fmt.Errorf("%w: KeyID is empty", vault.ErrMasterKeyMissing)
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
		return nil, fmt.Errorf("alikms: init SDK client: %w", err)
	}
	return NewWithClient(&realKMSClient{sdk: sdkClient, keyID: cfg.KeyID}, cfg.KeyID), nil
}

// NewWithClient 用自定义 KMSClient 构造 Vault，专供测试 / DI 场景。
// keyID 仍作为 vault.KeyID() 返回；client 自行决定如何使用它。
func NewWithClient(client KMSClient, keyID string) vault.Vault {
	return &alikmsVault{client: client, keyID: keyID}
}

func (v *alikmsVault) KeyID() string { return v.keyID }

func (v *alikmsVault) GenerateKey(ctx context.Context, alg vault.KeyAlg) ([]byte, vault.EncryptedKey, error) {
	var (
		priv any
		err  error
	)
	switch alg {
	case vault.KeyAlgECDSAP256:
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case vault.KeyAlgRSA2048:
		priv, err = rsa.GenerateKey(rand.Reader, 2048)
	default:
		return nil, vault.EncryptedKey{}, fmt.Errorf("%w: %q", vault.ErrUnsupportedAlg, alg)
	}
	if err != nil {
		return nil, vault.EncryptedKey{}, fmt.Errorf("alikms: generate %s: %w", alg, err)
	}
	// MarshalPKCS8PrivateKey 对 stdlib 生成的 ECDSA/RSA key 不会失败。
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	plainPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Bytes: der})

	ek, err := v.encrypt(ctx, plainPEM)
	if err != nil {
		return nil, vault.EncryptedKey{}, err
	}
	ek.Alg = alg
	return plainPEM, ek, nil
}

func (v *alikmsVault) EncryptKey(ctx context.Context, plainPEM []byte) (vault.EncryptedKey, error) {
	return v.encrypt(ctx, plainPEM)
}

func (v *alikmsVault) DecryptKey(ctx context.Context, ek vault.EncryptedKey) ([]byte, error) {
	if ek.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, ek.KeyID, v.keyID)
	}
	return v.decrypt(ctx, ek.Nonce, ek.Ciphertext)
}

func (v *alikmsVault) EncryptBlob(ctx context.Context, plaintext []byte) (vault.EncryptedBlob, error) {
	ek, err := v.encrypt(ctx, plaintext)
	if err != nil {
		return vault.EncryptedBlob{}, err
	}
	return vault.EncryptedBlob{
		KeyID:      ek.KeyID,
		Algorithm:  ek.Algorithm,
		Nonce:      ek.Nonce,
		Ciphertext: ek.Ciphertext,
	}, nil
}

func (v *alikmsVault) DecryptBlob(ctx context.Context, eb vault.EncryptedBlob) ([]byte, error) {
	if eb.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, eb.KeyID, v.keyID)
	}
	return v.decrypt(ctx, eb.Nonce, eb.Ciphertext)
}

// encrypt 做信封加密：申请 DEK → 本地 AES-GCM → 拼装 Ciphertext。
//
// 失败的 KMS 调用按原始 err 透传（用 fmt.Errorf 包一层加上下文），
// 调用方据上下文判断是否 retry。
func (v *alikmsVault) encrypt(ctx context.Context, plaintext []byte) (vault.EncryptedKey, error) {
	dek, encDEK, err := v.client.GenerateDataKey(ctx, v.keyID, dekKeySpec)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("alikms: GenerateDataKey: %w", err)
	}
	if len(dek) != dekLen {
		return vault.EncryptedKey{}, fmt.Errorf("alikms: KMS returned DEK length %d, want %d", len(dek), dekLen)
	}
	if len(encDEK) == 0 {
		return vault.EncryptedKey{}, errors.New("alikms: KMS returned empty encrypted DEK")
	}
	if len(encDEK) > maxEncDEKLen {
		return vault.EncryptedKey{}, fmt.Errorf("alikms: KMS returned encrypted DEK length %d, exceeds %d", len(encDEK), maxEncDEKLen)
	}

	nonce, gcmCT, err := sealAES(dek, plaintext)
	// dek 已经被 sealAES 消费；为防 GC 前在堆上滞留，明确清零。
	zero(dek)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("alikms: AES-GCM seal: %w", err)
	}

	// Ciphertext 布局：[uint32 BE encDEK 长度][encDEK][AES-GCM 输出（含 tag）]
	out := make([]byte, 0, dekLenPrefixBytes+len(encDEK)+len(gcmCT))
	out = binary.BigEndian.AppendUint32(out, uint32(len(encDEK)))
	out = append(out, encDEK...)
	out = append(out, gcmCT...)

	return vault.EncryptedKey{
		KeyID:      v.keyID,
		Algorithm:  algorithm,
		Nonce:      nonce,
		Ciphertext: out,
	}, nil
}

// decrypt 反向走信封：拆 Ciphertext → KMS Decrypt encDEK → 本地 AES-GCM Open。
func (v *alikmsVault) decrypt(ctx context.Context, nonce, ciphertext []byte) ([]byte, error) {
	if len(nonce) != nonceLen {
		return nil, fmt.Errorf("%w: nonce length %d", vault.ErrInvalidCiphertext, len(nonce))
	}
	if len(ciphertext) < dekLenPrefixBytes {
		return nil, fmt.Errorf("%w: ciphertext too short to hold DEK length prefix", vault.ErrInvalidCiphertext)
	}
	encDEKLen := binary.BigEndian.Uint32(ciphertext[:dekLenPrefixBytes])
	if encDEKLen == 0 {
		return nil, fmt.Errorf("%w: empty encrypted DEK header", vault.ErrInvalidCiphertext)
	}
	if encDEKLen > maxEncDEKLen {
		return nil, fmt.Errorf("%w: encrypted DEK length %d exceeds %d", vault.ErrInvalidCiphertext, encDEKLen, maxEncDEKLen)
	}
	if uint64(len(ciphertext)) < uint64(dekLenPrefixBytes)+uint64(encDEKLen) {
		return nil, fmt.Errorf("%w: ciphertext too short to hold encrypted DEK", vault.ErrInvalidCiphertext)
	}
	encDEK := ciphertext[dekLenPrefixBytes : dekLenPrefixBytes+encDEKLen]
	gcmCT := ciphertext[dekLenPrefixBytes+encDEKLen:]

	dek, err := v.client.Decrypt(ctx, encDEK)
	if err != nil {
		return nil, fmt.Errorf("alikms: KMS Decrypt: %w", err)
	}
	if len(dek) != dekLen {
		zero(dek)
		return nil, fmt.Errorf("alikms: KMS returned DEK length %d, want %d", len(dek), dekLen)
	}
	pt, err := openAES(dek, nonce, gcmCT)
	zero(dek)
	if err != nil {
		// AES-GCM auth 失败 / 篡改 → ErrInvalidCiphertext
		return nil, fmt.Errorf("%w: %v", vault.ErrInvalidCiphertext, err)
	}
	return pt, nil
}

// sealAES：用 dek 做 AES-256-GCM 加密；nonce 随机生成。
// dek 长度由调用方保证为 32 字节。
func sealAES(dek, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		// 与 envmaster 同步策略：crypto/rand 失败意味着内核 RNG 坏了，
		// 进程继续运行不安全，但接口签名上返回 err 给调用方而非 panic。
		return nil, nil, fmt.Errorf("crypto/rand: %w", err)
	}
	return nonce, aead.Seal(nil, nonce, plaintext, nil), nil
}

func openAES(dek, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}

// zero 抹掉切片内容。Go 编译器无法保证这不会被优化掉，但实际栈上 / 堆上
// 大概率会真清；对 DEK 这种短生命周期 secret 是 best-effort 缓解。
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ---------------- realKMSClient: 阿里云 V2 SDK 适配 ----------------

// realKMSClient 把 alibabacloud-go/kms-20160120 V2 SDK 适配成 KMSClient interface。
//
// SDK 没显式接受 context.Context（仅在 *WithOptions 调用支持 Runtime），
// 这里通过 ctx.Err() 在调用前做一次 cancellation 检查。
//
// SDK 调用通过 sdkGen / sdkDec 函数指针注入，便于测试覆盖响应解析逻辑而不打网络。
type realKMSClient struct {
	sdk   *kmssdk.Client
	keyID string
	// 函数指针抽象 SDK 调用边界（非 nil 时使用，nil 时走 sdk.GenerateDataKey/Decrypt）。
	sdkGen func(*kmssdk.GenerateDataKeyRequest) (*kmssdk.GenerateDataKeyResponse, error)
	sdkDec func(*kmssdk.DecryptRequest) (*kmssdk.DecryptResponse, error)
}

func (r *realKMSClient) callGen(req *kmssdk.GenerateDataKeyRequest) (*kmssdk.GenerateDataKeyResponse, error) {
	if r.sdkGen != nil {
		return r.sdkGen(req)
	}
	return r.sdk.GenerateDataKey(req)
}

func (r *realKMSClient) callDec(req *kmssdk.DecryptRequest) (*kmssdk.DecryptResponse, error) {
	if r.sdkDec != nil {
		return r.sdkDec(req)
	}
	return r.sdk.Decrypt(req)
}

func (r *realKMSClient) GenerateDataKey(ctx context.Context, keyID, keySpec string) ([]byte, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	req := &kmssdk.GenerateDataKeyRequest{
		KeyId:   &keyID,
		KeySpec: &keySpec,
	}
	resp, err := r.callGen(req)
	if err != nil {
		return nil, nil, err
	}
	return parseGenerateDataKeyResponse(resp)
}

func (r *realKMSClient) Decrypt(ctx context.Context, encryptedDEK []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	blob := base64.StdEncoding.EncodeToString(encryptedDEK)
	req := &kmssdk.DecryptRequest{CiphertextBlob: &blob}
	resp, err := r.callDec(req)
	if err != nil {
		return nil, err
	}
	return parseDecryptResponse(resp)
}

// parseGenerateDataKeyResponse 拆 KMS GenerateDataKey 响应：base64 解码 Plaintext +
// CiphertextBlob。提取为独立函数便于单元测试响应校验逻辑。
func parseGenerateDataKeyResponse(resp *kmssdk.GenerateDataKeyResponse) ([]byte, []byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil, errors.New("alikms: GenerateDataKey returned empty body")
	}
	body := resp.Body
	if body.Plaintext == nil || body.CiphertextBlob == nil {
		return nil, nil, errors.New("alikms: GenerateDataKey missing Plaintext or CiphertextBlob")
	}
	// 阿里云 KMS 返回的 Plaintext / CiphertextBlob 都是 base64 编码字符串。
	dek, err := base64.StdEncoding.DecodeString(*body.Plaintext)
	if err != nil {
		return nil, nil, fmt.Errorf("alikms: decode Plaintext base64: %w", err)
	}
	encDEK, err := base64.StdEncoding.DecodeString(*body.CiphertextBlob)
	if err != nil {
		zero(dek)
		return nil, nil, fmt.Errorf("alikms: decode CiphertextBlob base64: %w", err)
	}
	return dek, encDEK, nil
}

// parseDecryptResponse 拆 KMS Decrypt 响应：base64 解码 Plaintext。
func parseDecryptResponse(resp *kmssdk.DecryptResponse) ([]byte, error) {
	if resp == nil || resp.Body == nil || resp.Body.Plaintext == nil {
		return nil, errors.New("alikms: Decrypt returned empty body or Plaintext")
	}
	dek, err := base64.StdEncoding.DecodeString(*resp.Body.Plaintext)
	if err != nil {
		return nil, fmt.Errorf("alikms: decode Plaintext base64: %w", err)
	}
	return dek, nil
}
