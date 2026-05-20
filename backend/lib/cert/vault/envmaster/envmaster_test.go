package envmaster

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/cert/vault"
)

func mkKey(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func mustVault(t *testing.T, key []byte) vault.Vault {
	t.Helper()
	v, err := NewWithKey(key)
	if err != nil {
		t.Fatalf("NewWithKey: %v", err)
	}
	return v
}

func TestNewFromEnv_Missing(t *testing.T) {
	const name = "CERT_MASTER_KEY_TEST_MISSING"
	t.Setenv(name, "")
	_, err := NewFromEnv(name)
	if !errors.Is(err, vault.ErrMasterKeyMissing) {
		t.Fatalf("want ErrMasterKeyMissing, got %v", err)
	}
}

func TestNewFromEnv_NotBase64(t *testing.T) {
	const name = "CERT_MASTER_KEY_TEST_BADB64"
	t.Setenv(name, "@@@not-base64@@@")
	_, err := NewFromEnv(name)
	if !errors.Is(err, vault.ErrMasterKeyMissing) {
		t.Fatalf("want ErrMasterKeyMissing, got %v", err)
	}
}

func TestNewFromEnv_WrongLen(t *testing.T) {
	const name = "CERT_MASTER_KEY_TEST_WRONGLEN"
	t.Setenv(name, base64.StdEncoding.EncodeToString([]byte("too-short")))
	_, err := NewFromEnv(name)
	if !errors.Is(err, vault.ErrMasterKeyMissing) {
		t.Fatalf("want ErrMasterKeyMissing, got %v", err)
	}
}

func TestNewFromEnv_OK_DefaultName(t *testing.T) {
	t.Setenv("CERT_MASTER_KEY", base64.StdEncoding.EncodeToString(mkKey(0x11)))
	v, err := NewFromEnv("")
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	if v.KeyID() == "" {
		t.Fatal("KeyID empty")
	}
}

func TestNewFromEnv_OK_CustomName(t *testing.T) {
	const name = "CERT_MASTER_KEY_TEST_OK"
	t.Setenv(name, base64.StdEncoding.EncodeToString(mkKey(0x22)))
	v, err := NewFromEnv(name)
	if err != nil {
		t.Fatalf("NewFromEnv: %v", err)
	}
	if v.KeyID() == "" {
		t.Fatal("KeyID empty")
	}
}

func TestNewWithKey_WrongLen(t *testing.T) {
	_, err := NewWithKey([]byte("short"))
	if !errors.Is(err, vault.ErrMasterKeyMissing) {
		t.Fatalf("want ErrMasterKeyMissing, got %v", err)
	}
	_, err = NewWithKey(nil)
	if !errors.Is(err, vault.ErrMasterKeyMissing) {
		t.Fatalf("nil: want ErrMasterKeyMissing, got %v", err)
	}
}

func TestKeyID_Stable(t *testing.T) {
	key := mkKey(0xAB)
	v1 := mustVault(t, key)
	v2 := mustVault(t, key)
	if v1.KeyID() != v2.KeyID() {
		t.Fatalf("KeyID not stable: %q vs %q", v1.KeyID(), v2.KeyID())
	}
	if len(v1.KeyID()) != 16 {
		t.Fatalf("KeyID len = %d, want 16", len(v1.KeyID()))
	}
}

func TestKeyID_Distinct(t *testing.T) {
	v1 := mustVault(t, mkKey(0x01))
	v2 := mustVault(t, mkKey(0x02))
	if v1.KeyID() == v2.KeyID() {
		t.Fatalf("distinct keys produced same KeyID: %q", v1.KeyID())
	}
}

func TestGenerateAndDecrypt_ECDSA(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0x33))
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
		t.Errorf("ek.KeyID mismatch")
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

func TestGenerateAndDecrypt_RSA(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0x44))
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
	v := mustVault(t, mkKey(0x55))
	_, _, err := v.GenerateKey(ctx, vault.KeyAlg("ed25519-magic"))
	if !errors.Is(err, vault.ErrUnsupportedAlg) {
		t.Fatalf("want ErrUnsupportedAlg, got %v", err)
	}
}

func TestEncryptKey_Roundtrip(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0x66))

	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	plain := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	ek, err := v.EncryptKey(ctx, plain)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if ek.KeyID != v.KeyID() {
		t.Errorf("KeyID mismatch")
	}
	dec, err := v.DecryptKey(ctx, ek)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestEncryptBlob_Roundtrip_Empty(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0x77))
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
	v := mustVault(t, mkKey(0x88))
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
	v := mustVault(t, mkKey(0x99))
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

func TestDecryptKey_TamperedCiphertext(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0xAA))
	_, ek, err := v.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	ek.Ciphertext[0] ^= 0x01
	_, err = v.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

func TestDecryptKey_TamperedNonce(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0xBB))
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
	v := mustVault(t, mkKey(0xBC))
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

func TestDecryptKey_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()
	v1 := mustVault(t, mkKey(0xCC))
	v2 := mustVault(t, mkKey(0xDD))
	_, ek, err := v1.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v2.DecryptKey(ctx, ek)
	if !errors.Is(err, vault.ErrKeyIDMismatch) {
		t.Fatalf("want ErrKeyIDMismatch, got %v", err)
	}
}

func TestDecryptBlob_KeyIDMismatch(t *testing.T) {
	ctx := context.Background()
	v1 := mustVault(t, mkKey(0xEE))
	v2 := mustVault(t, mkKey(0xEF))
	eb, err := v1.EncryptBlob(ctx, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = v2.DecryptBlob(ctx, eb)
	if !errors.Is(err, vault.ErrKeyIDMismatch) {
		t.Fatalf("want ErrKeyIDMismatch, got %v", err)
	}
}

func TestDecryptBlob_Tampered(t *testing.T) {
	ctx := context.Background()
	v := mustVault(t, mkKey(0xF0))
	eb, err := v.EncryptBlob(ctx, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	eb.Ciphertext[0] ^= 0x80
	_, err = v.DecryptBlob(ctx, eb)
	if !errors.Is(err, vault.ErrInvalidCiphertext) {
		t.Fatalf("want ErrInvalidCiphertext, got %v", err)
	}
}

// Sanity: the error messages embed context (key id, env var name) — make sure
// they remain wrapped so errors.Is works AND surface useful detail.
func TestErrorMessages_HaveContext(t *testing.T) {
	const name = "CERT_MASTER_KEY_TEST_CTX"
	t.Setenv(name, "")
	_, err := NewFromEnv(name)
	if err == nil || !strings.Contains(err.Error(), name) {
		t.Fatalf("want error to mention %q, got %v", name, err)
	}
}
