// Package hashivault 是 Vault 的 S2 实现：HashiCorp Vault Transit Secrets Engine
// （D-FC-04 自托管选项）。
//
// 信封加密（envelope encryption）模型：
//
//  1. 向 Vault Transit `datakey/plaintext/{KeyName}` 申请 256-bit DEK，返回
//     plaintext DEK + 由主密钥包装的 encrypted_DEK（形如 "vault:v1:base64..."
//     的 ASCII 字符串）；
//  2. 本地用 plaintext DEK + AES-256-GCM 加密实际明文，立即销毁 plaintext DEK；
//  3. 落库存 (encrypted_DEK_ascii_bytes, nonce, ciphertext)；
//  4. 解密时把 encrypted_DEK_ascii_bytes 原样作为 ciphertext 字符串调
//     Transit `decrypt/{KeyName}` 拿回 plaintext DEK，本地解 AES-GCM。
//
// 与 alikms 共享 Ciphertext 序列化布局：[uint32 BE encDEK_len][encDEK][AES-GCM
// 输出（含 tag）]。注意 encDEK 是 Vault 返回的完整 "vault:vN:base64..." 字符串
// 的原始字节，不能 base64 解码后再存——Vault Decrypt 必须用原字串。
//
// 测试通过 transitClient interface 注入 mock，不依赖真实 Vault 服务。
package hashivault

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

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/kite365/idcd/lib/cert/vault"
)

const (
	algorithm         = "AES-256-GCM"
	pemTypePrivateKey = "PRIVATE KEY"
	dekLen            = 32 // AES-256 DEK
	nonceLen          = 12 // GCM standard
	dekLenPrefixBytes = 4  // uint32 BE
	// maxEncDEKLen 是 Ciphertext 头部 encrypted_DEK 长度上限。Vault Transit
	// "vault:vN:base64..." 字符串一般 < 200 字节；这里给 64 KiB 余量，
	// 同时拒绝异常巨大的头部，防止恶意构造耗资源。
	maxEncDEKLen = 64 * 1024
	// defaultMountPath 是 Transit Engine 在 Vault 中默认的挂载路径。
	defaultMountPath = "transit"
)

// transitClient 抽象出本包使用的 Vault Transit 调用，便于测试注入 mock。
// 生产实现见 realClient（包内私有，封装 hashicorp/vault/api SDK）。
type transitClient interface {
	// GenerateDataKey 向 Vault Transit 申请 AES-256 DEK：返回 plaintext DEK
	// 和被 keyName 主密钥包装后的 encryptedDEK（Vault 的 "vault:vN:base64..."
	// 完整字符串 ASCII bytes，不可解码）。
	GenerateDataKey(ctx context.Context, keyName string) (plaintext []byte, encryptedDEK []byte, err error)

	// Decrypt 用 keyName 主密钥解包 encryptedDEK，返回 plaintext DEK。
	// encryptedDEK 必须是 "vault:vN:base64..." 完整字符串的原始 ASCII bytes。
	Decrypt(ctx context.Context, keyName string, encryptedDEK []byte) (plaintext []byte, err error)
}

// Config 是构造 HashiCorp Vault Transit Vault 所需参数。
type Config struct {
	// Address is the Vault server URL, e.g. "https://vault.example:8200".
	Address string
	// Token authenticates Transit API calls. Use a long-lived service
	// token scoped to the transit/{KeyName} path.
	Token string
	// Namespace is the Vault Enterprise namespace, e.g. "cert-svc/".
	// Empty for OSS / single-namespace deploys.
	Namespace string
	// KeyName is the Transit key the platform uses for envelope
	// wrapping, e.g. "cert-master". The key must already exist in
	// Vault (we never create keys).
	KeyName string
	// MountPath defaults to "transit" when empty.
	MountPath string
}

// hashiVault 是 vault.Vault 的 HashiCorp Vault Transit 实现。并发安全：
// 内部不持有可变状态，client 实现自行保证并发安全（vault/api 是的）。
type hashiVault struct {
	client    transitClient
	keyName   string
	mountPath string
	keyID     string
}

