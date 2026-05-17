package hashivault

import (
	"bytes"
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
	"strings"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"

	"github.com/kite365/idcd/lib/cert/vault"
)

// ---------- mockTransitClient ----------
//
// 用一个本地固定 32 字节"假主密钥" + AES-256-GCM 模拟 Vault Transit：
//   - GenerateDataKey：随机 32 字节 DEK，encryptedDEK = "vault:v1:" +
//     base64(nonce || aead.Seal(DEK))。注意保留 Vault 的字符串前缀以模拟真实 wire 格式。
//   - Decrypt：拆 "vault:v1:" 前缀，base64 解码后还原 DEK。
//
// 这样 mock 的语义和真 Vault 一致：明文 DEK 离开 mock 后只剩 ASCII 字符串密文，
// 调用 Decrypt 才能还原。

type mockTransitClient struct {
	master []byte // 32 字节
	// 注入点供测试覆盖错误路径
	failGenerateDataKey error
	failDecrypt         error
	genReturnDEK        []byte // 非 nil 时强制 GenerateDataKey 返回此 DEK
	genReturnEncDEK     []byte // 非 nil 时强制 GenerateDataKey 返回此 encDEK
	decReturnPlaintext  []byte // 非 nil 时强制 Decrypt 返回此 plaintext
	// 记录调用次数 / 入参
	genCalls       int
	decCalls       int
	lastDecKeyName string
	lastGenKeyName string
}

func newMockTransit(t *testing.T) *mockTransitClient {
	t.Helper()
	m := make([]byte, 32)
	if _, err := rand.Read(m); err != nil {
		t.Fatalf("seed mock master: %v", err)
	}
	return &mockTransitClient{master: m}
}

const vaultCTPrefix = "vault:v1:"

func (m *mockTransitClient) GenerateDataKey(_ context.Context, keyName string) ([]byte, []byte, error) {
	m.genCalls++
	m.lastGenKeyName = keyName
	if m.failGenerateDataKey != nil {
		return nil, nil, m.failGenerateDataKey
	}
	dek := m.genReturnDEK
	if dek == nil {
		dek = make([]byte, 32)
		if _, err := rand.Read(dek); err != nil {
			return nil, nil, err
		}
	} else {
		c := make([]byte, len(dek))
		copy(c, dek)
		dek = c
	}
	encDEK := m.genReturnEncDEK
	if encDEK == nil {
		block, _ := aes.NewCipher(m.master)
		aead, _ := cipher.NewGCM(block)
		nonce := make([]byte, 12)
		if _, err := rand.Read(nonce); err != nil {
			return nil, nil, err
		}
		ct := aead.Seal(nil, nonce, dek, nil)
		raw := append(nonce, ct...)
		encDEK = []byte(vaultCTPrefix + base64.StdEncoding.EncodeToString(raw))
	} else {
		c := make([]byte, len(encDEK))
		copy(c, encDEK)
		encDEK = c
	}
	return dek, encDEK, nil
}

func (m *mockTransitClient) Decrypt(_ context.Context, keyName string, encryptedDEK []byte) ([]byte, error) {
	m.decCalls++
	m.lastDecKeyName = keyName
	if m.failDecrypt != nil {
		return nil, m.failDecrypt
	}
	if m.decReturnPlaintext != nil {
		c := make([]byte, len(m.decReturnPlaintext))
		copy(c, m.decReturnPlaintext)
		return c, nil
	}
	s := string(encryptedDEK)
	if !strings.HasPrefix(s, vaultCTPrefix) {
		return nil, errors.New("mock: encDEK missing vault prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, vaultCTPrefix))
	if err != nil {
		return nil, fmt.Errorf("mock: bad base64: %w", err)
	}
	if len(raw) < 12 {
		return nil, errors.New("mock: encDEK too short")
	}
	nonce := raw[:12]
	ct := raw[12:]
	block, _ := aes.NewCipher(m.master)
	aead, _ := cipher.NewGCM(block)
	return aead.Open(nil, nonce, ct, nil)
}

// ---------- helpers ----------

func mustVault(t *testing.T) (*mockTransitClient, vault.Vault) {
	t.Helper()
	m := newMockTransit(t)
	v := NewWithClient(m, "cert-master", "transit")
	return m, v
}

// ---------- tests: construction ----------

func TestNew_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"empty all", Config{}},
		{"empty Address", Config{Token: "t", KeyName: "k"}},
		{"empty Token", Config{Address: "https://v:8200", KeyName: "k"}},
		{"empty KeyName", Config{Address: "https://v:8200", Token: "t"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(c.cfg)
			if !errors.Is(err, vault.ErrMasterKeyMissing) {
				t.Fatalf("want ErrMasterKeyMissing, got %v", err)
			}
		})
	}
}

