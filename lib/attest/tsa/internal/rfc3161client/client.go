// Package rfc3161client centralises the RFC3161 over-HTTP boilerplate
// shared by every TSA provider adapter in lib/attest/tsa/* (digicert,
// globalsign, and any future addition).
//
// Before this package existed, digicert/digicert.go and
// globalsign/globalsign.go each carried ~70 lines of effectively
// identical code: marshal a timestamp.Request, POST it with the
// mandated Content-Type / Accept headers, read a length-capped body,
// classify the HTTP status code into a tsa.Err* sentinel, parse the
// response with digitorus/timestamp, and defence-in-depth compare the
// returned digest. The only per-provider differences were
//
//   - the endpoint URL,
//   - the provider name reported up the stack,
//   - (optionally) an http.Client override.
//
// Keeping two copies in lock-step is the kind of busywork that lets a
// security-relevant bug fix in one file silently miss the other. Stamp()
// in this package now owns the protocol; provider packages just supply
// configuration.
//
// This is intentionally an internal package — third-party TSA adapters
// would need to live under lib/attest/tsa/ to use it. That matches the
// architecture: TSA providers are part of the verdict pipeline and ship
// in this repo.
package rfc3161client

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
	// requestContentType / responseContentType are mandated by RFC3161 §3.4.
	// Servers that ignore Content-Type still parse the body fine, but
	// reverse proxies / WAFs in front of commercial TSAs often gate on
	// them.
	requestContentType  = "application/timestamp-query"
	responseContentType = "application/timestamp-reply"

	// MaxResponseSize bounds the body the client will read. A real
	// RFC3161 response is a few KiB; 1 MiB is paranoid headroom and
	// protects against accidental HTML error pages from a misconfigured
	// proxy bombing memory.
	MaxResponseSize = 1 << 20
)

// Config bundles the per-call parameters Stamp needs. Provider packages
// build one of these from their own Config and forward it.
type Config struct {
	// Endpoint is the absolute URL of the TSA service. Required.
	Endpoint string

	// HTTPClient is the client used for the POST. nil falls back to
	// http.DefaultClient. Provider packages SHOULD let callers override
	// this for testability and corp-proxy / mTLS deployments.
	HTTPClient *http.Client

	// ProviderName is only used for embedding in error messages so
	// fall-over logs can identify which adapter produced a given
	// failure. It is NOT used in the wire protocol.
	ProviderName string
}

// Stamp executes one RFC3161 timestamp request against cfg.Endpoint.
//
// Behaviour mirrors what each provider package implemented separately
// before consolidation:
//
//   - tsa.ValidateDigest first — wrong digest length / unavailable hash
//     algorithm is a tsa.ErrInvalidInput, no HTTP call made.
//   - HTTP 5xx / network error              → tsa.ErrUpstreamUnavailable
//   - HTTP 401 / 403 / 407                  → tsa.ErrAuthFailed
//   - HTTP 400                              → tsa.ErrInvalidInput
//   - HTTP 4xx (other)                      → tsa.ErrInvalidResponse
//   - Body > MaxResponseSize                → tsa.ErrInvalidResponse
//   - timestamp.ParseResponse failure        → tsa.ErrInvalidResponse
//   - empty RawToken                        → tsa.ErrInvalidResponse
//   - digest mismatch (defence in depth)    → tsa.ErrInvalidResponse
//
// All errors wrap a tsa.Err* sentinel via fmt.Errorf("%w: ...", ...),
// matching the contract tsa.Multi inspects with errors.Is.
func Stamp(ctx context.Context, cfg Config, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	if err := tsa.ValidateDigest(hashAlg, digest); err != nil {
		return nil, time.Time{}, err
	}

	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	tsq, err := (&timestamp.Request{
		HashAlgorithm: hashAlg,
		HashedMessage: digest,
		Certificates:  true,
	}).Marshal()
	if err != nil {
		// timestamp.Request.Marshal only fails on unsupported hash; the
		// ValidateDigest call above already filters most of those.
		return nil, time.Time{}, fmt.Errorf("%w: marshal TSQ: %v", tsa.ErrInvalidInput, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(tsq))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%w: build request: %v", tsa.ErrUpstreamUnavailable, err)
	}
	req.Header.Set("Content-Type", requestContentType)
	req.Header.Set("Accept", responseContentType)

	resp, err := client.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("%w: POST %s: %v", tsa.ErrUpstreamUnavailable, cfg.Endpoint, err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxResponseSize+1))
	if readErr != nil {
		return nil, time.Time{}, fmt.Errorf("%w: read body: %v", tsa.ErrUpstreamUnavailable, readErr)
	}
	if len(body) > MaxResponseSize {
		return nil, time.Time{}, fmt.Errorf("%w: TSA response exceeds %d bytes", tsa.ErrInvalidResponse, MaxResponseSize)
	}

	if err := classifyStatus(resp.StatusCode, cfg.Endpoint); err != nil {
		return nil, time.Time{}, err
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

	// Defence in depth: ParseResponse already validates the token
	// signature via Parse(); we additionally check the response is
	// actually about the digest we sent. Catches both a lazy mock and
	// a TSA that mixed responses across concurrent requests.
	if !bytes.Equal(ts.HashedMessage, digest) {
		return nil, time.Time{}, fmt.Errorf("%w: digest mismatch (sent %x, got %x)",
			tsa.ErrInvalidResponse,
			digest[:minInt(8, len(digest))],
			ts.HashedMessage[:minInt(8, len(ts.HashedMessage))])
	}

	return ts.RawToken, ts.Time, nil
}

// classifyStatus turns an HTTP status code into the matching tsa.Err*
// sentinel. Exposed as a separate function so the test suite can
// exhaustively probe boundaries without spinning up a real httptest
// server for each code.
func classifyStatus(code int, endpoint string) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code >= 500:
		return fmt.Errorf("%w: HTTP %d from %s", tsa.ErrUpstreamUnavailable, code, endpoint)
	case code == http.StatusUnauthorized,
		code == http.StatusForbidden,
		code == http.StatusProxyAuthRequired:
		return fmt.Errorf("%w: HTTP %d from %s", tsa.ErrAuthFailed, code, endpoint)
	case code == http.StatusBadRequest:
		return fmt.Errorf("%w: HTTP 400 from %s", tsa.ErrInvalidInput, endpoint)
	case code >= 400:
		return fmt.Errorf("%w: HTTP %d from %s", tsa.ErrInvalidResponse, code, endpoint)
	default:
		// 1xx / 3xx — http.Client follows 3xx by default, so we should
		// never see them here. Be conservative and treat as transient.
		return fmt.Errorf("%w: unexpected HTTP %d from %s", tsa.ErrUpstreamUnavailable, code, endpoint)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
