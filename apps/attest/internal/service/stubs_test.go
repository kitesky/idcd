package service

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
)

// ---------------------------------------------------------------------------
// stubs.go behaviour
// ---------------------------------------------------------------------------

func TestRenderPDF_HasMagicHeader(t *testing.T) {
	out, err := renderPDF(&Order{ID: "vo_x"}, nil, nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	if len(out) < 5 || string(out[:5]) != "%PDF-" {
		t.Fatalf("renderPDF output missing %%PDF- header: %q", out[:minInt(5, len(out))])
	}
}

func TestCrossValidate_AllOK(t *testing.T) {
	obs := []observation{
		{NodeID: "a", OK: true},
		{NodeID: "b", OK: true},
		{NodeID: "c", OK: true},
	}
	nodes, pct := crossValidate(context.Background(), obs)
	if len(nodes) != 3 {
		t.Fatalf("nodes = %v, want 3 entries", nodes)
	}
	if pct != 100 {
		t.Fatalf("consistency = %v, want 100", pct)
	}
}

func TestCrossValidate_PartialFailures(t *testing.T) {
	obs := []observation{
		{NodeID: "a", OK: true},
		{NodeID: "b", OK: false},
	}
	_, pct := crossValidate(context.Background(), obs)
	if pct != 50 {
		t.Fatalf("consistency = %v, want 50", pct)
	}
}

func TestCrossValidate_Empty(t *testing.T) {
	nodes, pct := crossValidate(context.Background(), nil)
	if nodes != nil {
		t.Fatalf("nodes = %v, want nil", nodes)
	}
	if pct != 0 {
		t.Fatalf("consistency = %v, want 0", pct)
	}
}

func TestFetchObservations_NilOrderRejected(t *testing.T) {
	if _, err := fetchObservations(context.Background(), nil); err == nil {
		t.Fatalf("expected error for nil order")
	}
}

func TestFetchObservations_HappyShape(t *testing.T) {
	obs, err := fetchObservations(context.Background(), &Order{ID: "vo_x"})
	if err != nil {
		t.Fatalf("fetchObservations: %v", err)
	}
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}
}

// ---------------------------------------------------------------------------
// localArchiver
// ---------------------------------------------------------------------------

func TestLocalArchiver_WritesFileAndEtag(t *testing.T) {
	dir := t.TempDir()
	a := NewLocalArchiver(dir)

	pdf := []byte("%PDF-1.4 fake")
	url, etag, err := a.Archive(context.Background(), "vr_test.pdf", pdf)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if !strings.HasPrefix(url, "file://") {
		t.Fatalf("url = %q, expected file:// prefix", url)
	}
	if len(etag) != 16 {
		t.Fatalf("etag length = %d, want 16", len(etag))
	}
	data, err := os.ReadFile(filepath.Join(dir, "vr_test.pdf"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(pdf) {
		t.Fatalf("written bytes mismatch")
	}
}

func TestLocalArchiver_EmptyDirRejected(t *testing.T) {
	a := NewLocalArchiver("")
	if _, _, err := a.Archive(context.Background(), "k", []byte("x")); err == nil {
		t.Fatalf("expected error for empty dir")
	}
}

// ---------------------------------------------------------------------------
// shared TSA test server (used by orchestrator_test.go too)
// ---------------------------------------------------------------------------

type tsaTestServer struct {
	srv *httptest.Server
}

func (s *tsaTestServer) URL() string { return s.srv.URL }

// startTSAServer spawns an httptest.Server that speaks a minimal
// RFC3161 dialect, signing every request with the supplied key + cert.
// Reused from the pdfsign test fixtures pattern so the orchestrator's
// call into pdfsign exercises a real PAdES-T flow end-to-end.
func startTSAServer(t *testing.T, key *rsa.PrivateKey, cert *x509.Certificate) *tsaTestServer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		req, err := timestamp.ParseRequest(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
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
		token, err := ts.CreateResponseWithOpts(cert, key, crypto.SHA256)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(token)
	}))
	t.Cleanup(srv.Close)
	return &tsaTestServer{srv: srv}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
