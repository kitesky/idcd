package ca

import (
	"context"
	"crypto"
	"time"
)

// AcmeCA is the interface implemented by every ACME-protocol CA adapter
// (Let's Encrypt, ZeroSSL, Buypass, GTS).
//
// The interface intentionally collapses the ACME state machine (newOrder
// → authorize → challenge → finalize → download) into a single
// RequestCertificate call. The lego library already implements that flow
// safely; exposing each step would force every adapter to reinvent it.
// cert-worker keeps its own coarse-grained order state machine and treats
// a single RequestCertificate invocation as one atomic step.
type AcmeCA interface {
	CA

	// RequestCertificate runs the full ACME flow for one order and
	// returns the issued leaf + chain. ctx controls the whole flow
	// (including DNS propagation waits). On error the returned value
	// is the zero CertificateResult and err is one of the package
	// sentinel errors.
	RequestCertificate(ctx context.Context, req CertificateRequest) (CertificateResult, error)

	// Revoke revokes a previously issued certificate. accountKey must
	// be the account key the certificate was issued under (or the key
	// of the certificate itself; both are accepted by ACME RFC 8555
	// §7.6, but adapters should prefer the account key).
	Revoke(ctx context.Context, cert []byte, reason RevokeReason, accountKey crypto.Signer) error
}

// CertificateRequest is the input to AcmeCA.RequestCertificate.
//
// Either CSR or PrivateKey must be supplied. When CSR is non-nil the
// adapter uses lego's ObtainForCSR path and PrivateKey is ignored; the
// caller is responsible for having generated the CSR with the key it
// wants to keep. When CSR is nil, PrivateKey is used as-is to obtain
// the certificate.
type CertificateRequest struct {
	// AccountKey is the long-lived ACME account key. cert-worker
	// loads it from vault per workspace and hands it in here; the
	// adapter never persists it.
	AccountKey crypto.Signer

	// AccountEmail is used during account registration (first call
	// for a fresh AccountKey) and otherwise ignored.
	AccountEmail string

	// Domains is the SAN list. Entries must already be ASCII
	// (Punycode-encoded for IDN); the adapter does no normalisation.
	// The first entry becomes the CommonName.
	Domains []string

	// CSR is an optional PEM-encoded PKCS#10 request. If supplied,
	// it is used verbatim and the Domains list must match the
	// SANs already in the CSR.
	CSR []byte

	// PrivateKey is the certificate key; used only when CSR is nil.
	PrivateKey crypto.Signer

	// DNS is the DNS-01 solver. Required: this S1 path only supports
	// dns-01 because that's the only challenge that works for
	// wildcard certificates and for non-public origins.
	DNS DnsSolver

	// Timeout caps the whole RequestCertificate call. If zero the
	// adapter applies its own default (typically 5 minutes).
	Timeout time.Duration
}

// CertificateResult is what AcmeCA.RequestCertificate returns on success.
type CertificateResult struct {
	// LeafPEM is the issued end-entity certificate.
	LeafPEM []byte

	// ChainPEM is the issuer chain WITHOUT the leaf, ready to be
	// concatenated after LeafPEM to form a fullchain.pem.
	ChainPEM []byte

	// IssuerURL is the certUrl returned by the CA; useful for audit
	// and for follow-up Revoke calls.
	IssuerURL string

	// Serial is the leaf certificate serial number in lowercase hex.
	Serial string

	NotBefore time.Time
	NotAfter  time.Time
}

// DnsSolver is implemented by cert-worker and injected per request.
// Keeping the interface in this package (rather than in lib/cert/dns)
// avoids a circular import between ca and dns adapters.
type DnsSolver interface {
	// Present writes a TXT record with name fqdn (already trailing-dot
	// or not — adapter normalises) and value `value`. The adapter
	// computes fqdn as "_acme-challenge.<domain>" and value as the
	// base64(SHA-256(keyAuthorization)) per RFC 8555 §8.4.
	Present(ctx context.Context, fqdn, value string) error

	// CleanUp removes the TXT record set by Present. Called whether
	// the challenge succeeded or failed.
	CleanUp(ctx context.Context, fqdn, value string) error

	// Timeout is the maximum the adapter should wait for global DNS
	// propagation before asking the CA to validate. Provider-specific
	// because Cloudflare propagates in seconds, Aliyun in minutes.
	Timeout() time.Duration
}

// RevokeReason mirrors the RFC 5280 §5.3.1 CRL reason codes we expose to
// callers. Only the values we actually use are listed; anything else
// should go through RevokeUnspecified.
type RevokeReason int

const (
	RevokeUnspecified          RevokeReason = 0
	RevokeKeyCompromise        RevokeReason = 1
	RevokeCessationOfOperation RevokeReason = 5
	RevokeCertificateHold      RevokeReason = 6
)
