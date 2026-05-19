package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"time"
)

// envSignerCertPEM / envSignerChainPEM are the env vars the production
// loader consults. Held as constants so tests can reuse them.
const (
	envSignerCertPEM  = "ATTEST_SIGNER_CERT_PEM"
	envSignerChainPEM = "ATTEST_SIGNER_CHAIN_PEM"

	// signerCertExpiryWarnWindow — if leaf.NotAfter is within this
	// window the source string is annotated so wireSignerCert's
	// existing INFO log surfaces it (operators grep for "expiring").
	signerCertExpiryWarnWindow = 30 * 24 * time.Hour
)

// signerPubKeySource is the minimal interface wireSignerCert needs to
// bind the loaded X.509 cert to the KMS-held signing key. The concrete
// implementation passed at runtime is a sign.Verifier (built from the
// same KMS config as the sign.Signer), which exposes both KeyID() and
// PublicKey(ctx) — we accept it as a local interface so this file
// stays decoupled from the sign package's broader API surface and so
// tests can fake it without spinning up a KMS client.
type signerPubKeySource interface {
	KeyID() string
	PublicKey(ctx context.Context) ([]byte, error)
}

// wireSignerCert resolves the X.509 cert pdfsign attaches to the CMS
// SignedData. Picks production loader when ATTEST_SIGNER_CERT_PEM is
// set; otherwise falls back to the dev fixture and logs a loud warning.
//
// When a production cert loads successfully, wireSignerCert ALSO binds
// the cert's public key to the KMS signer's public key — a mismatch
// here would mean every PDF we sign fails /verify with "pubkey
// mismatch", so we refuse to start. The bind is intentionally skipped
// for the dev fixture path because the self-signed dev cert is never
// expected to match the KMS key (pre-prod runs validate pipeline
// mechanics only; verifiers are expected to reject those PDFs).
//
// The production loader implementation lives next to this function
// (loadProdSignerCert). It is intentionally kept out of main.go so the
// cmd-wiring file stays at one screen.
func wireSignerCert(log *slog.Logger, signer signerPubKeySource) (*x509.Certificate, []*x509.Certificate, error) {
	cert, chain, source, err := loadProdSignerCert()
	if err != nil {
		return nil, nil, err
	}
	if cert != nil {
		if err := bindCertToKMS(cert, signer); err != nil {
			return nil, nil, err
		}
		log.Info("attest-generator: signer certificate loaded",
			"source", source,
			"subject", cert.Subject.CommonName,
			"issuer", cert.Issuer.CommonName,
			"not_after", cert.NotAfter.Format(time.RFC3339),
			"chain_len", len(chain),
			"kms_key_id", signer.KeyID(),
		)
		return cert, chain, nil
	}

	devCert, devChain, err := loadDevSignerCert()
	if err != nil {
		return nil, nil, err
	}
	log.Warn("attest-generator: using DEV signer certificate (self-signed); production must override before S2 GA",
		"subject", devCert.Subject.CommonName,
		"not_after", devCert.NotAfter.Format(time.RFC3339),
	)
	return devCert, devChain, nil
}

// bindCertToKMS verifies that cert.PublicKey is bit-for-bit identical
// to the public key the KMS signer would expose via GetPublicKey. We
// compare PKIX SubjectPublicKeyInfo DER bytes (via
// crypto/x509.MarshalPKIXPublicKey) on both sides — that works for
// RSA / ECDSA / Ed25519 uniformly without per-type type switches.
//
// signer.PublicKey returns a PEM-wrapped PKIX SubjectPublicKeyInfo (the
// convention all KMS adapters in this repo follow). We parse it back
// into a crypto.PublicKey and re-marshal so the comparison is robust to
// trailing whitespace / extra PEM headers from different KMS providers.
func bindCertToKMS(cert *x509.Certificate, signer signerPubKeySource) error {
	if signer == nil {
		return fmt.Errorf("bind signer cert to KMS: signer is nil")
	}
	// Use a fresh, bounded context: at startup we don't yet have a
	// request-scoped ctx, and KMS GetPublicKey should comfortably
	// finish in seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pemBytes, err := signer.PublicKey(ctx)
	if err != nil {
		return fmt.Errorf("bind signer cert to KMS: fetch KMS public key (key_id=%q, subject=%q): %w",
			signer.KeyID(), cert.Subject.CommonName, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return fmt.Errorf("bind signer cert to KMS: KMS PublicKey not PEM-encoded (key_id=%q)", signer.KeyID())
	}
	kmsPub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("bind signer cert to KMS: parse KMS PKIX (key_id=%q): %w", signer.KeyID(), err)
	}
	kmsDER, err := x509.MarshalPKIXPublicKey(kmsPub)
	if err != nil {
		return fmt.Errorf("bind signer cert to KMS: marshal KMS pubkey (key_id=%q): %w", signer.KeyID(), err)
	}
	certDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return fmt.Errorf("bind signer cert to KMS: marshal cert pubkey (subject=%q): %w", cert.Subject.CommonName, err)
	}
	if !bytes.Equal(kmsDER, certDER) {
		return fmt.Errorf("bind signer cert to KMS: public key mismatch (kms_key_id=%q, cert_subject=%q) — the cert in ATTEST_SIGNER_CERT_PEM does not match the KMS-held key; every PDF would fail /verify",
			signer.KeyID(), cert.Subject.CommonName)
	}
	return nil
}

