package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"time"
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
	return nil, nil, "", nil
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
