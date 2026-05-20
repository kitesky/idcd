package freetsa

import (
	"context"
	"crypto"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_DefaultsEndpoint(t *testing.T) {
	p := New(Config{}).(*provider)
	if p.endpoint != DefaultEndpoint {
		t.Fatalf("expected DefaultEndpoint, got %q", p.endpoint)
	}
	if p.Name() != "freetsa" {
		t.Fatalf("provider name: %s", p.Name())
	}
}

func TestNew_HonorsCustomEndpoint(t *testing.T) {
	p := New(Config{Endpoint: "https://custom.tsa/example"}).(*provider)
	if p.endpoint != "https://custom.tsa/example" {
		t.Fatalf("endpoint not honored: %s", p.endpoint)
	}
}

// TestStamp_RejectsBadServer verifies Stamp wires the rfc3161client through
// — a server that returns garbage should produce an error. This is not a
// full RFC3161 round-trip test (the shared client package already covers
// that); the goal here is purely "is the adapter calling client.Stamp
// with the right config".
func TestStamp_RejectsBadServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	p := New(Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("hello"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if err == nil {
		t.Fatal("expected error on 502 response")
	}
}
