package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- helpers ----------------------------------------------------------------

type testCert struct {
	cert *x509.Certificate
	der  []byte
	key  *rsa.PrivateKey
}

// genCert mints a self-signed cert (when parent == nil) or a cert signed by
// parent. tweak lets a test mutate the template (e.g. expire, drop KU).
func genCert(t *testing.T, cn string, parent *testCert, tweak func(*x509.Certificate)) *testCert {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"idcd-test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  parent == nil,
	}
	if tweak != nil {
		tweak(tmpl)
	}
	parentCert := tmpl
	parentKey := key
	if parent != nil {
		parentCert = parent.cert
		parentKey = parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parentCert, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return &testCert{cert: cert, der: der, key: key}
}

func writePEM(t *testing.T, path string, ders ...[]byte) {
	t.Helper()
	var buf strings.Builder
	for _, d := range ders {
		if err := pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "CERTIFICATE", Bytes: d}); err != nil {
			t.Fatalf("encode pem: %v", err)
		}
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
}

type stringWriter struct{ b *strings.Builder }

func (s *stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

// setEnv sets an env var for the duration of the test, restoring the prior
// value on cleanup. Empty value means "unset".
func setEnv(t *testing.T, key, value string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// --- tests ------------------------------------------------------------------

func TestLoadProdSignerCert_EnvUnset(t *testing.T) {
	setEnv(t, envSignerCertPEM, "")
	setEnv(t, envSignerChainPEM, "")

	cert, chain, src, err := loadProdSignerCert()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if cert != nil || chain != nil || src != "" {
		t.Fatalf("expected (nil,nil,\"\"); got (%v,%v,%q)", cert, chain, src)
	}
}

func TestLoadProdSignerCert_FileMissing(t *testing.T) {
	dir := t.TempDir()
	setEnv(t, envSignerCertPEM, filepath.Join(dir, "does-not-exist.pem"))
	setEnv(t, envSignerChainPEM, "")

	_, _, _, err := loadProdSignerCert()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "load signer leaf cert") {
		t.Fatalf("error should mention leaf load: %v", err)
	}
}

func TestLoadProdSignerCert_GarbageFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "junk.pem")
	if err := os.WriteFile(path, []byte("not pem data at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, envSignerCertPEM, path)
	setEnv(t, envSignerChainPEM, "")

	_, _, _, err := loadProdSignerCert()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no PEM block") {
		t.Fatalf("expected 'no PEM block' error, got: %v", err)
	}
}

func TestLoadProdSignerCert_WrongPEMBlockType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wrong.pem")
	var buf strings.Builder
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("xx")})
	if err := os.WriteFile(path, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, envSignerCertPEM, path)

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "expected CERTIFICATE") {
		t.Fatalf("expected CERTIFICATE-type error, got: %v", err)
	}
}

func TestLoadProdSignerCert_UnparseableCert(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.pem")
	var buf strings.Builder
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "CERTIFICATE", Bytes: []byte("garbage-der-bytes")})
	if err := os.WriteFile(path, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, envSignerCertPEM, path)

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "parse x509") {
		t.Fatalf("expected parse x509 error, got: %v", err)
	}
}

func TestLoadProdSignerCert_Expired(t *testing.T) {
	leaf := genCert(t, "expired-leaf", nil, func(c *x509.Certificate) {
		c.NotBefore = time.Now().Add(-48 * time.Hour)
		c.NotAfter = time.Now().Add(-1 * time.Hour)
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "leaf.pem")
	writePEM(t, path, leaf.der)
	setEnv(t, envSignerCertPEM, path)
	setEnv(t, envSignerChainPEM, "")

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got: %v", err)
	}
}

func TestLoadProdSignerCert_MissingDigitalSignatureKeyUsage(t *testing.T) {
	leaf := genCert(t, "no-ku", nil, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageCertSign // no DigitalSignature
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "leaf.pem")
	writePEM(t, path, leaf.der)
	setEnv(t, envSignerCertPEM, path)
	setEnv(t, envSignerChainPEM, "")

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "DigitalSignature") {
		t.Fatalf("expected DigitalSignature error, got: %v", err)
	}
}

func TestLoadProdSignerCert_HappyPath_NoChain(t *testing.T) {
	leaf := genCert(t, "happy-leaf", nil, nil)
	dir := t.TempDir()
	path := filepath.Join(dir, "leaf.pem")
	writePEM(t, path, leaf.der)
	setEnv(t, envSignerCertPEM, path)
	setEnv(t, envSignerChainPEM, "")

	cert, chain, src, err := loadProdSignerCert()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cert == nil {
		t.Fatal("cert nil")
	}
	if len(chain) != 0 {
		t.Fatalf("expected empty chain, got %d", len(chain))
	}
	if src != "pem-file" {
		t.Fatalf("source = %q, want %q", src, "pem-file")
	}
	if cert.Subject.CommonName != "happy-leaf" {
		t.Fatalf("CN = %q", cert.Subject.CommonName)
	}
}

