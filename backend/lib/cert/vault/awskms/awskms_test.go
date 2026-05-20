package awskms

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
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"testing"

	kmssdk "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/kite365/idcd/lib/cert/vault"
)

// ---------- mockKMSClient ----------
//
// 用一个本地固定 32 字节 "假主密钥" + AES-256-GCM 模拟 AWS KMS：
//   - GenerateDataKey：随机 32 字节 DEK，encryptedDEK = AES-GCM(DEK, mockMaster,
//     12 字节随机 nonce) 的 "nonce || ciphertext"。
//   - Decrypt：拆出 nonce / ciphertext 反向走一遍。
//
// mock 语义和真 KMS 一致：明文 DEK 离开 mock 后只剩密文版本，
// 调用 Decrypt 才能还原。

type mockKMSClient struct {
	master []byte // 32 字节
	// 注入点供测试覆盖错误路径
	failGenerateDataKey error
	failDecrypt         error
	genReturnDEK        []byte // 非 nil 时强制 GenerateDataKey 返回此 DEK
	genReturnEncDEK     []byte // 非 nil 时强制 GenerateDataKey 返回此 encDEK
	decReturnPlaintext  []byte // 非 nil 时强制 Decrypt 返回此 plaintext
	// 记录调用次数，便于断言
	genCalls int
	decCalls int
}

func newMockKMS(t *testing.T) *mockKMSClient {
	t.Helper()
	m := make([]byte, 32)
	if _, err := rand.Read(m); err != nil {
		t.Fatalf("seed mock master: %v", err)
	}
	return &mockKMSClient{master: m}
}

func (m *mockKMSClient) GenerateDataKey(_ context.Context, _ string, _ string) ([]byte, []byte, error) {
	m.genCalls++
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
		// 拷贝一份避免外部测试持有的切片被 zero 掉
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
		encDEK = append(nonce, ct...)
	} else {
		c := make([]byte, len(encDEK))
		copy(c, encDEK)
		encDEK = c
	}
	return dek, encDEK, nil
}

func (m *mockKMSClient) Decrypt(_ context.Context, encryptedDEK []byte) ([]byte, error) {
	m.decCalls++
	if m.failDecrypt != nil {
		return nil, m.failDecrypt
	}
	if m.decReturnPlaintext != nil {
		c := make([]byte, len(m.decReturnPlaintext))
		copy(c, m.decReturnPlaintext)
		return c, nil
	}
	if len(encryptedDEK) < 12 {
		return nil, errors.New("mock: encDEK too short")
	}
	nonce := encryptedDEK[:12]
	ct := encryptedDEK[12:]
	block, _ := aes.NewCipher(m.master)
	aead, _ := cipher.NewGCM(block)
	return aead.Open(nil, nonce, ct, nil)
}

// ---------- helpers ----------

func mustVault(t *testing.T) (*mockKMSClient, vault.Vault) {
	t.Helper()
	m := newMockKMS(t)
	v := NewWithClient(m, "alias/cert-master-test")
	return m, v
}

// ---------- tests: construction ----------

func TestNew_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"empty all", Config{}},
		{"empty Region", Config{AccessKeyID: "a", SecretAccessKey: "b", KeyID: "k"}},
		{"empty KeyID", Config{Region: "us-east-1", AccessKeyID: "a", SecretAccessKey: "b"}},
		{"AccessKeyID only", Config{Region: "us-east-1", AccessKeyID: "a", KeyID: "k"}},
		{"SecretAccessKey only", Config{Region: "us-east-1", SecretAccessKey: "b", KeyID: "k"}},
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

