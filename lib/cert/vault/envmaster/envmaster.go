// Package envmaster 是 Vault 的 S1 实现：从环境变量读 32 字节 base64 主密钥，
// 用 AES-256-GCM 做信封加密。
//
// 仅 MVP / 测试使用；生产环境必须在 S2 切到真 KMS。
package envmaster

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/kite365/idcd/lib/cert/vault"
)

const (
	defaultEnvVarName = "CERT_MASTER_KEY"
	algorithm         = "AES-256-GCM"
	masterKeyLen      = 32 // AES-256
	nonceLen          = 12 // GCM standard
	pemTypePrivateKey = "PRIVATE KEY"
)

// envVault 是 vault.Vault 的 envmaster 实现。cipher.AEAD 并发安全，无需额外锁。
type envVault struct {
	keyID string
	aead  cipher.AEAD
}

// NewFromEnv 从环境变量读 base64 编码的 32 字节主密钥。
// envVarName 为空时默认 "CERT_MASTER_KEY"。
// 返回的 Vault 实现是并发安全的。
func NewFromEnv(envVarName string) (vault.Vault, error) {
	if envVarName == "" {
		envVarName = defaultEnvVarName
	}
	raw := os.Getenv(envVarName)
	if raw == "" {
		return nil, fmt.Errorf("%w: env var %q not set", vault.ErrMasterKeyMissing, envVarName)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: env var %q not valid base64", vault.ErrMasterKeyMissing, envVarName)
	}
	return NewWithKey(key)
}

// NewWithKey 显式传入 master key（测试友好）。masterKey 必须正好 32 字节。
func NewWithKey(masterKey []byte) (vault.Vault, error) {
	if len(masterKey) != masterKeyLen {
		return nil, fmt.Errorf("%w: master key must be %d bytes, got %d", vault.ErrMasterKeyMissing, masterKeyLen, len(masterKey))
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		// aes.NewCipher 仅在 key 长度非 16/24/32 时失败，前面已校验，理论不可达。
		return nil, fmt.Errorf("%w: %v", vault.ErrMasterKeyMissing, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", vault.ErrMasterKeyMissing, err)
	}
	sum := sha256.Sum256(masterKey)
	return &envVault{
		keyID: hex.EncodeToString(sum[:])[:16],
		aead:  aead,
	}, nil
}

func (v *envVault) KeyID() string { return v.keyID }

func (v *envVault) GenerateKey(_ context.Context, alg vault.KeyAlg) ([]byte, vault.EncryptedKey, error) {
	var der []byte
	switch alg {
	case vault.KeyAlgECDSAP256:
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, vault.EncryptedKey{}, fmt.Errorf("ecdsa generate: %w", err)
		}
		der, err = x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, vault.EncryptedKey{}, fmt.Errorf("marshal pkcs8: %w", err)
		}
	case vault.KeyAlgRSA2048:
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, vault.EncryptedKey{}, fmt.Errorf("rsa generate: %w", err)
		}
		der, err = x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, vault.EncryptedKey{}, fmt.Errorf("marshal pkcs8: %w", err)
		}
	default:
		return nil, vault.EncryptedKey{}, fmt.Errorf("%w: %q", vault.ErrUnsupportedAlg, alg)
	}

	plainPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Bytes: der})

	nonce, ct, err := v.seal(plainPEM)
	if err != nil {
		return nil, vault.EncryptedKey{}, err
	}
	return plainPEM, vault.EncryptedKey{
		KeyID:      v.keyID,
		Algorithm:  algorithm,
		Nonce:      nonce,
		Ciphertext: ct,
		Alg:        alg,
	}, nil
}

func (v *envVault) EncryptKey(_ context.Context, plainPEM []byte) (vault.EncryptedKey, error) {
	nonce, ct, err := v.seal(plainPEM)
	if err != nil {
		return vault.EncryptedKey{}, err
	}
	return vault.EncryptedKey{
		KeyID:      v.keyID,
		Algorithm:  algorithm,
		Nonce:      nonce,
		Ciphertext: ct,
	}, nil
}

func (v *envVault) DecryptKey(_ context.Context, ek vault.EncryptedKey) ([]byte, error) {
	if ek.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, ek.KeyID, v.keyID)
	}
	return v.open(ek.Nonce, ek.Ciphertext)
}

func (v *envVault) EncryptBlob(_ context.Context, plaintext []byte) (vault.EncryptedBlob, error) {
	nonce, ct, err := v.seal(plaintext)
	if err != nil {
		return vault.EncryptedBlob{}, err
	}
	return vault.EncryptedBlob{
		KeyID:      v.keyID,
		Algorithm:  algorithm,
		Nonce:      nonce,
		Ciphertext: ct,
	}, nil
}

func (v *envVault) DecryptBlob(_ context.Context, eb vault.EncryptedBlob) ([]byte, error) {
	if eb.KeyID != v.keyID {
		return nil, fmt.Errorf("%w: have %q want %q", vault.ErrKeyIDMismatch, eb.KeyID, v.keyID)
	}
	return v.open(eb.Nonce, eb.Ciphertext)
}

func (v *envVault) seal(plaintext []byte) (nonce, ciphertext []byte, err error) {
	nonce = make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("nonce read: %w", err)
	}
	ciphertext = v.aead.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func (v *envVault) open(nonce, ciphertext []byte) ([]byte, error) {
	if len(nonce) != nonceLen {
		return nil, fmt.Errorf("%w: nonce length %d", vault.ErrInvalidCiphertext, len(nonce))
	}
	pt, err := v.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", vault.ErrInvalidCiphertext, err)
	}
	return pt, nil
}
