package rfc3161client

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digitorus/timestamp"

	"github.com/kite365/idcd/lib/attest/tsa"
)

// newTestTSA builds a fresh RSA-2048 cert+key the mock TSA server can
// sign with. We do not reuse the digicert/digicert_test.go fixture
// because importing across `_test.go` boundaries between packages would
// require exporting it; a freshly generated key is functionally
// equivalent for protocol-level tests.
func newTestTSA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return cert, key
}

// stampResponse signs an RFC3161 response over the supplied request body
// using the supplied cert+key. Mirrors what a real TSA does on the wire.
func stampResponse(t *testing.T, cert *x509.Certificate, key *rsa.PrivateKey, body []byte) []byte {
	t.Helper()
	req, err := timestamp.ParseRequest(body)
	if err != nil {
		t.Fatalf("parse req: %v", err)
	}
	ts := timestamp.Timestamp{
		HashAlgorithm: req.HashAlgorithm,
		HashedMessage: req.HashedMessage,
		Time:          time.Now(),
		Nonce:         req.Nonce,
		Qualified:     false,
		Policy:        asn1.ObjectIdentifier{1, 2, 3, 4},
		SerialNumber:  big.NewInt(1),
	}
	out, err := ts.CreateResponseWithOpts(cert, key, crypto.SHA256)
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	return out
}

func TestStamp_HappyPath(t *testing.T) {
	cert, key := newTestTSA(t)
	digest := sha256.Sum256([]byte("payload"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/timestamp-query" {
			t.Errorf("bad content type: %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/timestamp-reply" {
			t.Errorf("bad accept: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(stampResponse(t, cert, key, body))
	}))
	defer srv.Close()

	tok, issued, err := Stamp(context.Background(),
		Config{Endpoint: srv.URL, ProviderName: "test"},
		crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("Stamp: %v", err)
	}
	if len(tok) == 0 {
		t.Fatal("empty token")
	}
	if issued.IsZero() {
		t.Fatal("zero issuedAt")
	}
}

func TestStamp_InvalidDigest(t *testing.T) {
	_, _, err := Stamp(context.Background(),
		Config{Endpoint: "http://invalid"},
		crypto.SHA256, []byte("short"))
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestStamp_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	digest := sha256.Sum256([]byte("p"))
	_, _, err := Stamp(ctx, Config{Endpoint: srv.URL}, crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestStamp_NetworkRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	digest := sha256.Sum256([]byte("p"))
	_, _, err = Stamp(context.Background(), Config{Endpoint: "http://" + addr},
		crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestStamp_NonAsn1Body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write([]byte("not asn.1"))
	}))
	defer srv.Close()
	digest := sha256.Sum256([]byte("p"))
	_, _, err := Stamp(context.Background(), Config{Endpoint: srv.URL},
		crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse, got %v", err)
	}
}

func TestStamp_DigestMismatch(t *testing.T) {
	cert, key := newTestTSA(t)
	asked := sha256.Sum256([]byte("real"))
	other := sha256.Sum256([]byte("forged"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forged, err := (&timestamp.Request{
			HashAlgorithm: crypto.SHA256,
			HashedMessage: other[:],
			Certificates:  true,
		}).Marshal()
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(stampResponse(t, cert, key, forged))
	}))
	defer srv.Close()

	_, _, err := Stamp(context.Background(),
		Config{Endpoint: srv.URL, ProviderName: "test"},
		crypto.SHA256, asked[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse on digest mismatch, got %v", err)
	}
}

func TestStamp_LargeBodyRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/timestamp-reply")
		buf := make([]byte, MaxResponseSize+10)
		_, _ = w.Write(buf)
	}))
	defer srv.Close()
	digest := sha256.Sum256([]byte("p"))
	_, _, err := Stamp(context.Background(), Config{Endpoint: srv.URL},
		crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse for oversized body, got %v", err)
	}
}

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		code int
		want error
	}{
		{200, nil},
		{299, nil},
		{500, tsa.ErrUpstreamUnavailable},
		{503, tsa.ErrUpstreamUnavailable},
		{401, tsa.ErrAuthFailed},
		{403, tsa.ErrAuthFailed},
		{407, tsa.ErrAuthFailed},
		{400, tsa.ErrInvalidInput},
		{418, tsa.ErrInvalidResponse},
		{404, tsa.ErrInvalidResponse},
		{100, tsa.ErrUpstreamUnavailable},
	}
	for _, c := range cases {
		err := classifyStatus(c.code, "http://x")
		if c.want == nil {
			if err != nil {
				t.Errorf("status %d: want nil, got %v", c.code, err)
			}
			continue
		}
		if !errors.Is(err, c.want) {
			t.Errorf("status %d: want %v, got %v", c.code, c.want, err)
		}
	}
}

func TestStamp_HTTPStatusCodes(t *testing.T) {
	cases := []struct {
		name string
		code int
		want error
	}{
		{"5xx", http.StatusBadGateway, tsa.ErrUpstreamUnavailable},
		{"401", http.StatusUnauthorized, tsa.ErrAuthFailed},
		{"403", http.StatusForbidden, tsa.ErrAuthFailed},
		{"400", http.StatusBadRequest, tsa.ErrInvalidInput},
		{"418", http.StatusTeapot, tsa.ErrInvalidResponse},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.code)
			}))
			defer srv.Close()
			digest := sha256.Sum256([]byte("p"))
			_, _, err := Stamp(context.Background(),
				Config{Endpoint: srv.URL},
				crypto.SHA256, digest[:])
			if !errors.Is(err, c.want) {
				t.Fatalf("status %d: want %v, got %v", c.code, c.want, err)
			}
		})
	}
}