// loadProdSignerCert reads the production signer cert + optional chain
// from the ATTEST_SIGNER_CERT_PEM / ATTEST_SIGNER_CHAIN_PEM file paths.
// Returns (nil, nil, "", nil) when ATTEST_SIGNER_CERT_PEM is unset so
// wireSignerCert can fall back to the dev fixture.
//
// Validation performed:
//   - leaf cert parses as a valid x509
//   - leaf.NotAfter is in the future
//   - leaf KeyUsage includes DigitalSignature
//   - if chain provided, leaf is signed by chain[0]
func loadProdSignerCert() (*x509.Certificate, []*x509.Certificate, string, error) {
	certPath := os.Getenv(envSignerCertPEM)
	if certPath == "" {
		// Not configured — let wireSignerCert fall back to the dev fixture.
		return nil, nil, "", nil
	}

	leaf, err := readSingleCertPEM(certPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("load signer leaf cert %q: %w", certPath, err)
	}

	if !leaf.NotAfter.After(time.Now()) {
		return nil, nil, "", fmt.Errorf("signer leaf cert expired: not_after=%s", leaf.NotAfter.Format(time.RFC3339))
	}
	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, nil, "", fmt.Errorf("signer leaf cert missing DigitalSignature key usage (subject=%q)", leaf.Subject.CommonName)
	}

	var chain []*x509.Certificate
	if chainPath := os.Getenv(envSignerChainPEM); chainPath != "" {
		chain, err = readCertChainPEM(chainPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("load signer chain %q: %w", chainPath, err)
		}
		if len(chain) == 0 {
			return nil, nil, "", fmt.Errorf("signer chain %q contained no CERTIFICATE blocks", chainPath)
		}
		// Chain verification: leaf must be signed by chain[0]; we use
		// CheckSignatureFrom rather than x509.Certificate.Verify so we
		// don't require chain[n-1] to be a system-trusted root — the
		// production chain typically terminates in a private CA root
		// that operators pin out-of-band.
		if err := leaf.CheckSignatureFrom(chain[0]); err != nil {
			return nil, nil, "", fmt.Errorf("signer leaf not signed by chain[0] (issuer=%q): %w", chain[0].Subject.CommonName, err)
		}
	}

	source := "pem-file"
	if time.Until(leaf.NotAfter) < signerCertExpiryWarnWindow {
		source = "pem-file (expiring soon)"
	}
	return leaf, chain, source, nil
}

// readSingleCertPEM reads a PEM file expected to contain exactly one
// CERTIFICATE block and returns the parsed x509 cert.
func readSingleCertPEM(path string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("expected CERTIFICATE PEM block, got %q", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse x509: %w", err)
	}
	return cert, nil
}

// readCertChainPEM reads a PEM file that may contain one or more
// CERTIFICATE blocks (concatenated) and returns them in file order.
// Non-CERTIFICATE blocks are skipped.
func readCertChainPEM(path string) ([]*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	var certs []*x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse x509 block #%d: %w", len(certs)+1, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// loadDevSignerCert returns a freshly-generated self-signed RSA-2048
// certificate that pdfsign can attach to the CMS SignedData. The cert
// chain is empty (self-signed). This is the pre-prod fallback used
// when ATTEST_SIGNER_CERT_PEM is unset; verifiers will reject the
// resulting PDFs because the dev key never matches the KMS-held key.
func loadDevSignerCert() (*x509.Certificate, []*x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate dev RSA key: %w", err)
	}
	return signerCertForKey(key)
}

// signerCertForKey 用传入的 RSA 私钥生成一份 self-signed cert，cert 的
// SubjectPublicKey == key 的公钥。Local backend 走这个路径，能保证 cert
// 与 localfile signer 的 key 严格匹配，PDF embed 后 verifier 能用 cert
// 公钥验证嵌入的 CMS signature。
func signerCertForKey(key *rsa.PrivateKey) (*x509.Certificate, []*x509.Certificate, error) {
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "idcd Evidence (DEV)",
			Organization: []string{"idcd"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create dev cert: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse dev cert: %w", err)
	}
	return cert, nil, nil
}
