package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

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
