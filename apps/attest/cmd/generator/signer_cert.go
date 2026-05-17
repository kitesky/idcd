package main

import (
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

// wireSignerCert resolves the X.509 cert pdfsign attaches to the CMS
// SignedData. Picks production loader when ATTEST_SIGNER_CERT_PEM is
// set; otherwise falls back to the dev fixture and logs a loud warning.
//
// The production loader implementation lives next to this function
// (loadProdSignerCert). It is intentionally kept out of main.go so the
// cmd-wiring file stays at one screen.
func wireSignerCert(log *slog.Logger) (*x509.Certificate, []*x509.Certificate, error) {
	cert, chain, source, err := loadProdSignerCert()
	if err != nil {
		return nil, nil, err
	}
	if cert != nil {
		log.Info("attest-generator: signer certificate loaded",
			"source", source,
			"subject", cert.Subject.CommonName,
			"issuer", cert.Issuer.CommonName,
			"not_after", cert.NotAfter.Format(time.RFC3339),
			"chain_len", len(chain),
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

// loadProdSignerCert is the production-cert loader. The first return
// value is nil + source="" when no production cert is configured (env
// vars unset), signalling wireSignerCert to fall back to the dev
// fixture. The S2-W7 task will replace this stub with a real PEM-file
// loader; until then it always returns (nil, nil, "", nil).
//
// Contract for the real impl:
//
//	ATTEST_SIGNER_CERT_PEM   path to the leaf signer cert (PEM)
//	ATTEST_SIGNER_CHAIN_PEM  path to the intermediates (PEM concat)
//	                         optional; empty → empty chain
//	                         (pdfsign will only emit leaf)
//
// Validation the real impl must perform:
//   - leaf cert parses as a valid x509
//   - leaf.NotAfter is in the future (warn if < 30d remaining)
//   - leaf KeyUsage includes DigitalSignature
//   - if chain provided, leaf chains up to the last chain cert
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
// chain is empty (self-signed).
//
// TODO(prod): replace this with a real cert loaded from
//
//	ATTEST_SIGNER_CERT_PEM
//	ATTEST_SIGNER_CHAIN_PEM
//
// before S2 GA. The dev fixture lets the binary boot in pre-prod /
// integration environments without external secrets; verifiers will
// reject signatures (the public key won't match the KMS-held key
// returned by verifier.PublicKey() during the bind-to-KMS check).
//
// This is intentional: pre-prod runs validate the pipeline mechanics
// without producing PDFs that pass /verify.
func loadDevSignerCert() (*x509.Certificate, []*x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate dev RSA key: %w", err)
	}
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
