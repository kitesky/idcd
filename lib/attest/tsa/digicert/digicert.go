// Package digicert is the RFC3161 TSA adapter for DigiCert's public
// timestamp service (http://timestamp.digicert.com). The service is
// free, requires no authentication, and accepts ASN.1 timestamp queries
// over HTTP POST per RFC3161 §3.4.
//
// This adapter is one of the providers wired into tsa.Multi in S2 —
// see docs/prd/18-evidence-and-attestation.md §3.4.
package digicert

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/digitorus/timestamp"

	"github.com/kite365/idcd/lib/attest/tsa"
)

const (
	// DefaultEndpoint is DigiCert's public, unauthenticated TSA URL.
	// HTTP is intentional — the TSA response is a signed ASN.1
	// structure verified by certificate, not by TLS.
	DefaultEndpoint = "http://timestamp.digicert.com"

	// providerName is the value used in attestation_record.external_id
	// and tsa_response.provider. Lowercase, stable.
	providerName = "digicert"

	// requestContentType / responseContentType are mandated by RFC3161.
	requestContentType  = "application/timestamp-query"
	responseContentType = "application/timestamp-reply"

	// maxResponseSize bounds the body we will read from a TSA. A real
	// RFC3161 response is a few KiB; 1 MiB is paranoid headroom.
	maxResponseSize = 1 << 20
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
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &provider{
		endpoint: endpoint,
		client:   client,
	}
}

type provider struct {
	endpoint string
	client   *http.Client
}

func (p *provider) Name() string { return providerName }

// Stamp implements tsa.Provider.
func (p *provider) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	if err := tsa.ValidateDigest(hashAlg, digest); err != nil {
		return nil, time.Time{}, err
	}

	tsq, err := (&timestamp.Request{
		HashAlgorithm: hashAlg,
		HashedMessage: digest,
		Certificates:  true,
	}).Marshal()
	if err != nil {
		// A malformed timestamp.Request only happens on unsupported hash
		// algorithms; ValidateDigest already filtered most of those.
		return nil, time.Time{}, fmt.Errorf("%w: marshal TSQ: %v", tsa.ErrInvalidInput, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(tsq))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%w: build request: %v", tsa.ErrUpstreamUnavailable, err)
	}
	req.Header.Set("Content-Type", requestContentType)
	req.Header.Set("Accept", responseContentType)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%w: POST %s: %v", tsa.ErrUpstreamUnavailable, p.endpoint, err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if readErr != nil {
		return nil, time.Time{}, fmt.Errorf("%w: read body: %v", tsa.ErrUpstreamUnavailable, readErr)
	}
	if len(body) > maxResponseSize {
		return nil, time.Time{}, fmt.Errorf("%w: TSA response exceeds %d bytes", tsa.ErrInvalidResponse, maxResponseSize)
	}

	switch {
	case resp.StatusCode >= 500:
		return nil, time.Time{}, fmt.Errorf("%w: HTTP %d from %s", tsa.ErrUpstreamUnavailable, resp.StatusCode, p.endpoint)
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusProxyAuthRequired:
		return nil, time.Time{}, fmt.Errorf("%w: HTTP %d from %s", tsa.ErrAuthFailed, resp.StatusCode, p.endpoint)
	case resp.StatusCode == http.StatusBadRequest:
		return nil, time.Time{}, fmt.Errorf("%w: HTTP 400 from %s", tsa.ErrInvalidInput, p.endpoint)
	case resp.StatusCode >= 400:
		return nil, time.Time{}, fmt.Errorf("%w: HTTP %d from %s", tsa.ErrInvalidResponse, resp.StatusCode, p.endpoint)
	}

	ts, err := timestamp.ParseResponse(body)
	if err != nil {
		// ParseResponse returns a non-nil error both for transport
		// glitches (truncated body) and for granted-but-malformed
		// tokens. From the caller's standpoint these are all
		// "response unusable" — fall over.
		return nil, time.Time{}, fmt.Errorf("%w: %v", tsa.ErrInvalidResponse, err)
	}
	if len(ts.RawToken) == 0 {
		return nil, time.Time{}, fmt.Errorf("%w: empty TimeStampToken", tsa.ErrInvalidResponse)
	}

	// Defence in depth: ParseResponse already calls Parse which
	// validates the token signature; we additionally sanity-check that
	// the response is about the digest we sent (defeats lazy mocks and
	// catches a TSA that mixed responses across concurrent requests).
	if !bytes.Equal(ts.HashedMessage, digest) {
		return nil, time.Time{}, fmt.Errorf("%w: digest mismatch (sent %x, got %x)",
			tsa.ErrInvalidResponse, digest[:min(8, len(digest))], ts.HashedMessage[:min(8, len(ts.HashedMessage))])
	}

	return ts.RawToken, ts.Time, nil
}

// Ensure interface satisfaction at compile time.
var _ tsa.Provider = (*provider)(nil)