// New 构造 HashiCorp Vault Transit Vault。任一必填字段缺失返回 ErrMasterKeyMissing。
//
// 网络/凭据问题在首次 GenerateKey / EncryptKey 调用时才会暴露；
// 本函数仅做参数校验和 SDK 客户端初始化。
func New(cfg Config) (vault.Vault, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("%w: Address is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("%w: Token is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.KeyName == "" {
		return nil, fmt.Errorf("%w: KeyName is empty", vault.ErrMasterKeyMissing)
	}

	mountPath := cfg.MountPath
	if mountPath == "" {
		mountPath = defaultMountPath
	}

	apiCfg := vaultapi.DefaultConfig()
	apiCfg.Address = cfg.Address
	apiClient, err := vaultapi.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("hashivault: init SDK client: %w", err)
	}
	apiClient.SetToken(cfg.Token)
	if cfg.Namespace != "" {
		apiClient.SetNamespace(cfg.Namespace)
	}

	rc := &realClient{api: apiClient, mountPath: mountPath}
	return newVault(rc, cfg.KeyName, mountPath), nil
}

// NewWithClient 用自定义 transitClient 构造 Vault，专供测试 / DI 场景。
// keyName 既作为 transitClient 调用的 key 名，也参与 vault.KeyID() 构造。
// mountPath 为空时使用默认 "transit"。
func NewWithClient(client transitClient, keyName, mountPath string) vault.Vault {
	if mountPath == "" {
		mountPath = defaultMountPath
	}
	return newVault(client, keyName, mountPath)
}

func newVault(client transitClient, keyName, mountPath string) *hashiVault {
	return &hashiVault{
		client:    client,
		keyName:   keyName,
		mountPath: mountPath,
		keyID:     fmt.Sprintf("hashivault://%s/%s", mountPath, keyName),
	}
}

func (v *hashiVault) KeyID() string { return v.keyID }

func (v *hashiVault) GenerateKey(ctx context.Context, alg vault.KeyAlg) ([]byte, vault.EncryptedKey, error) {
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
		return nil, vault.EncryptedKey{}, fmt.Errorf("hashivault: generate %s: %w", alg, err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	plainPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Bytes: der})

	ek, err := v.encrypt(ctx, plainPEM)
	if err != nil {
		return nil, vault.EncryptedKey{}, err
	}
	ek.Alg = alg
	return plainPEM, ek, nil
}

func (v *hashiVault) EncryptKey(ctx context.Context, plainPEM []byte) (vault.EncryptedKey, error) {
	return v.encrypt(ctx, plainPEM)
}

func (v *hashiVault) DecryptKey(ctx context.Context, ek vault.EncryptedKey) ([]byte, error) {
	if ek.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, ek.KeyID, v.keyID)
	}
	return v.decrypt(ctx, ek.Nonce, ek.Ciphertext)
}

func (v *hashiVault) EncryptBlob(ctx context.Context, plaintext []byte) (vault.EncryptedBlob, error) {
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

func (v *hashiVault) DecryptBlob(ctx context.Context, eb vault.EncryptedBlob) ([]byte, error) {
	if eb.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, eb.KeyID, v.keyID)
	}
	return v.decrypt(ctx, eb.Nonce, eb.Ciphertext)
}

// encrypt 做信封加密：申请 DEK → 本地 AES-GCM → 拼装 Ciphertext。
func (v *hashiVault) encrypt(ctx context.Context, plaintext []byte) (vault.EncryptedKey, error) {
	dek, encDEK, err := v.client.GenerateDataKey(ctx, v.keyName)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("hashivault: GenerateDataKey: %w", err)
	}
	if len(dek) != dekLen {
		return vault.EncryptedKey{}, fmt.Errorf("hashivault: Vault returned DEK length %d, want %d", len(dek), dekLen)
	}
	if len(encDEK) == 0 {
		return vault.EncryptedKey{}, errors.New("hashivault: Vault returned empty encrypted DEK")
	}
	if len(encDEK) > maxEncDEKLen {
		return vault.EncryptedKey{}, fmt.Errorf("hashivault: Vault returned encrypted DEK length %d, exceeds %d", len(encDEK), maxEncDEKLen)
	}

	nonce, gcmCT, err := sealAES(dek, plaintext)
	zero(dek)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("hashivault: AES-GCM seal: %w", err)
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

// decrypt 反向走信封：拆 Ciphertext → Vault Decrypt encDEK → 本地 AES-GCM Open。
func (v *hashiVault) decrypt(ctx context.Context, nonce, ciphertext []byte) ([]byte, error) {
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

	dek, err := v.client.Decrypt(ctx, v.keyName, encDEK)
	if err != nil {
		return nil, fmt.Errorf("hashivault: Vault Decrypt: %w", err)
	}
	if len(dek) != dekLen {
		zero(dek)
		return nil, fmt.Errorf("hashivault: Vault returned DEK length %d, want %d", len(dek), dekLen)
	}
	pt, err := openAES(dek, nonce, gcmCT)
	zero(dek)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", vault.ErrInvalidCiphertext, err)
	}
	return pt, nil
}

