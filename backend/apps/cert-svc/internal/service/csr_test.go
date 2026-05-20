package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCSR_HappyPath(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	pemBytes, err := buildCSR(key, []string{"foo.example.com", "bar.example.com"})
	require.NoError(t, err)

	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block)
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	require.NoError(t, err)

	// SAN list should be sorted ASC, CN = first sorted entry.
	assert.Equal(t, []string{"bar.example.com", "foo.example.com"}, csr.DNSNames)
	assert.Equal(t, "bar.example.com", csr.Subject.CommonName)
	require.NoError(t, csr.CheckSignature())
}

func TestBuildCSR_Deterministic(t *testing.T) {
	// Same (key, SANs) → identical SAN list ordering. The signature itself
	// is non-deterministic for ECDSA, but the to-be-signed structure must
	// match.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	a, _ := buildCSR(key, []string{"a.com", "b.com", "a.com"})
	b, _ := buildCSR(key, []string{"B.com", " A.COM ", "a.com"})

	blockA, _ := pem.Decode(a)
	blockB, _ := pem.Decode(b)
	csrA, _ := x509.ParseCertificateRequest(blockA.Bytes)
	csrB, _ := x509.ParseCertificateRequest(blockB.Bytes)

	assert.Equal(t, csrA.DNSNames, csrB.DNSNames)
	assert.Equal(t, csrA.Subject.CommonName, csrB.Subject.CommonName)
}

func TestBuildCSR_Errors(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	_, err := buildCSR(nil, []string{"x.com"})
	assert.Error(t, err)

	_, err = buildCSR(key, nil)
	assert.Error(t, err)

	_, err = buildCSR(key, []string{"", " "})
	assert.Error(t, err)
}

func TestNormalizeSANs(t *testing.T) {
	got := normalizeSANs([]string{"Foo.COM", " bar.com ", "foo.com", ""})
	assert.Equal(t, []string{"bar.com", "foo.com"}, got)
}
