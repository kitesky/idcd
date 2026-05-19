// Package localfile 是 lib/attest/sign 的本地 PEM keyfile 实现，专供 dev /
// pre-prod 环境使用 — 不依赖 AWS / 阿里云 KMS，把 RSA-2048 私钥放在磁盘
// 上的 PEM 文件里直接签。
//
// **不要用于生产**：私钥裸存磁盘，无 HSM / 无 IAM 审计 / 无 KeyVersion
// 轮换机制。本包仅满足 sign.Signer / sign.Verifier 接口，让 attest-generator
// 能在 dev 环境完成端到端 pipeline 验证（D4 WAL + PDF embed + TSA）。
//
// 算法选择：默认 RSA_PKCS1_V1_5_SHA_256，与 cmd/generator 的 loadDevSignerCert
// 自签 cert 默认签名算法（SHA256-RSA）匹配，pdfsign 嵌入 CMS 时 cert 公钥
// 才能和 KMSSign 闭包返回的 signature 验签通过。
//
// 启动行为：
//   - 若 KeyPath 文件存在：解析 PEM 私钥，复用既有 key（保证多次启动
//     生成的 cert 公钥稳定，因此 self-verify worker 跨重启仍能验通）。
//   - 不存在：generate RSA-2048，atomic write PEM (mode 0600)。
//   - 解析失败 / 算法不支持：返回 sign.ErrInvalidInput。
package localfile

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/sign/internal/idempcache"
)

// Config 是构造 localfile Signer / Verifier 所需的参数。
type Config struct {
	// KeyPath 是 PEM 私钥文件的绝对路径。文件不存在时自动生成并写入。
	// 必填。
	KeyPath string

	// Algorithm 是签名算法。默认 RSA_PKCS1_V1_5_SHA_256（与 loadDevSignerCert
	// 自签 cert 的默认签名算法兼容）。可选 RSA_PSS_SHA_256 / 384 / 512。
	Algorithm string
}

// PrivateKeyHolder 暴露当前持有的 *rsa.PrivateKey — 让 cmd/generator
// 的 loadDevSignerCert 可以拿到同一把 key 生成 cert，保证 cert 公钥
// 与 signer 公钥严格匹配。仅本包 + cmd/generator 用，对外不暴露。
type PrivateKeyHolder interface {
	PrivateKey() *rsa.PrivateKey
}

type signer struct {
	keyPath   string
	algorithm string
	key       *rsa.PrivateKey

	// idempCache 单进程内 (idempotencyKey → signature) 缓存。
	// 与 awskms / alikms 共用 idempcache.Cache：bounded FIFO + 防御性 byte copy；
	// 并发同 key 由 LoadOrStore 处理（先到者写入，后到者读回同值），
	// 即使 PSS salt 让两次 sign 产出不同 sig，外层拿到的也是同一份。
	idempCache *idempcache.Cache
}

type verifier struct {
	keyPath   string
	algorithm string
	pubPEM    []byte
}

// New 构造 localfile Signer。
func New(cfg Config) (sign.Signer, error) {
	key, alg, err := loadOrCreateKey(cfg)
	if err != nil {
		return nil, err
	}
	return &signer{
		keyPath:    cfg.KeyPath,
		algorithm:  alg,
		key:        key,
		idempCache: idempcache.New(0),
	}, nil
}

