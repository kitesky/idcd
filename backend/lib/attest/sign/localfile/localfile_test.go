package localfile

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kite365/idcd/lib/attest/sign"
)

func TestNew_CreatesKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "key.pem")

	s, err := New(Config{KeyPath: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected keyfile created at %s: %v", path, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %v", info.Mode().Perm())
	}
	if s.Algorithm() != sign.AlgorithmRSAPKCS1SHA256 {
		t.Fatalf("default algorithm mismatch: %s", s.Algorithm())
	}
}

func TestNew_LoadsExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")

	s1, err := New(Config{KeyPath: path})
	if err != nil {
		t.Fatal(err)
	}
	pk1 := s1.(PrivateKeyHolder).PrivateKey()

	s2, err := New(Config{KeyPath: path})
	if err != nil {
		t.Fatal(err)
	}
	pk2 := s2.(PrivateKeyHolder).PrivateKey()

	if pk1.N.Cmp(pk2.N) != 0 {
		t.Fatal("reload produced a different key")
	}
}

func TestNew_RejectsEmptyKeyPath(t *testing.T) {
	_, err := New(Config{KeyPath: ""})
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNew_RejectsUnsupportedAlgorithm(t *testing.T) {
	dir := t.TempDir()
	_, err := New(Config{KeyPath: filepath.Join(dir, "k.pem"), Algorithm: sign.AlgorithmECDSASHA256})
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestSign_Verify_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.pem")
	s, err := New(Config{KeyPath: path})
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("hello idcd")
	digest := sha256.Sum256(msg)
	sig, err := s.Sign(context.Background(), digest[:], "idem-1")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("empty signature")
	}

	pk := s.(PrivateKeyHolder).PrivateKey()
	if err := rsa.VerifyPKCS1v15(&pk.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("VerifyPKCS1v15: %v", err)
	}
}

func TestSign_IdempotencyCache(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(Config{KeyPath: filepath.Join(dir, "k.pem")})

	digest := sha256.Sum256([]byte("payload"))
	sig1, err := s.Sign(context.Background(), digest[:], "same-key")
	if err != nil {
		t.Fatal(err)
	}
	sig2, err := s.Sign(context.Background(), digest[:], "same-key")
	if err != nil {
		t.Fatal(err)
	}
	if string(sig1) != string(sig2) {
		t.Fatal("idempotency cache miss: sig differs across calls")
	}
}

func TestSign_RejectsEmptyIdempotencyKey(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(Config{KeyPath: filepath.Join(dir, "k.pem")})
	digest := sha256.Sum256([]byte("x"))
	_, err := s.Sign(context.Background(), digest[:], "")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestSign_RejectsWrongDigestLen(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(Config{KeyPath: filepath.Join(dir, "k.pem")})
	_, err := s.Sign(context.Background(), []byte("short"), "idem")
	if !errors.Is(err, sign.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestSigner_KeyIDAndAlgorithm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.pem")
	s, _ := New(Config{KeyPath: path, Algorithm: sign.AlgorithmRSAPSSSHA512})
	if s.KeyID() != "localfile:"+path {
		t.Fatalf("KeyID: %s", s.KeyID())
	}
	if s.Algorithm() != sign.AlgorithmRSAPSSSHA512 {
		t.Fatalf("Algorithm: %s", s.Algorithm())
	}
}

func TestVerifier_Algorithm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.pem")
	if _, err := New(Config{KeyPath: path, Algorithm: sign.AlgorithmRSAPSSSHA384}); err != nil {
		t.Fatal(err)
	}
	v, _ := NewVerifier(Config{KeyPath: path, Algorithm: sign.AlgorithmRSAPSSSHA384})
	if v.Algorithm() != sign.AlgorithmRSAPSSSHA384 {
		t.Fatalf("Algorithm: %s", v.Algorithm())
	}
}

func TestVerifier_PublicKeyPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.pem")
	if _, err := New(Config{KeyPath: path}); err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(Config{KeyPath: path})
	if err != nil {
		t.Fatal(err)
	}
	pem, err := v.PublicKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(pem[:27]) != "-----BEGIN PUBLIC KEY-----\n" {
		t.Fatalf("expected PEM PUBLIC KEY header, got %q", pem[:27])
	}
	if v.KeyID() != "localfile:"+path {
		t.Fatalf("unexpected KeyID: %s", v.KeyID())
	}
}

func TestKeyVersion_StableAcrossReads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "k.pem")
	s, _ := New(Config{KeyPath: path})
	v1, err := s.KeyVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	v2, err := s.KeyVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v1 != v2 {
		t.Fatalf("KeyVersion not stable: %d vs %d", v1, v2)
	}
}

func TestPSSAlgorithms(t *testing.T) {
	for _, alg := range []string{
		sign.AlgorithmRSAPSSSHA256,
		sign.AlgorithmRSAPSSSHA384,
		sign.AlgorithmRSAPSSSHA512,
	} {
		t.Run(alg, func(t *testing.T) {
			dir := t.TempDir()
			s, err := New(Config{KeyPath: filepath.Join(dir, "k.pem"), Algorithm: alg})
			if err != nil {
				t.Fatal(err)
			}
			hash, _ := sign.HashAlgFor(alg)
			h := hash.New()
			h.Write([]byte("data"))
			digest := h.Sum(nil)
			sig, err := s.Sign(context.Background(), digest, "k")
			if err != nil {
				t.Fatal(err)
			}
			if len(sig) == 0 {
				t.Fatal("empty sig")
			}
		})
	}
}

func TestParsePEMKey_RejectsGarbage(t *testing.T) {
	if _, err := parsePEMKey([]byte("not a pem")); err == nil {
		t.Fatal("expected error on non-PEM input")
	}
	if _, err := parsePEMKey([]byte("-----BEGIN CERTIFICATE-----\nABC\n-----END CERTIFICATE-----\n")); err == nil {
		t.Fatal("expected error on non-key PEM")
	}
}
