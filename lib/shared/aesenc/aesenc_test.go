package aesenc_test

import (
	"bytes"
	"testing"

	"github.com/kite365/idcd/lib/shared/aesenc"
)

var zeroKey = make([]byte, 32)

func newTestCipher(t *testing.T) *aesenc.Cipher {
	t.Helper()
	c, err := aesenc.New(zeroKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	c := newTestCipher(t)
	plain := []byte("hello world secret")
	ct, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ct, plain) {
		t.Error("ciphertext must not equal plaintext")
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Errorf("got %q, want %q", pt, plain)
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	c := newTestCipher(t)
	ct, err := c.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if len(pt) != 0 {
		t.Errorf("expected empty plaintext, got %q", pt)
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	c := newTestCipher(t)
	ct1, _ := c.Encrypt([]byte("same plaintext"))
	ct2, _ := c.Encrypt([]byte("same plaintext"))
	// First 12 bytes are the nonce — must differ across calls.
	if bytes.Equal(ct1[:12], ct2[:12]) {
		t.Error("nonces should be statistically unique")
	}
}

func TestNewFromHex(t *testing.T) {
	hexKey := "0000000000000000000000000000000000000000000000000000000000000000"
	c, err := aesenc.NewFromHex(hexKey)
	if err != nil {
		t.Fatalf("NewFromHex: %v", err)
	}
	plain := []byte("roundtrip via hex key")
	ct, _ := c.Encrypt(plain)
	pt, err := c.Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt, plain) {
		t.Errorf("got %q, want %q", pt, plain)
	}
}

func TestNewFromHex_InvalidHex(t *testing.T) {
	_, err := aesenc.NewFromHex("zzzz")
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestNew_WrongKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 24, 31, 33} {
		_, err := aesenc.New(make([]byte, n))
		if err == nil {
			t.Errorf("expected error for %d-byte key", n)
		}
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	c := newTestCipher(t)
	_, err := c.Decrypt([]byte("short"))
	if err == nil {
		t.Error("expected error for too-short input")
	}
}

func TestDecrypt_Tampered(t *testing.T) {
	c := newTestCipher(t)
	ct, _ := c.Encrypt([]byte("data"))
	ct[len(ct)-1] ^= 0xFF // flip last byte (GCM tag)
	_, err := c.Decrypt(ct)
	if err == nil {
		t.Error("expected authentication failure on tampered ciphertext")
	}
}