// NewVerifier 构造 localfile Verifier — 读取同一份 keyfile，返回其公钥。
func NewVerifier(cfg Config) (sign.Verifier, error) {
	key, alg, err := loadOrCreateKey(cfg)
	if err != nil {
		return nil, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("localfile: marshal pub key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return &verifier{
		keyPath:   cfg.KeyPath,
		algorithm: alg,
		pubPEM:    pubPEM,
	}, nil
}

func loadOrCreateKey(cfg Config) (*rsa.PrivateKey, string, error) {
	if cfg.KeyPath == "" {
		return nil, "", fmt.Errorf("%w: KeyPath is empty", sign.ErrInvalidInput)
	}
	alg := cfg.Algorithm
	if alg == "" {
		alg = sign.AlgorithmRSAPKCS1SHA256
	}
	switch alg {
	case sign.AlgorithmRSAPKCS1SHA256,
		sign.AlgorithmRSAPSSSHA256,
		sign.AlgorithmRSAPSSSHA384,
		sign.AlgorithmRSAPSSSHA512:
	default:
		return nil, "", fmt.Errorf("%w: unsupported Algorithm %q (localfile is RSA only)", sign.ErrInvalidInput, alg)
	}

	data, err := os.ReadFile(cfg.KeyPath)
	if err == nil {
		key, err := parsePEMKey(data)
		if err != nil {
			return nil, "", fmt.Errorf("localfile: parse %s: %w", cfg.KeyPath, err)
		}
		return key, alg, nil
	}
	if !os.IsNotExist(err) {
		return nil, "", fmt.Errorf("localfile: read %s: %w", cfg.KeyPath, err)
	}

	// Generate + persist.
	if err := os.MkdirAll(filepath.Dir(cfg.KeyPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("localfile: mkdir parent: %w", err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", fmt.Errorf("localfile: generate RSA: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, "", fmt.Errorf("localfile: marshal PKCS8: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	// atomic write: tmp + rename
	tmp := cfg.KeyPath + ".tmp"
	if err := os.WriteFile(tmp, pemBytes, 0o600); err != nil {
		return nil, "", fmt.Errorf("localfile: write tmp: %w", err)
	}
	if err := os.Rename(tmp, cfg.KeyPath); err != nil {
		_ = os.Remove(tmp)
		return nil, "", fmt.Errorf("localfile: rename: %w", err)
	}
	return key, alg, nil
}

func parsePEMKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS8 key is %T, want *rsa.PrivateKey", k)
		}
		return rk, nil
	default:
		return nil, fmt.Errorf("unsupported PEM type %q", block.Type)
	}
}

// --- signer interface ---

func (s *signer) KeyID() string     { return "localfile:" + s.keyPath }
func (s *signer) Algorithm() string { return s.algorithm }

// KeyVersion 用 keyfile 内容的 sha256 首 4 字节作为伪版本号 — 同一 key
// 重启稳定，更换 key 则跳变。dev only。
//
// 高位 bit 强制清零，确保结果落在 int32 正数范围（DB schema 把
// signature_key_version 定为 int4，超过 2^31-1 写入会 numeric overflow）。
func (s *signer) KeyVersion(ctx context.Context) (int, error) {
	data, err := os.ReadFile(s.keyPath)
	if err != nil {
		return 0, fmt.Errorf("localfile: KeyVersion read: %w", err)
	}
	sum := sha256.Sum256(data)
	raw := uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])
	return int(raw & 0x7FFFFFFF), nil
}

func (s *signer) Sign(ctx context.Context, digest []byte, idempotencyKey string) ([]byte, error) {
	if idempotencyKey == "" {
		return nil, fmt.Errorf("%w: idempotencyKey is empty", sign.ErrInvalidInput)
	}
	hashAlg, err := sign.HashAlgFor(s.algorithm)
	if err != nil {
		return nil, err
	}
	if len(digest) != hashAlg.Size() {
		return nil, fmt.Errorf("%w: digest len=%d want=%d", sign.ErrInvalidInput, len(digest), hashAlg.Size())
	}

	if cached, ok := s.idempCache.Load(idempotencyKey); ok {
		return cached, nil
	}

	var sig []byte
	switch s.algorithm {
	case sign.AlgorithmRSAPKCS1SHA256:
		sig, err = rsa.SignPKCS1v15(rand.Reader, s.key, hashAlg, digest)
	case sign.AlgorithmRSAPSSSHA256, sign.AlgorithmRSAPSSSHA384, sign.AlgorithmRSAPSSSHA512:
		sig, err = rsa.SignPSS(rand.Reader, s.key, hashAlg, digest, &rsa.PSSOptions{
			SaltLength: rsa.PSSSaltLengthEqualsHash,
			Hash:       hashAlg,
		})
	default:
		return nil, fmt.Errorf("%w: %s", sign.ErrInvalidInput, s.algorithm)
	}
	if err != nil {
		return nil, fmt.Errorf("localfile: sign: %w", err)
	}

	// LoadOrStore 处理并发同 key — 先到者写入，后到者读回同值；
	// PSS 即便两次 sign 产出不同 sig，外层拿到的是同一份。
	stored, _ := s.idempCache.LoadOrStore(idempotencyKey, sig)
	return stored, nil
}

// PrivateKey 暴露内部持有的 RSA key，让 cmd/generator 在用同一把 key
// 生成自签 cert。see PrivateKeyHolder.
func (s *signer) PrivateKey() *rsa.PrivateKey { return s.key }

var _ sign.Signer = (*signer)(nil)
var _ PrivateKeyHolder = (*signer)(nil)

// --- verifier interface ---

func (v *verifier) KeyID() string     { return "localfile:" + v.keyPath }
func (v *verifier) Algorithm() string { return v.algorithm }

func (v *verifier) PublicKey(ctx context.Context) ([]byte, error) {
	out := make([]byte, len(v.pubPEM))
	copy(out, v.pubPEM)
	return out, nil
}

var _ sign.Verifier = (*verifier)(nil)
