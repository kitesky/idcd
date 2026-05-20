package ca

import "context"

// ResellerCA is implemented by paid-CA adapters that speak a vendor's
// reseller REST API (DigiCert, Sectigo, GoGetSSL, …). Interfaces are
// defined in S1 so cert-worker can be written against them now; concrete
// implementations land in S3 alongside the billing integration.
//
// Unlike AcmeCA, the reseller flow cannot be collapsed into one call:
//
//  1. CreateOrder  – submit domains + CSR, receive an order ref and a
//                    DCV instruction (TXT record / HTTP file / email).
//  2. SubmitOrgInfo – OV/EV only; uploads company details after the user
//                    completes the in-app form.
//  3. PollOrder    – worker polls until the vendor reports issued/failed.
//  4. FetchCert    – pull the issued leaf + chain.
//  5. Revoke       – revoke later.
type ResellerCA interface {
	CA

	// CreateOrder submits an order to the reseller. The returned
	// orderRef is the vendor's opaque identifier that worker stores
	// in cert.orders.reseller_order_ref. dcv is the validation
	// instruction to surface to the end user.
	CreateOrder(ctx context.Context, req ResellerOrderRequest) (orderRef string, dcv DCVInstruction, err error)

	// SubmitOrgInfo uploads the organisation details required for
	// OV/EV. Worker calls this once the user has filled the form
	// and uploaded their business licence. No-op for DV orders;
	// adapters should return ErrInvalidInput if called on a DV
	// order to flag worker bugs early.
	SubmitOrgInfo(ctx context.Context, orderRef string, info OrganizationInfo) error

	// PollOrder returns the current state of an order. Worker calls
	// this on a backoff schedule until status.Issued is true or
	// status.State is "failed".
	PollOrder(ctx context.Context, orderRef string) (ResellerOrderStatus, error)

	// FetchCert downloads the issued certificate. Only valid once
	// PollOrder reported issued; otherwise returns ErrInvalidInput.
	FetchCert(ctx context.Context, orderRef string) (leafPEM, chainPEM []byte, err error)

	// Revoke revokes a previously issued certificate by its order
	// reference (reseller APIs index by order, not cert serial).
	Revoke(ctx context.Context, orderRef string, reason RevokeReason) error
}

// ResellerOrderRequest is the input to ResellerCA.CreateOrder.
type ResellerOrderRequest struct {
	// Domains is the SAN list (ASCII / Punycode).
	Domains []string

	// CSR is the PEM-encoded PKCS#10 request. Required for all
	// reseller CAs.
	CSR []byte

	// OrgID is the reseller-side organisation identifier. Required
	// for OV / EV, ignored for DV.
	OrgID int64

	// Validity is the requested cert lifetime in days. Reseller CAs
	// today commonly issue 397-day certs.
	Validity int
}

// DCVInstruction is returned alongside the order ref and tells the user
// what they need to put where to prove domain control.
type DCVInstruction struct {
	// Method is the validation channel: dns-01, http-01 or email.
	Method ChallengeType

	// Token is the random per-domain token issued by the CA.
	Token string

	// Value is what to publish (TXT record value / HTTP file body).
	// Empty for email method.
	Value string

	// Email is the address the CA sent the validation email to.
	// Only set when Method == ChallengeEmail.
	Email string
}

// OrganizationInfo is the OV / EV organisation packet. Persisted by
// worker in cert.organizations.
type OrganizationInfo struct {
	LegalName     string
	Country       string
	State         string
	City          string
	Address       string
	Phone         string
	BizLicenseURL string
}

// ResellerOrderStatus is the snapshot returned by PollOrder.
type ResellerOrderStatus struct {
	// State is one of "pending" | "awaiting_org" | "issued" | "failed".
	// Worker maps this onto its own state machine.
	State string

	// Detail is a human-readable explanation, surfaced verbatim to
	// the user on failure.
	Detail string

	// Issued is the convenience flag (State == "issued").
	Issued bool
}