func TestNew_OK_DefaultMount(t *testing.T) {
	v, err := New(Config{
		Address: "https://vault.example:8200",
		Token:   "s.abcdef",
		KeyName: "cert-master",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if v.KeyID() != "hashivault://transit/cert-master" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

func TestNew_OK_CustomMountAndNamespace(t *testing.T) {
	v, err := New(Config{
		Address:   "https://vault.example:8200",
		Token:     "s.abcdef",
		Namespace: "cert-svc/",
		KeyName:   "cert-master",
		MountPath: "kms-transit",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if v.KeyID() != "hashivault://kms-transit/cert-master" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

func TestNewWithClient_KeyID(t *testing.T) {
	_, v := mustVault(t)
	if v.KeyID() != "hashivault://transit/cert-master" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

func TestNewWithClient_DefaultMountWhenEmpty(t *testing.T) {
	m := newMockTransit(t)
	v := NewWithClient(m, "cert-master", "")
	if v.KeyID() != "hashivault://transit/cert-master" {
		t.Fatalf("KeyID() = %q, want default transit mount", v.KeyID())
	}
}

// ---------- tests: GenerateKey ----------

func TestGenerateKey_ECDSA(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	plainPEM, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if ek.Alg != vault.KeyAlgECDSAP256 {
		t.Errorf("ek.Alg = %q", ek.Alg)
	}
	if ek.Algorithm != "AES-256-GCM" {
		t.Errorf("ek.Algorithm = %q", ek.Algorithm)
	}
	if ek.KeyID != v.KeyID() {
		t.Errorf("ek.KeyID = %q want %q", ek.KeyID, v.KeyID())
	}
	if len(ek.Nonce) != 12 {
		t.Errorf("nonce len = %d", len(ek.Nonce))
	}
	if m.lastGenKeyName != "cert-master" {
		t.Errorf("lastGenKeyName = %q", m.lastGenKeyName)
	}

	dec, err := v.DecryptKey(ctx, ek)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(dec, plainPEM) {
		t.Fatal("decrypted PEM != original")
	}
	block, _ := pem.Decode(dec)
	if block == nil {
		t.Fatal("pem.Decode returned nil")
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey: %v", err)
	}
	if _, ok := pk.(*ecdsa.PrivateKey); !ok {
		t.Fatalf("not an ECDSA key: %T", pk)
	}
	if m.lastDecKeyName != "cert-master" {
		t.Errorf("lastDecKeyName = %q", m.lastDecKeyName)
	}
}

func TestGenerateKey_RSA(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	plainPEM, ek, err := v.GenerateKey(ctx, vault.KeyAlgRSA2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if ek.Alg != vault.KeyAlgRSA2048 {
		t.Errorf("ek.Alg = %q", ek.Alg)
	}
	dec, err := v.DecryptKey(ctx, ek)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(dec, plainPEM) {
		t.Fatal("decrypted PEM != original")
	}
	block, _ := pem.Decode(dec)
	if block == nil {
		t.Fatal("pem.Decode returned nil")
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey: %v", err)
	}
	if _, ok := pk.(*rsa.PrivateKey); !ok {
		t.Fatalf("not an RSA key: %T", pk)
	}
}

func TestGenerateKey_UnsupportedAlg(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, _, err := v.GenerateKey(ctx, vault.KeyAlg("ed25519-magic"))
	if !errors.Is(err, vault.ErrUnsupportedAlg) {
		t.Fatalf("want ErrUnsupportedAlg, got %v", err)
	}
}

func TestGenerateKey_VaultFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.failGenerateDataKey = errors.New("vault 403 permission denied")
	_, _, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("vault 403")) {
		t.Fatalf("err missing context: %v", err)
	}
}

// ---------- tests: EncryptKey / DecryptKey roundtrip + tampering ----------

func TestEncryptDecryptKey_Roundtrip(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)

	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	plain := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	ek, err := v.EncryptKey(ctx, plain)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if ek.KeyID != v.KeyID() {
		t.Errorf("KeyID mismatch")
	}
	if ek.Algorithm != "AES-256-GCM" {
		t.Errorf("Algorithm = %q", ek.Algorithm)
	}
	dec, err := v.DecryptKey(ctx, ek)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestDecryptKey_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	ek.KeyID = "hashivault://transit/different-master"
	_, err = v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrKeyIDMismatch) {
		t.Fatalf("want ErrKeyIDMismatch, got %v", err)
	}
}

func TestDecryptKey_TamperedNonce(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	ek.Nonce[0] ^= 0x01
	_, err = v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_ShortNonce(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	ek.Nonce = ek.Nonce[:8]
	_, err = v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_TamperedGCMCiphertext(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	ek.Ciphertext[len(ek.Ciphertext)-1] ^= 0x01
	_, err = v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_TamperedEncryptedDEK(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	// 翻转 Ciphertext[5]，落在 encDEK 区域（前 4 字节是长度前缀）。
	// encDEK 是 "vault:v1:..."，翻转一字节后 mock Decrypt 要么前缀失配、
	// 要么 base64 解码失败、要么 GCM 验证失败，总之报错。
	ek.Ciphertext[5] ^= 0x01
	_, err = v.DecryptKey(ctx, ek)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	// Vault-side 失败，不应归类为 ErrInvalidCiphertext（那是本地 AES-GCM）
	if errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want non-ErrInvalidCiphertext, got %v", err)
	}
}

// ---------- tests: Ciphertext header (encrypted_DEK length prefix) ----------

func TestDecryptKey_HeaderTooShort(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	ek := vault.EncryptedKey{
		KeyID:      v.KeyID(),
		Algorithm:  "AES-256-GCM",
		Nonce:      bytes.Repeat([]byte{0}, 12),
		Ciphertext: []byte{0x00, 0x01}, // < 4 bytes
	}
	_, err := v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_HeaderZeroLen(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	ct := make([]byte, 4)
	binary.BigEndian.PutUint32(ct, 0)
	ek := vault.EncryptedKey{
		KeyID:      v.KeyID(),
		Algorithm:  "AES-256-GCM",
		Nonce:      bytes.Repeat([]byte{0}, 12),
		Ciphertext: ct,
	}
	_, err := v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_HeaderOverflow(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	ct := make([]byte, 4)
	binary.BigEndian.PutUint32(ct, maxEncDEKLen+1)
	ek := vault.EncryptedKey{
		KeyID:      v.KeyID(),
		Algorithm:  "AES-256-GCM",
		Nonce:      bytes.Repeat([]byte{0}, 12),
		Ciphertext: ct,
	}
	_, err := v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_HeaderLenExceedsBuf(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	ct := make([]byte, 4)
	binary.BigEndian.PutUint32(ct, 100)
	ek := vault.EncryptedKey{
		KeyID:      v.KeyID(),
		Algorithm:  "AES-256-GCM",
		Nonce:      bytes.Repeat([]byte{0}, 12),
		Ciphertext: ct,
	}
	_, err := v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestEncryptKey_HeaderParsable(t *testing.T) {
	// 验证序列化布局：[uint32 BE len][encDEK][gcmCT]
	ctx := context.Background()
	m, v := mustVault(t)
	ek, err := v.EncryptKey(ctx, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ek.Ciphertext) < 4 {
		t.Fatal("ciphertext too short for header")
	}
	encLen := binary.BigEndian.Uint32(ek.Ciphertext[:4])
	if encLen == 0 {
		t.Fatal("encLen = 0")
	}
	if int(encLen) > len(ek.Ciphertext)-4 {
		t.Fatal("encLen exceeds remaining buf")
	}
	// 解开 encDEK，确认 mock Vault 能 Decrypt
	encDEK := ek.Ciphertext[4 : 4+encLen]
	if !bytes.HasPrefix(encDEK, []byte(vaultCTPrefix)) {
		t.Fatalf("encDEK missing vault prefix: %q", encDEK)
	}
	dek, err := m.Decrypt(ctx, "cert-master", encDEK)
	if err != nil {
		t.Fatalf("mock Decrypt encDEK: %v", err)
	}
	if len(dek) != 32 {
		t.Fatalf("DEK len = %d", len(dek))
	}
}

// ---------- tests: bad Vault DEK length / payload (defense in depth) ----------

func TestEncrypt_VaultReturnsBadDEKLen(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 16)
	m.genReturnEncDEK = []byte("vault:v1:irrelevant")
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("DEK length")) {
		t.Fatalf("want DEK length error, got %v", err)
	}
}

func TestEncrypt_VaultReturnsEmptyEncDEK(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 32)
	m.genReturnEncDEK = []byte{}
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty encrypted DEK")) {
		t.Fatalf("want empty encDEK error, got %v", err)
	}
}

func TestEncrypt_VaultReturnsHugeEncDEK(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 32)
	m.genReturnEncDEK = bytes.Repeat([]byte{0xBB}, maxEncDEKLen+1)
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("exceeds")) {
		t.Fatalf("want overflow error, got %v", err)
	}
}

func TestDecrypt_VaultReturnsBadDEKLen(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	ek, err := v.EncryptKey(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.decReturnPlaintext = bytes.Repeat([]byte{0xCC}, 16)
	_, err = v.DecryptKey(ctx, ek)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("DEK length")) {
		t.Fatalf("want DEK length error, got %v", err)
	}
}

func TestDecrypt_VaultFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	ek, err := v.EncryptKey(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.failDecrypt = errors.New("vault 500 internal")
	_, err = v.DecryptKey(ctx, ek)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("vault 500")) {
		t.Fatalf("want passthrough vault err, got %v", err)
	}
}

// ---------- tests: EncryptBlob / DecryptBlob ----------

func TestEncryptBlob_Roundtrip_Empty(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte{})
	if err != nil {
		t.Fatalf("EncryptBlob: %v", err)
	}
	if eb.KeyID != v.KeyID() {
		t.Errorf("KeyID mismatch")
	}
	if eb.Algorithm != "AES-256-GCM" {
		t.Errorf("Algorithm = %q", eb.Algorithm)
	}
	dec, err := v.DecryptBlob(ctx, eb)
	if err != nil {
		t.Fatalf("DecryptBlob: %v", err)
	}
	if len(dec) != 0 {
		t.Fatalf("dec = %x, want empty", dec)
	}
}

func TestEncryptBlob_Roundtrip_Large(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	plain := bytes.Repeat([]byte("idcd-cert-blob-payload-"), 500)
	eb, err := v.EncryptBlob(ctx, plain)
	if err != nil {
		t.Fatalf("EncryptBlob: %v", err)
	}
	dec, err := v.DecryptBlob(ctx, eb)
	if err != nil {
		t.Fatalf("DecryptBlob: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatal("large blob roundtrip mismatch")
	}
}

func TestEncryptBlob_NonceUnique(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	eb1, err := v.EncryptBlob(ctx, []byte("same plaintext"))
	if err != nil {
		t.Fatal(err)
	}
	eb2, err := v.EncryptBlob(ctx, []byte("same plaintext"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(eb1.Nonce, eb2.Nonce) {
		t.Fatal("nonces collided")
	}
	if bytes.Equal(eb1.Ciphertext, eb2.Ciphertext) {
		t.Fatal("ciphertexts identical for same plaintext")
	}
}

func TestDecryptBlob_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	eb.KeyID = "hashivault://transit/other"
	_, err = v.DecryptBlob(ctx, eb)
	if !errors.Is(err, vault.ErrKeyIDMismatch) {
		t.Fatalf("want ErrKeyIDMismatch, got %v", err)
	}
}

func TestDecryptBlob_Tampered(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	eb.Ciphertext[len(eb.Ciphertext)-1] ^= 0x80
	_, err = v.DecryptBlob(ctx, eb)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptBlob_VaultFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.failDecrypt = errors.New("vault token expired")
	_, err = v.DecryptBlob(ctx, eb)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("token expired")) {
		t.Fatalf("want vault err passthrough, got %v", err)
	}
}

// ---------- tests: error messages have context ----------

func TestErrorMessages_HaveContext(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("Address")) {
		t.Fatalf("err lacks field name: %v", err)
	}
	cases := []Config{
		{Address: "https://v:8200"},
		{Address: "https://v:8200", Token: "t"},
	}
	wantFields := []string{"Token", "KeyName"}
	for i, c := range cases {
		_, err := New(c)
		if err == nil {
			t.Fatalf("case %d: want err", i)
		}
		if !bytes.Contains([]byte(err.Error()), []byte(wantFields[i])) {
			t.Fatalf("case %d: err lacks %q: %v", i, wantFields[i], err)
		}
	}
}

// ---------- tests: zero() helper ----------

func TestZero(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5}
	zero(b)
	for i, v := range b {
		if v != 0 {
			t.Fatalf("b[%d] = %d, want 0", i, v)
		}
	}
	zero(nil)
	zero([]byte{})
}

// ---------- tests: realClient context cancellation ----------
//
// realClient.GenerateDataKey / Decrypt 在调用 SDK 前会先检查 ctx.Err()。

func TestRealClient_GenerateDataKey_CtxCanceled(t *testing.T) {
	r := &realClient{api: nil, mountPath: "transit"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := r.GenerateDataKey(ctx, "cert-master")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRealClient_Decrypt_CtxCanceled(t *testing.T) {
	r := &realClient{api: nil, mountPath: "transit"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Decrypt(ctx, "cert-master", []byte("vault:v1:abc"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// ---------- tests: realClient via injected SDK function pointers ----------

func TestRealClient_GenerateDataKey_SDKError(t *testing.T) {
	r := &realClient{
		mountPath: "transit",
		sdkWrite: func(context.Context, string, map[string]interface{}) (*vaultapi.Secret, error) {
			return nil, errors.New("connection refused")
		},
	}
	_, _, err := r.GenerateDataKey(context.Background(), "cert-master")
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("connection refused")) {
		t.Fatalf("want sdk error passthrough, got %v", err)
	}
}

func TestRealClient_GenerateDataKey_OK(t *testing.T) {
	dek := bytes.Repeat([]byte{0x42}, 32)
	encStr := "vault:v1:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x9F}, 64))
	var gotPath string
	r := &realClient{
		mountPath: "transit",
		sdkWrite: func(_ context.Context, path string, data map[string]interface{}) (*vaultapi.Secret, error) {
			gotPath = path
			if data == nil {
				t.Error("data nil")
			}
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"plaintext":  base64.StdEncoding.EncodeToString(dek),
					"ciphertext": encStr,
				},
			}, nil
		},
	}
	gotDEK, gotEnc, err := r.GenerateDataKey(context.Background(), "cert-master")
	if err != nil {
		t.Fatalf("GenerateDataKey: %v", err)
	}
	if gotPath != "transit/datakey/plaintext/cert-master" {
		t.Errorf("path = %q", gotPath)
	}
	if !bytes.Equal(gotDEK, dek) {
		t.Errorf("DEK mismatch")
	}
	if !bytes.Equal(gotEnc, []byte(encStr)) {
		t.Errorf("encDEK mismatch: want %q got %q", encStr, gotEnc)
	}
}

func TestRealClient_GenerateDataKey_CustomMount(t *testing.T) {
	dek := bytes.Repeat([]byte{0x42}, 32)
	encStr := "vault:v1:" + base64.StdEncoding.EncodeToString([]byte("blob"))
	var gotPath string
	r := &realClient{
		mountPath: "kms-transit",
		sdkWrite: func(_ context.Context, path string, _ map[string]interface{}) (*vaultapi.Secret, error) {
			gotPath = path
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"plaintext":  base64.StdEncoding.EncodeToString(dek),
					"ciphertext": encStr,
				},
			}, nil
		},
	}
	_, _, err := r.GenerateDataKey(context.Background(), "k")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "kms-transit/datakey/plaintext/k" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRealClient_Decrypt_SDKError(t *testing.T) {
	r := &realClient{
		mountPath: "transit",
		sdkWrite: func(context.Context, string, map[string]interface{}) (*vaultapi.Secret, error) {
			return nil, errors.New("vault 5xx")
		},
	}
	_, err := r.Decrypt(context.Background(), "cert-master", []byte("vault:v1:abc"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("vault 5xx")) {
		t.Fatalf("want sdk error passthrough, got %v", err)
	}
}

func TestRealClient_Decrypt_OK(t *testing.T) {
	dek := bytes.Repeat([]byte{0x77}, 32)
	encDEK := []byte("vault:v1:payload-encdek")
	var gotPath string
	var gotCT string
	r := &realClient{
		mountPath: "transit",
		sdkWrite: func(_ context.Context, path string, data map[string]interface{}) (*vaultapi.Secret, error) {
			gotPath = path
			s, _ := data["ciphertext"].(string)
			gotCT = s
			return &vaultapi.Secret{
				Data: map[string]interface{}{
					"plaintext": base64.StdEncoding.EncodeToString(dek),
				},
			}, nil
		},
	}
	got, err := r.Decrypt(context.Background(), "cert-master", encDEK)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if gotPath != "transit/decrypt/cert-master" {
		t.Errorf("path = %q", gotPath)
	}
	if gotCT != string(encDEK) {
		t.Errorf("ciphertext sent = %q want %q", gotCT, encDEK)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("DEK mismatch")
	}
}

// ---------- tests: parseGenerateDataKeyResponse / parseDecryptResponse ----------

func TestParseGenerateDataKeyResponse_NilResp(t *testing.T) {
	_, _, err := parseGenerateDataKeyResponse(nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty body")) {
		t.Fatalf("want empty body err, got %v", err)
	}
}

func TestParseGenerateDataKeyResponse_NilData(t *testing.T) {
	_, _, err := parseGenerateDataKeyResponse(&vaultapi.Secret{Data: nil})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty body")) {
		t.Fatalf("want empty body err, got %v", err)
	}
}

func TestParseGenerateDataKeyResponse_MissingFields(t *testing.T) {
	// missing plaintext
	_, _, err := parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{"ciphertext": "vault:v1:abc"},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("missing plaintext")) {
		t.Fatalf("want missing plaintext err, got %v", err)
	}
	// missing ciphertext
	_, _, err = parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{"plaintext": "abc"},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("missing ciphertext")) {
		t.Fatalf("want missing ciphertext err, got %v", err)
	}
}

func TestParseGenerateDataKeyResponse_WrongTypes(t *testing.T) {
	_, _, err := parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{
			"plaintext":  123, // not string
			"ciphertext": "vault:v1:abc",
		},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("plaintext not string")) {
		t.Fatalf("want plaintext type err, got %v", err)
	}
	_, _, err = parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{
			"plaintext":  base64.StdEncoding.EncodeToString([]byte("dek")),
			"ciphertext": 42, // not string
		},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("ciphertext not string")) {
		t.Fatalf("want ciphertext type err, got %v", err)
	}
}

