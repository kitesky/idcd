// Package awskms 是 Vault 的 S2 实现：AWS KMS（D-FC-04 海外路径）。
//
// 设计与 alikms 包对称：信封加密（envelope encryption）模型 + 同一份
// Ciphertext 序列化布局 + 同一组 KMSClient 测试桩点。两者唯一的差异是
// SDK 适配层：
//
//  1. 向 KMS 调 GenerateDataKey(KeySpec=AES_256) 取一个 256-bit DEK，
//     返回 plaintext DEK + 由 CMK 加密过的 encrypted_DEK；
//  2. 本地用 plaintext DEK + AES-256-GCM 加密实际明文，立即销毁 plaintext DEK；
//  3. 落库存 (encrypted_DEK, nonce, ciphertext)，序列化布局与 alikms 一致：
//     [uint32 BE encDEK 长度][encDEK][AES-GCM 输出（含 tag）]
//  4. 解密时用 encrypted_DEK 调 KMS Decrypt(KeyId=CMK) 拿回 plaintext DEK，
//     本地解 AES-GCM。
//
// 跨 vault 实现的密文本来就不应互通；本包仅与本包自身的 Ciphertext 对解。
//
// 凭据来源：
//   - 同时设置 AccessKeyID + SecretAccessKey：走静态凭据；
//   - 两者均留空：走 AWS SDK 默认 credential chain
//     （IRSA / instance profile / shared config / env）；
//   - 只设置其中一个：返回 ErrMasterKeyMissing（半凭据是配置错误）。
//
// 测试通过 KMSClient interface 注入 mock，不依赖真实 AWS。
package awskms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	kmssdk "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/kite365/idcd/lib/cert/vault"
)

const (
	algorithm         = "AES-256-GCM"
	pemTypePrivateKey = "PRIVATE KEY"
	dekKeySpec        = "AES_256"
	dekLen            = 32 // AES-256 DEK
	nonceLen          = 12 // GCM standard
	dekLenPrefixBytes = 4  // uint32 BE
	// maxEncDEKLen 是 Ciphertext 头部 encrypted_DEK 长度上限。AWS KMS
	// 的 CiphertextBlob 通常 < 1 KiB（GenerateDataKey AES_256 实测 ~184 B）；
	// 64 KiB 余量足够，同时拒绝异常巨大的头部，防止恶意构造耗资源。
	maxEncDEKLen = 64 * 1024
)

// KMSClient 抽象出本包使用的 KMS 调用，便于测试注入 mock。生产实现见
// realKMSClient（包内私有，封装 AWS SDK Go v2）。接口签名与 alikms.KMSClient
// 一致，方便日后抽公共接口。
type KMSClient interface {
	// GenerateDataKey 向 KMS 申请 AES-256 DEK：返回 plaintext DEK
	// 和被 keyID 主密钥加密后的 encrypted_DEK。
	GenerateDataKey(ctx context.Context, keyID string, keySpec string) (plaintext []byte, encryptedDEK []byte, err error)

	// Decrypt 用 keyID 主密钥解密 encrypted_DEK，返回 plaintext DEK。
	Decrypt(ctx context.Context, encryptedDEK []byte) (plaintext []byte, err error)
}

// Config 是构造 AWS KMS Vault 所需的全部参数。
type Config struct {
	// Region 是 AWS region，例如 "us-east-1"。必填。
	Region string

	// AccessKeyID / SecretAccessKey 是静态 IAM 凭据。两者必须同时设置或同时
	// 留空。留空时走 SDK 默认 credential chain（IRSA / instance profile /
	// shared config / env）。
	AccessKeyID     string
	SecretAccessKey string

	// KeyID 是 KMS CMK 的 alias 或 ARN，例如 "alias/cert-master" 或
	// "arn:aws:kms:us-east-1:123:key/uuid"。必填。同时作为 vault.KeyID()
	// 返回值。
	KeyID string
}

// awskmsVault 是 vault.Vault 的 AWS KMS 实现。并发安全：内部不持有可变
// 状态，client 实现自行保证并发安全（AWS SDK v2 client 是的）。
type awskmsVault struct {
	client KMSClient
	keyID  string
}