func TestNew_OK_StaticCredentials(t *testing.T) {
	v, err := New(Config{
		Region:          "us-east-1",
		AccessKeyID:     "AKIAEXAMPLE0000000000",
		SecretAccessKey: "secret",
		KeyID:           "alias/cert-master",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if v.KeyID() != "alias/cert-master" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

func TestNew_OK_DefaultCredentialChain(t *testing.T) {
	// AccessKeyID + SecretAccessKey 都留空 → 走默认 chain。
	// LoadDefaultConfig 本身不发起 IMDS 请求（lazy），所以这里能跑通。
	v, err := New(Config{
		Region: "us-east-1",
		KeyID:  "alias/cert-master",
	})
	if err != nil {
		t.Fatalf("New (default chain): %v", err)
	}
	if v.KeyID() != "alias/cert-master" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

func TestNewWithClient_KeyID(t *testing.T) {
	_, v := mustVault(t)
	if v.KeyID() != "alias/cert-master-test" {
		t.Fatalf("KeyID() = %q", v.KeyID())
	}
}

// ---------- tests: GenerateKey ----------

func TestGenerateKey_ECDSA(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
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

func TestGenerateKey_KMSFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.failGenerateDataKey = errors.New("kms boom")
	_, _, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("kms boom")) {
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
	ek.KeyID = "alias/different-master"
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
	// 翻转 AES-GCM 输出的最后一字节（GCM auth tag 内）
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
	// 翻转 Ciphertext[5]，落在 encDEK 区域内（前 4 字节是长度前缀）。
	ek.Ciphertext[5] ^= 0x01
	_, err = v.DecryptKey(ctx, ek)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	// 改 encDEK 后 mock KMS Decrypt 会失败（GCM auth），错误被透传，
	// 不会归类为 ErrInvalidCiphertext（那是本地 AES-GCM 的错误）。
	if errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want non-ErrInvalidCiphertext (KMS-side failure), got %v", err)
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
	// 解开 encDEK，确认 mock KMS 能 Decrypt
	encDEK := ek.Ciphertext[4 : 4+encLen]
	dek, err := m.Decrypt(ctx, encDEK)
	if err != nil {
		t.Fatalf("mock Decrypt encDEK: %v", err)
	}
	if len(dek) != 32 {
		t.Fatalf("DEK len = %d", len(dek))
	}
}

// ---------- tests: bad KMS DEK length (defense in depth) ----------

func TestEncrypt_KMSReturnsBadDEKLen(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 16) // not 32
	m.genReturnEncDEK = []byte{0x01, 0x02, 0x03}    // 任意非空
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("DEK length")) {
		t.Fatalf("want DEK length error, got %v", err)
	}
}

func TestEncrypt_KMSReturnsEmptyEncDEK(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 32)
	m.genReturnEncDEK = []byte{}
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty encrypted DEK")) {
		t.Fatalf("want empty encDEK error, got %v", err)
	}
}

func TestEncrypt_KMSReturnsHugeEncDEK(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	m.genReturnDEK = bytes.Repeat([]byte{0xAA}, 32)
	m.genReturnEncDEK = bytes.Repeat([]byte{0xBB}, maxEncDEKLen+1)
	_, err := v.EncryptKey(ctx, []byte("x"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("exceeds")) {
		t.Fatalf("want overflow error, got %v", err)
	}
}

func TestDecrypt_KMSReturnsBadDEKLen(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	ek, err := v.EncryptKey(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.decReturnPlaintext = bytes.Repeat([]byte{0xCC}, 16) // not 32
	_, err = v.DecryptKey(ctx, ek)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("DEK length")) {
		t.Fatalf("want DEK length error, got %v", err)
	}
}

func TestDecrypt_KMSFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	ek, err := v.EncryptKey(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.failDecrypt = errors.New("kms decrypt boom")
	_, err = v.DecryptKey(ctx, ek)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("kms decrypt boom")) {
		t.Fatalf("want passthrough kms err, got %v", err)
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
	plain := bytes.Repeat([]byte("idcd-cert-blob-payload-"), 500) // ~11.5KB
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
		t.Fatal("nonces collided across encryptions")
	}
	if bytes.Equal(eb1.Ciphertext, eb2.Ciphertext) {
		t.Fatal("ciphertexts identical for same plaintext (nonce reuse?)")
	}
}

func TestDecryptBlob_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()
	_, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	eb.KeyID = "alias/another-key"
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