func TestParseGenerateDataKeyResponse_BadBase64(t *testing.T) {
	_, _, err := parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{
			"plaintext":  "@@@not-base64@@@",
			"ciphertext": "vault:v1:abc",
		},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("decode plaintext")) {
		t.Fatalf("want decode plaintext err, got %v", err)
	}
}

func TestParseGenerateDataKeyResponse_EmptyCiphertext(t *testing.T) {
	_, _, err := parseGenerateDataKeyResponse(&vaultapi.Secret{
		Data: map[string]interface{}{
			"plaintext":  base64.StdEncoding.EncodeToString([]byte("dek")),
			"ciphertext": "",
		},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty ciphertext")) {
		t.Fatalf("want empty ciphertext err, got %v", err)
	}
}

func TestParseDecryptResponse_NilResp(t *testing.T) {
	_, err := parseDecryptResponse(nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty body")) {
		t.Fatalf("want empty body err, got %v", err)
	}
	_, err = parseDecryptResponse(&vaultapi.Secret{Data: nil})
	if err == nil {
		t.Fatal("want err for nil data")
	}
	_, err = parseDecryptResponse(&vaultapi.Secret{Data: map[string]interface{}{}})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("missing plaintext")) {
		t.Fatalf("want missing plaintext err, got %v", err)
	}
}

func TestParseDecryptResponse_WrongType(t *testing.T) {
	_, err := parseDecryptResponse(&vaultapi.Secret{
		Data: map[string]interface{}{"plaintext": 42},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("plaintext not string")) {
		t.Fatalf("want plaintext type err, got %v", err)
	}
}

func TestParseDecryptResponse_BadBase64(t *testing.T) {
	_, err := parseDecryptResponse(&vaultapi.Secret{
		Data: map[string]interface{}{"plaintext": "@@@bad@@@"},
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("decode plaintext")) {
		t.Fatalf("want decode err, got %v", err)
	}
}

// ---------- tests: Vault call counts (sanity) ----------

func TestVaultCallCounts(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)

	for i := 0; i < 3; i++ {
		ek, err := v.EncryptKey(ctx, []byte(fmt.Sprintf("data-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.DecryptKey(ctx, ek); err != nil {
			t.Fatal(err)
		}
	}
	if m.genCalls != 3 {
		t.Errorf("genCalls = %d, want 3", m.genCalls)
	}
	if m.decCalls != 3 {
		t.Errorf("decCalls = %d, want 3", m.decCalls)
	}
}