// New 构造 AWS KMS Vault。Region / KeyID 缺失返回 ErrMasterKeyMissing；
// AccessKeyID / SecretAccessKey 半凭据（只设一个）也返回 ErrMasterKeyMissing。
//
// 网络/凭据问题在首次 GenerateKey / EncryptKey 调用时才会暴露；本函数仅
// 做参数校验和 SDK 客户端初始化（默认 credential chain 也是 lazy，不会
// 在 New 阶段触发 IMDS 请求）。
func New(cfg Config) (vault.Vault, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("%w: Region is empty", vault.ErrMasterKeyMissing)
	}
	if cfg.KeyID == "" {
		return nil, fmt.Errorf("%w: KeyID is empty", vault.ErrMasterKeyMissing)
	}
	// 半凭据（只设一个）是配置错误，明确拒绝。
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, fmt.Errorf("%w: AccessKeyID and SecretAccessKey must both be set or both empty (got AccessKeyID empty=%v, SecretAccessKey empty=%v)",
			vault.ErrMasterKeyMissing, cfg.AccessKeyID == "", cfg.SecretAccessKey == "")
	}

	opts := kmssdk.Options{Region: cfg.Region}
	if cfg.AccessKeyID != "" {
		// 静态凭据路径。
		opts.Credentials = credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)
	} else {
		// 默认 credential chain。LoadDefaultConfig 会读 env / shared config /
		// IMDS / ECS credential endpoint / IRSA 等。即使 IMDS 不可用，本调
		// 用本身不会触发凭据请求，只是登记 provider；真正取凭据发生在第一次
		// API 调用时。
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Region),
		)
		if err != nil {
			return nil, fmt.Errorf("awskms: load default AWS config: %w", err)
		}
		opts.Credentials = awsCfg.Credentials
	}

	sdkClient := kmssdk.New(opts)
	return NewWithClient(&realKMSClient{sdk: sdkClient, keyID: cfg.KeyID}, cfg.KeyID), nil
}

// NewWithClient 用自定义 KMSClient 构造 Vault，专供测试 / DI 场景。
// keyID 仍作为 vault.KeyID() 返回；client 自行决定如何使用它。
func NewWithClient(client KMSClient, keyID string) vault.Vault {
	return &awskmsVault{client: client, keyID: keyID}
}

func (v *awskmsVault) KeyID() string { return v.keyID }

func (v *awskmsVault) GenerateKey(ctx context.Context, alg vault.KeyAlg) ([]byte, vault.EncryptedKey, error) {
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
		return nil, vault.EncryptedKey{}, fmt.Errorf("awskms: generate %s: %w", alg, err)
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

func (v *awskmsVault) EncryptKey(ctx context.Context, plainPEM []byte) (vault.EncryptedKey, error) {
	return v.encrypt(ctx, plainPEM)
}

func (v *awskmsVault) DecryptKey(ctx context.Context, ek vault.EncryptedKey) ([]byte, error) {
	if ek.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, ek.KeyID, v.keyID)
	}
	return v.decrypt(ctx, ek.Nonce, ek.Ciphertext)
}

func (v *awskmsVault) EncryptBlob(ctx context.Context, plaintext []byte) (vault.EncryptedBlob, error) {
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

func (v *awskmsVault) DecryptBlob(ctx context.Context, eb vault.EncryptedBlob) ([]byte, error) {
	if eb.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, eb.KeyID, v.keyID)
	}
	return v.decrypt(ctx, eb.Nonce, eb.Ciphertext)
}