// sealAES：用 dek 做 AES-256-GCM 加密；nonce 随机生成。
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

// zero 抹掉切片内容（best-effort）。
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// ---------------- realClient: hashicorp/vault/api SDK 适配 ----------------

// realClient 把 hashicorp/vault/api SDK 适配成 transitClient interface。
//
// SDK 调用通过 sdkWrite 函数指针注入，便于测试覆盖响应解析逻辑而不打网络。
type realClient struct {
	api       *vaultapi.Client
	mountPath string
	// sdkWrite 非 nil 时使用，nil 时走 api.Logical().WriteWithContext。
	sdkWrite func(ctx context.Context, path string, data map[string]interface{}) (*vaultapi.Secret, error)
}

func (r *realClient) call(ctx context.Context, path string, data map[string]interface{}) (*vaultapi.Secret, error) {
	if r.sdkWrite != nil {
		return r.sdkWrite(ctx, path, data)
	}
	return r.api.Logical().WriteWithContext(ctx, path, data)
}

func (r *realClient) GenerateDataKey(ctx context.Context, keyName string) ([]byte, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	path := fmt.Sprintf("%s/datakey/plaintext/%s", r.mountPath, keyName)
	resp, err := r.call(ctx, path, map[string]interface{}{})
	if err != nil {
		return nil, nil, err
	}
	return parseGenerateDataKeyResponse(resp)
}

func (r *realClient) Decrypt(ctx context.Context, keyName string, encryptedDEK []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/decrypt/%s", r.mountPath, keyName)
	resp, err := r.call(ctx, path, map[string]interface{}{
		"ciphertext": string(encryptedDEK),
	})
	if err != nil {
		return nil, err
	}
	return parseDecryptResponse(resp)
}

// parseGenerateDataKeyResponse 拆 Vault Transit datakey 响应：base64 解码
// plaintext（DEK），ciphertext（"vault:vN:base64..."）原样返回 ASCII bytes。
func parseGenerateDataKeyResponse(resp *vaultapi.Secret) ([]byte, []byte, error) {
	if resp == nil || resp.Data == nil {
		return nil, nil, errors.New("hashivault: GenerateDataKey returned empty body")
	}
	plainAny, ok := resp.Data["plaintext"]
	if !ok {
		return nil, nil, errors.New("hashivault: GenerateDataKey missing plaintext")
	}
	ctAny, ok := resp.Data["ciphertext"]
	if !ok {
		return nil, nil, errors.New("hashivault: GenerateDataKey missing ciphertext")
	}
	plainStr, ok := plainAny.(string)
	if !ok {
		return nil, nil, fmt.Errorf("hashivault: plaintext not string: %T", plainAny)
	}
	ctStr, ok := ctAny.(string)
	if !ok {
		return nil, nil, fmt.Errorf("hashivault: ciphertext not string: %T", ctAny)
	}
	dek, err := base64.StdEncoding.DecodeString(plainStr)
	if err != nil {
		return nil, nil, fmt.Errorf("hashivault: decode plaintext base64: %w", err)
	}
	if ctStr == "" {
		zero(dek)
		return nil, nil, errors.New("hashivault: empty ciphertext string")
	}
	return dek, []byte(ctStr), nil
}

// parseDecryptResponse 拆 Vault Transit decrypt 响应：base64 解码 plaintext。
func parseDecryptResponse(resp *vaultapi.Secret) ([]byte, error) {
	if resp == nil || resp.Data == nil {
		return nil, errors.New("hashivault: Decrypt returned empty body")
	}
	plainAny, ok := resp.Data["plaintext"]
	if !ok {
		return nil, errors.New("hashivault: Decrypt missing plaintext")
	}
	plainStr, ok := plainAny.(string)
	if !ok {
		return nil, fmt.Errorf("hashivault: plaintext not string: %T", plainAny)
	}
	dek, err := base64.StdEncoding.DecodeString(plainStr)
	if err != nil {
		return nil, fmt.Errorf("hashivault: decode plaintext base64: %w", err)
	}
	return dek, nil
}
