// Package digicert is the RFC3161 TSA adapter for DigiCert's public
// timestamp service (http://timestamp.digicert.com). The service is
// free, requires no authentication, and accepts ASN.1 timestamp queries
// over HTTP POST per RFC3161 §3.4.
//
// This adapter is one of the providers wired into tsa.Multi in S2 —
// see docs/prd/18-evidence-and-attestation.md §3.4.
//
// Implementation note: the protocol logic lives in
// tsa/internal/rfc3161client; this file is configuration + naming only.
package digicert

import (
	"context"
	"crypto"
	"net/http"
	"time"

	"github.com/kite365/idcd/lib/attest/tsa"
	"github.com/kite365/idcd/lib/attest/tsa/internal/rfc3161client"
)

const (
	// DefaultEndpoint is DigiCert's public, unauthenticated TSA URL.
	// HTTP is intentional — the TSA response is a signed ASN.1
	// structure verified by certificate, not by TLS.
	DefaultEndpoint = "http://timestamp.digicert.com"

	// providerName is the value used in attestation_record.external_id
	// and tsa_response.provider. Lowercase, stable.
	providerName = "digicert"
)

// Config holds construction parameters for the DigiCert adapter. All
// fields are optional — the zero value yields a working client against
// the public DigiCert endpoint.
type Config struct {
	// Endpoint overrides DefaultEndpoint when non-empty. Useful for
	// tests (httptest.Server) and DigiCert enterprise customers with a
	// dedicated TSA URL.
	Endpoint string

	// HTTPClient overrides http.DefaultClient when non-nil. Inject a
	// client with custom timeouts, retries, or proxy. Note: tsa.Multi
	// already enforces a per-call deadline via context, so a per-client
	// timeout is usually unnecessary.
	HTTPClient *http.Client
}

// New returns a tsa.Provider that posts RFC3161 timestamp queries to
// DigiCert. The returned value is stateless and safe for concurrent
// use.
func New(cfg Config) tsa.Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &provider{
		endpoint: endpoint,
		client:   cfg.HTTPClient, // nil → http.DefaultClient inside rfc3161client.Stamp
	}
}

type provider struct {
	endpoint string
	client   *http.Client
}

func (p *provider) Name() string { return providerName }

// Stamp delegates to the shared rfc3161client.Stamp. Per-provider
// behaviour is purely the endpoint + provider name; protocol semantics
// (HTTP classification, body length cap, digest mismatch check) live in
// the shared helper so any future fix lands in both DigiCert and
// GlobalSign in one place.
func (p *provider) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	return rfc3161client.Stamp(ctx, rfc3161client.Config{
		Endpoint:     p.endpoint,
		HTTPClient:   p.client,
		ProviderName: providerName,
	}, hashAlg, digest)
}

// Ensure interface satisfaction at compile time.
var _ tsa.Provider = (*provider)(nil)
