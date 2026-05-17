package service

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
)

// ---------------------------------------------------------------------------
// shared TSA test server (used by orchestrator_test.go and other tests)
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