func TestLoadProdSignerCert_HappyPath_WithChain(t *testing.T) {
	ca := genCert(t, "test-ca", nil, nil)
	leaf := genCert(t, "happy-leaf-chained", ca, func(c *x509.Certificate) {
		c.IsCA = false
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})

	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	chainPath := filepath.Join(dir, "chain.pem")
	writePEM(t, leafPath, leaf.der)
	writePEM(t, chainPath, ca.der)

	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, chainPath)

	cert, chain, src, err := loadProdSignerCert()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cert.Subject.CommonName != "happy-leaf-chained" {
		t.Fatalf("leaf CN = %q", cert.Subject.CommonName)
	}
	if len(chain) != 1 {
		t.Fatalf("chain len = %d, want 1", len(chain))
	}
	if chain[0].Subject.CommonName != "test-ca" {
		t.Fatalf("chain[0] CN = %q", chain[0].Subject.CommonName)
	}
	if src != "pem-file" {
		t.Fatalf("source = %q", src)
	}
}

func TestLoadProdSignerCert_ChainMismatch(t *testing.T) {
	caA := genCert(t, "ca-a", nil, nil)
	caB := genCert(t, "ca-b", nil, nil)
	leaf := genCert(t, "mismatched-leaf", caA, func(c *x509.Certificate) {
		c.IsCA = false
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})

	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	chainPath := filepath.Join(dir, "chain.pem")
	writePEM(t, leafPath, leaf.der)
	writePEM(t, chainPath, caB.der) // wrong CA

	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, chainPath)

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "not signed by chain[0]") {
		t.Fatalf("expected chain mismatch error, got: %v", err)
	}
}

func TestLoadProdSignerCert_ChainEmptyFile(t *testing.T) {
	leaf := genCert(t, "leaf", nil, nil)
	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	chainPath := filepath.Join(dir, "empty-chain.pem")
	writePEM(t, leafPath, leaf.der)
	if err := os.WriteFile(chainPath, []byte("this file has no PEM blocks"), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, chainPath)

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "no CERTIFICATE blocks") {
		t.Fatalf("expected no-CERTIFICATE-blocks error, got: %v", err)
	}
}

func TestLoadProdSignerCert_ChainFileMissing(t *testing.T) {
	leaf := genCert(t, "leaf", nil, nil)
	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	writePEM(t, leafPath, leaf.der)
	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, filepath.Join(dir, "missing-chain.pem"))

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "load signer chain") {
		t.Fatalf("expected chain load error, got: %v", err)
	}
}

func TestLoadProdSignerCert_ExpiringSoonAnnotated(t *testing.T) {
	leaf := genCert(t, "expiring-soon", nil, func(c *x509.Certificate) {
		c.NotAfter = time.Now().Add(7 * 24 * time.Hour) // < 30d window
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "leaf.pem")
	writePEM(t, path, leaf.der)
	setEnv(t, envSignerCertPEM, path)
	setEnv(t, envSignerChainPEM, "")

	cert, _, src, err := loadProdSignerCert()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cert == nil {
		t.Fatal("cert nil")
	}
	if !strings.Contains(src, "expiring soon") {
		t.Fatalf("expected expiring-soon annotation in source, got %q", src)
	}
}

func TestLoadProdSignerCert_MultiBlockChainSkipsNonCert(t *testing.T) {
	ca := genCert(t, "ca-multi", nil, nil)
	intermediate := genCert(t, "intermediate-multi", ca, func(c *x509.Certificate) {
		c.IsCA = true
		c.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	})
	leaf := genCert(t, "leaf-multi", intermediate, func(c *x509.Certificate) {
		c.IsCA = false
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})

	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	chainPath := filepath.Join(dir, "chain.pem")
	writePEM(t, leafPath, leaf.der)

	// Write chain: intermediate + a junk PRIVATE KEY block + ca.
	var buf strings.Builder
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "CERTIFICATE", Bytes: intermediate.der})
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not used")})
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "CERTIFICATE", Bytes: ca.der})
	if err := os.WriteFile(chainPath, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, chainPath)

	cert, chain, _, err := loadProdSignerCert()
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cert.Subject.CommonName != "leaf-multi" {
		t.Fatalf("leaf CN = %q", cert.Subject.CommonName)
	}
	if len(chain) != 2 {
		t.Fatalf("chain len = %d, want 2 (intermediate + ca, junk block skipped)", len(chain))
	}
	if chain[0].Subject.CommonName != "intermediate-multi" || chain[1].Subject.CommonName != "ca-multi" {
		t.Fatalf("chain order wrong: %q, %q", chain[0].Subject.CommonName, chain[1].Subject.CommonName)
	}
}

func TestLoadProdSignerCert_ChainHasUnparseableCert(t *testing.T) {
	leaf := genCert(t, "leaf", nil, nil)
	dir := t.TempDir()
	leafPath := filepath.Join(dir, "leaf.pem")
	chainPath := filepath.Join(dir, "chain.pem")
	writePEM(t, leafPath, leaf.der)

	var buf strings.Builder
	_ = pem.Encode(&stringWriter{&buf}, &pem.Block{Type: "CERTIFICATE", Bytes: []byte("bogus")})
	if err := os.WriteFile(chainPath, []byte(buf.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	setEnv(t, envSignerCertPEM, leafPath)
	setEnv(t, envSignerChainPEM, chainPath)

	_, _, _, err := loadProdSignerCert()
	if err == nil || !strings.Contains(err.Error(), "parse x509") {
		t.Fatalf("expected parse x509 error from chain, got: %v", err)
	}
}
