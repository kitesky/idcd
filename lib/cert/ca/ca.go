// Package ca defines the CA adapter layer used by the cert platform.
//
// Three interface tiers:
//
//   - CA       — metadata interface every implementation satisfies.
//   - AcmeCA   — free CAs that speak ACME RFC 8555 (Let's Encrypt, ZeroSSL,
//                Buypass, GTS). Implemented in subpackages under ca/.
//   - ResellerCA — paid CAs reached via reseller REST (DigiCert, Sectigo,
//                  GoGetSSL …). Interface only in S1; implementations land S3.
//
// cert-worker dispatches by the order's tier field to either an AcmeExecutor
// or ResellerExecutor; the higher-level state machine stays unified.
//
// All implementations MUST translate transport / protocol errors into one of
// the sentinel errors declared here so the worker's retry policy and the
// admin SLA dashboard remain CA-agnostic.
package ca

import "errors"

// Tier identifies the certificate product class.
type Tier string

const (
	TierFreeDV Tier = "free-dv"
	TierPaidDV Tier = "paid-dv"
	TierPaidOV Tier = "paid-ov"
	TierPaidEV Tier = "paid-ev"
)

// ChallengeType identifies how domain control is validated.
//
// ChallengeEmail is reserved for S3 reseller OV/EV flows; ACME CAs in S1
// only return dns-01 / http-01.
type ChallengeType string

const (
	ChallengeDNS01  ChallengeType = "dns-01"
	ChallengeHTTP01 ChallengeType = "http-01"
	ChallengeEmail  ChallengeType = "email"
)

// CA is the metadata interface satisfied by every CA implementation,
// free or paid. It carries no I/O; the protocol-specific methods live on
// the AcmeCA / ResellerCA sub-interfaces so the worker can type-assert.
type CA interface {
	// Name returns a stable identifier (e.g. "lets-encrypt", "zerossl",
	// "digicert"). Used for logs, metrics and the reseller_channel column.
	Name() string

	// Tier returns the product class this CA issues.
	Tier() Tier

	// SupportsWildcard reports whether the CA can issue *.example.com
	// SAN entries. Some free CAs historically did not.
	SupportsWildcard() bool

	// ValidityDays is the nominal lifetime of certificates from this CA.
	// Used by the renewal scheduler to compute T-30d job timing.
	ValidityDays() int

	// SupportedChallenges lists DCV methods this CA accepts. Worker picks
	// the first one this adapter supports that the user has DNS access for.
	SupportedChallenges() []ChallengeType
}

// Sentinel errors. Implementations must wrap or return one of these for
// every non-success outcome the worker is expected to act on. Anything
// else is treated as a transient implementation bug and routed to the
// 24h SLA queue (D12).
var (
	// ErrCAQuotaExceeded is returned when the CA's rate limit, weekly
	// duplicate certificate cap or account-level issuance budget is hit.
	// Worker schedules retry with exponential backoff plus jitter.
	ErrCAQuotaExceeded = errors.New("ca: rate limit / quota exceeded")

	// ErrAuthzInvalid covers authorization failures that are not the
	// CAA case below: the CA could not validate the challenge (DNS not
	// propagated, HTTP token mismatch, etc.). Worker surfaces to the
	// user and waits for them to fix the underlying record.
	ErrAuthzInvalid = errors.New("ca: authorization invalid (caa/dns failed)")

	// ErrCAATooStrict is returned when the domain's CAA record forbids
	// this CA. Worker should not retry; the user must edit CAA.
	ErrCAATooStrict = errors.New("ca: caa policy forbids this ca")

	// ErrAccountInvalid means the supplied account key is unknown or
	// deactivated upstream. Worker rotates / re-registers the account.
	ErrAccountInvalid = errors.New("ca: acme account invalid or unauthorized")

	// ErrNetwork covers nonce errors, 5xx and transport-level failures.
	// Worker retries with short backoff.
	ErrNetwork = errors.New("ca: network / upstream unavailable")

	// ErrInvalidInput is for caller mistakes (empty domain list, missing
	// account key, malformed CSR). Worker does not retry.
	ErrInvalidInput = errors.New("ca: invalid input")
)
