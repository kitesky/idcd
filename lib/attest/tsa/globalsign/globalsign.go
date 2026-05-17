// Package globalsign is the RFC3161 TSA adapter for GlobalSign's free
// public timestamp service (http://rfc3161.globalsign.com/advanced).
// Like DigiCert it requires no authentication and speaks plain
// RFC3161 over HTTP POST.
//
// This adapter is wired into tsa.Multi as the backup provider in S2 —
// see docs/prd/18-evidence-and-attestation.md §3.4.
package globalsign

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
	// DefaultEndpoint is GlobalSign's public, unauthenticated TSA URL.
	// (The "advanced" suffix exposes the policy that returns the full
	// certificate chain, which we need for PAdES B-T embedding.)
	DefaultEndpoint = "http://rfc3161.globalsign.com/advanced"

	providerName = "globalsign"

	requestContentType  = "application/timestamp-query"
	responseContentType = "application/timestamp-reply"

	maxResponseSize = 1 << 20
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
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &provider{endpoint: endpoint, client: client}
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
		return nil, time.Time{}, fmt.Errorf("%w: %v", tsa.ErrInvalidResponse, err)
	}
	if len(ts.RawToken) == 0 {
		return nil, time.Time{}, fmt.Errorf("%w: empty TimeStampToken", tsa.ErrInvalidResponse)
	}
	if !bytes.Equal(ts.HashedMessage, digest) {
		return nil, time.Time{}, fmt.Errorf("%w: digest mismatch (sent %x, got %x)",
			tsa.ErrInvalidResponse, digest[:min(8, len(digest))], ts.HashedMessage[:min(8, len(ts.HashedMessage))])
	}

	return ts.RawToken, ts.Time, nil
}

var _ tsa.Provider = (*provider)(nil)
