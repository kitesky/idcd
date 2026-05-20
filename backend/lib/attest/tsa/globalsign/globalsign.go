// Package globalsign is the RFC3161 TSA adapter for GlobalSign's free
// public timestamp service (http://rfc3161.globalsign.com/advanced).
// Like DigiCert it requires no authentication and speaks plain
// RFC3161 over HTTP POST.
//
// This adapter is wired into tsa.Multi as the backup provider in S2 —
// see docs/prd/18-evidence-and-attestation.md §3.4.
//
// Implementation note: the protocol logic lives in
// tsa/internal/rfc3161client; this file is configuration + naming only.
package globalsign

import (
	"context"
	"crypto"
	"net/http"
	"time"

	"github.com/kite365/idcd/lib/attest/tsa"
	"github.com/kite365/idcd/lib/attest/tsa/internal/rfc3161client"
)

const (
	// DefaultEndpoint is GlobalSign's public, unauthenticated TSA URL.
	// (The "advanced" suffix exposes the policy that returns the full
	// certificate chain, which we need for PAdES B-T embedding.)
	DefaultEndpoint = "http://rfc3161.globalsign.com/advanced"

	providerName = "globalsign"
)

// Config mirrors digicert.Config. See that package for field docs;
// behaviour is identical other than the default endpoint.
type Config struct {
	Endpoint   string
	HTTPClient *http.Client
}

// New returns a tsa.Provider posting RFC3161 timestamp queries to
// GlobalSign. Stateless and safe for concurrent use.
func New(cfg Config) tsa.Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &provider{endpoint: endpoint, client: cfg.HTTPClient}
}

type provider struct {
	endpoint string
	client   *http.Client
}

func (p *provider) Name() string { return providerName }

// Stamp delegates to the shared rfc3161client.Stamp. See
// digicert.provider.Stamp for the rationale.
func (p *provider) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	return rfc3161client.Stamp(ctx, rfc3161client.Config{
		Endpoint:     p.endpoint,
		HTTPClient:   p.client,
		ProviderName: providerName,
	}, hashAlg, digest)
}

var _ tsa.Provider = (*provider)(nil)