func TestDecryptBlob_KMSFailure(t *testing.T) {
	ctx := context.Background()
	m, v := mustVault(t)
	eb, err := v.EncryptBlob(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	m.failDecrypt = errors.New("kms outage")
	_, err = v.DecryptBlob(ctx, eb)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("kms outage")) {
		t.Fatalf("want kms outage err, got %v", err)
	}
}

// ---------- tests: error messages have context ----------

func TestErrorMessages_HaveContext(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("Region")) {
		t.Fatalf("err lacks field name: %v", err)
	}

	cases := []struct {
		cfg       Config
		wantField string
	}{
		{Config{Region: "us-east-1"}, "KeyID"},
		{Config{Region: "us-east-1", AccessKeyID: "a", KeyID: "k"}, "SecretAccessKey"},
		{Config{Region: "us-east-1", SecretAccessKey: "b", KeyID: "k"}, "AccessKeyID"},
	}
	for i, c := range cases {
		_, err := New(c.cfg)
		if err == nil {
			t.Fatalf("case %d: want err", i)
		}
		if !bytes.Contains([]byte(err.Error()), []byte(c.wantField)) {
			t.Fatalf("case %d: err lacks %q: %v", i, c.wantField, err)
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

// ---------- tests: realKMSClient context cancellation ----------
//
// realKMSClient.GenerateDataKey / Decrypt 在调用 SDK 前会先检查 ctx.Err()，
// 这是唯一不依赖真实 AWS 就能跑的路径。其余路径走 sdk 函数指针 mock。

func TestRealKMSClient_GenerateDataKey_CtxCanceled(t *testing.T) {
	r := &realKMSClient{sdk: nil, keyID: "alias/test"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := r.GenerateDataKey(ctx, "alias/test", "AES_256")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRealKMSClient_Decrypt_CtxCanceled(t *testing.T) {
	r := &realKMSClient{sdk: nil, keyID: "alias/test"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Decrypt(ctx, []byte("encdek"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// ---------- tests: realKMSClient via injected SDK function pointers ----------

func TestRealKMSClient_GenerateDataKey_SDKError(t *testing.T) {
	r := &realKMSClient{
		keyID: "alias/test",
		sdkGen: func(_ context.Context, _ *kmssdk.GenerateDataKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.GenerateDataKeyOutput, error) {
			return nil, errors.New("network down")
		},
	}
	_, _, err := r.GenerateDataKey(context.Background(), "alias/test", "AES_256")
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("network down")) {
		t.Fatalf("want sdk error passthrough, got %v", err)
	}
}

func TestRealKMSClient_GenerateDataKey_OK(t *testing.T) {
	dek := bytes.Repeat([]byte{0x42}, 32)
	enc := bytes.Repeat([]byte{0x9F}, 64)
	r := &realKMSClient{
		keyID: "alias/test",
		sdkGen: func(_ context.Context, in *kmssdk.GenerateDataKeyInput, _ ...func(*kmssdk.Options)) (*kmssdk.GenerateDataKeyOutput, error) {
			if in.KeyId == nil || *in.KeyId != "alias/test" {
				t.Errorf("KeyId = %v", in.KeyId)
			}
			if in.KeySpec != kmstypes.DataKeySpecAes256 {
				t.Errorf("KeySpec = %v", in.KeySpec)
			}
			return &kmssdk.GenerateDataKeyOutput{
				Plaintext:      append([]byte(nil), dek...),
				CiphertextBlob: append([]byte(nil), enc...),
			}, nil
		},
	}
	gotDEK, gotEnc, err := r.GenerateDataKey(context.Background(), "alias/test", "AES_256")
	if err != nil {
		t.Fatalf("GenerateDataKey: %v", err)
	}
	if !bytes.Equal(gotDEK, dek) {
		t.Errorf("DEK mismatch")
	}
	if !bytes.Equal(gotEnc, enc) {
		t.Errorf("encDEK mismatch")
	}
}

func TestRealKMSClient_Decrypt_SDKError(t *testing.T) {
	r := &realKMSClient{
		keyID: "alias/test",
		sdkDec: func(_ context.Context, _ *kmssdk.DecryptInput, _ ...func(*kmssdk.Options)) (*kmssdk.DecryptOutput, error) {
			return nil, errors.New("kms 5xx")
		},
	}
	_, err := r.Decrypt(context.Background(), []byte("encdek"))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("kms 5xx")) {
		t.Fatalf("want sdk error passthrough, got %v", err)
	}
}

func TestRealKMSClient_Decrypt_OK(t *testing.T) {
	dek := bytes.Repeat([]byte{0x77}, 32)
	r := &realKMSClient{
		keyID: "alias/test",
		sdkDec: func(_ context.Context, in *kmssdk.DecryptInput, _ ...func(*kmssdk.Options)) (*kmssdk.DecryptOutput, error) {
			if in.CiphertextBlob == nil {
				t.Fatal("CiphertextBlob nil")
			}
			if !bytes.Equal(in.CiphertextBlob, []byte("payload-encdek")) {
				t.Errorf("encDEK mismatch: %x", in.CiphertextBlob)
			}
			if in.KeyId == nil || *in.KeyId != "alias/test" {
				t.Errorf("KeyId = %v", in.KeyId)
			}
			return &kmssdk.DecryptOutput{
				Plaintext: append([]byte(nil), dek...),
			}, nil
		},
	}
	got, err := r.Decrypt(context.Background(), []byte("payload-encdek"))
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("DEK mismatch")
	}
}

// ---------- tests: parseGenerateDataKeyOutput / parseDecryptOutput ----------

func TestParseGenerateDataKeyOutput_NilOutput(t *testing.T) {
	_, _, err := parseGenerateDataKeyOutput(nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("nil output")) {
		t.Fatalf("want nil output err, got %v", err)
	}
}

func TestParseGenerateDataKeyOutput_MissingFields(t *testing.T) {
	// Plaintext empty
	_, _, err := parseGenerateDataKeyOutput(&kmssdk.GenerateDataKeyOutput{
		CiphertextBlob: []byte("enc"),
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("missing")) {
		t.Fatalf("want missing err, got %v", err)
	}
	// CiphertextBlob empty
	_, _, err = parseGenerateDataKeyOutput(&kmssdk.GenerateDataKeyOutput{
		Plaintext: []byte("plain"),
	})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("missing")) {
		t.Fatalf("want missing err, got %v", err)
	}
}

func TestParseGenerateDataKeyOutput_OK_CopiesBuffers(t *testing.T) {
	plain := []byte("plain-dek")
	enc := []byte("enc-dek")
	dek, encDEK, err := parseGenerateDataKeyOutput(&kmssdk.GenerateDataKeyOutput{
		Plaintext:      plain,
		CiphertextBlob: enc,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dek, plain) || !bytes.Equal(encDEK, enc) {
		t.Fatal("value mismatch")
	}
	// 验证拷贝：mutate 原 buffer，返回值不变。
	plain[0] = 0xFF
	enc[0] = 0xFF
	if dek[0] == 0xFF || encDEK[0] == 0xFF {
		t.Fatal("parseGenerateDataKeyOutput did not copy buffers")
	}
}

func TestParseDecryptOutput_NilOutput(t *testing.T) {
	_, err := parseDecryptOutput(nil)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("nil output")) {
		t.Fatalf("want nil output err, got %v", err)
	}
}

func TestParseDecryptOutput_EmptyPlaintext(t *testing.T) {
	_, err := parseDecryptOutput(&kmssdk.DecryptOutput{})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("empty Plaintext")) {
		t.Fatalf("want empty Plaintext err, got %v", err)
	}
}

func TestParseDecryptOutput_OK_CopiesBuffer(t *testing.T) {
	plain := []byte("plain-dek")
	dek, err := parseDecryptOutput(&kmssdk.DecryptOutput{Plaintext: plain})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dek, plain) {
		t.Fatal("value mismatch")
	}
	plain[0] = 0xFF
	if dek[0] == 0xFF {
		t.Fatal("parseDecryptOutput did not copy buffer")
	}
}

// ---------- tests: KMS calls counted (sanity) ----------

func TestKMSCallCounts(t *testing.T) {
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