// encrypt 做信封加密：申请 DEK → 本地 AES-GCM → 拼装 Ciphertext。
//
// 失败的 KMS 调用按原始 err 透传（用 fmt.Errorf 包一层加上下文），
// 调用方据上下文判断是否 retry（throttle / 5xx / network）。
func (v *awskmsVault) encrypt(ctx context.Context, plaintext []byte) (vault.EncryptedKey, error) {
	dek, encDEK, err := v.client.GenerateDataKey(ctx, v.keyID, dekKeySpec)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("awskms: GenerateDataKey: %w", err)
	}
	if len(dek) != dekLen {
		return vault.EncryptedKey{}, fmt.Errorf("awskms: KMS returned DEK length %d, want %d", len(dek), dekLen)
	}
	if len(encDEK) == 0 {
		return vault.EncryptedKey{}, errors.New("awskms: KMS returned empty encrypted DEK")
	}
	if len(encDEK) > maxEncDEKLen {
		return vault.EncryptedKey{}, fmt.Errorf("awskms: KMS returned encrypted DEK length %d, exceeds %d", len(encDEK), maxEncDEKLen)
	}

	nonce, gcmCT, err := sealAES(dek, plaintext)
	// dek 已经被 sealAES 消费；为防 GC 前在堆上滞留，明确清零。
	zero(dek)
	if err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("awskms: AES-GCM seal: %w", err)
	}

	// Ciphertext 布局（与 alikms 一致）：
	//   [uint32 BE encDEK 长度][encDEK][AES-GCM 输出（含 tag）]
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
func (v *awskmsVault) decrypt(ctx context.Context, nonce, ciphertext []byte) ([]byte, error) {
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
		return nil, fmt.Errorf("awskms: KMS Decrypt: %w", err)
	}
	if len(dek) != dekLen {
		zero(dek)
		return nil, fmt.Errorf("awskms: KMS returned DEK length %d, want %d", len(dek), dekLen)
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
		// 与 alikms / envmaster 同步策略：crypto/rand 失败意味着内核 RNG
		// 坏了，进程继续运行不安全，但接口签名上返回 err 给调用方而非 panic。
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

// ---------------- realKMSClient: AWS SDK Go v2 适配 ----------------

// realKMSClient 把 aws-sdk-go-v2/service/kms 适配成 KMSClient interface。
//
// SDK 调用通过 sdkGen / sdkDec 函数指针注入，便于测试覆盖响应解析逻辑
// 而不打网络。
type realKMSClient struct {
	sdk   *kmssdk.Client
	keyID string
	// 函数指针抽象 SDK 调用边界（非 nil 时使用，nil 时走 sdk.GenerateDataKey/Decrypt）。
	sdkGen func(ctx context.Context, in *kmssdk.GenerateDataKeyInput, optFns ...func(*kmssdk.Options)) (*kmssdk.GenerateDataKeyOutput, error)
	sdkDec func(ctx context.Context, in *kmssdk.DecryptInput, optFns ...func(*kmssdk.Options)) (*kmssdk.DecryptOutput, error)
}

func (r *realKMSClient) callGen(ctx context.Context, in *kmssdk.GenerateDataKeyInput) (*kmssdk.GenerateDataKeyOutput, error) {
	if r.sdkGen != nil {
		return r.sdkGen(ctx, in)
	}
	return r.sdk.GenerateDataKey(ctx, in)
}

func (r *realKMSClient) callDec(ctx context.Context, in *kmssdk.DecryptInput) (*kmssdk.DecryptOutput, error) {
	if r.sdkDec != nil {
		return r.sdkDec(ctx, in)
	}
	return r.sdk.Decrypt(ctx, in)
}

func (r *realKMSClient) GenerateDataKey(ctx context.Context, keyID, keySpec string) ([]byte, []byte, error) {
	// AWS SDK Go v2 的 ctx 检查在底层中间件里做；这里仍显式 short-circuit
	// 已 canceled 的 ctx，避免空跑 SDK pipeline。
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	in := &kmssdk.GenerateDataKeyInput{
		KeyId:   aws.String(keyID),
		KeySpec: kmstypes.DataKeySpec(keySpec),
	}
	out, err := r.callGen(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	return parseGenerateDataKeyOutput(out)
}

func (r *realKMSClient) Decrypt(ctx context.Context, encryptedDEK []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	in := &kmssdk.DecryptInput{
		CiphertextBlob: encryptedDEK,
		// 显式带 KeyId：AWS 推荐做法，避免依赖 ciphertext blob 内嵌的 ARN
		// （后者在跨 region 多区 CMK 场景下可能漂移）。
		KeyId: aws.String(r.keyID),
	}
	out, err := r.callDec(ctx, in)
	if err != nil {
		return nil, err
	}
	return parseDecryptOutput(out)
}

// parseGenerateDataKeyOutput 拆 KMS GenerateDataKey 响应。AWS SDK Go v2 的
// Plaintext / CiphertextBlob 是 []byte（已 base64 解码），所以这里只做 nil
// 校验。提取为独立函数便于单元测试响应校验逻辑。
func parseGenerateDataKeyOutput(out *kmssdk.GenerateDataKeyOutput) ([]byte, []byte, error) {
	if out == nil {
		return nil, nil, errors.New("awskms: GenerateDataKey returned nil output")
	}
	if len(out.Plaintext) == 0 || len(out.CiphertextBlob) == 0 {
		return nil, nil, errors.New("awskms: GenerateDataKey missing Plaintext or CiphertextBlob")
	}
	// 拷贝出来，避免 SDK 内部缓冲复用导致的别名问题。
	dek := make([]byte, len(out.Plaintext))
	copy(dek, out.Plaintext)
	encDEK := make([]byte, len(out.CiphertextBlob))
	copy(encDEK, out.CiphertextBlob)
	return dek, encDEK, nil
}

// parseDecryptOutput 拆 KMS Decrypt 响应。
func parseDecryptOutput(out *kmssdk.DecryptOutput) ([]byte, error) {
	if out == nil {
		return nil, errors.New("awskms: Decrypt returned nil output")
	}
	if len(out.Plaintext) == 0 {
		return nil, errors.New("awskms: Decrypt returned empty Plaintext")
	}
	dek := make([]byte, len(out.Plaintext))
	copy(dek, out.Plaintext)
	return dek, nil
}
