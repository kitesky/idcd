package service

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
)

// buildCSR generates a PEM-encoded PKCS#10 CertificateRequest with the
// given SAN list. CN = sans[0]. SAN list is deduplicated and sorted so
// the same (key, SANs) pair always produces the same CSR bytes — the
// crash-recovery path relies on that for idempotency.
func buildCSR(key crypto.Signer, sans []string) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("buildCSR: nil key")
	}
	if len(sans) == 0 {
		return nil, fmt.Errorf("buildCSR: empty SAN list")
	}

	normalized := normalizeSANs(sans)
	if len(normalized) == 0 {
		return nil, fmt.Errorf("buildCSR: SAN list normalized to empty")
	}
	tmpl := &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: normalized[0]},
		DNSNames: normalized,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return nil, fmt.Errorf("buildCSR: create: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}), nil
}

// normalizeSANs lowercases, deduplicates and sorts the SAN list. The
// returned slice is a copy.
func normalizeSANs(sans []string) []string {
	seen := make(map[string]struct{}, len(sans))
	out := make([]string, 0, len(sans))
	for _, s := range sans {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
