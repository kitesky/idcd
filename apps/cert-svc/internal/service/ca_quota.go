package service

import "context"

// QuotaUsage describes how much of a CA's free issuance budget has been
// spent in the recent window. Both fields are 0..1 ratios; values >1 are
// clamped to 1 by the producer.
type QuotaUsage struct {
	// PerRegisteredDomain peaks across all (CA, registered_domain)
	// pairs in the last 7 days. For LE this is the 50 certs / RD /
	// week limit.
	PerRegisteredDomain float64

	// PerAccount3h is the newOrder count in the rolling 3 hours
	// divided by the CA's per-account ceiling (300 for LE).
	PerAccount3h float64
}

// QuotaChecker computes recent issuance volume against a CA's known
// rate limits. Returns (Usage, nil) on success; the Router never
// short-circuits on errors — it logs and proceeds with the default CA.
type QuotaChecker interface {
	// Usage queries the underlying store for a CA. Implementations
	// must respect ctx; Router calls this on the hot issuance path.
	Usage(ctx context.Context, caName string) (QuotaUsage, error)
}

// SwitchThreshold is the usage ratio at which Router falls over from
// the default CA to the secondary. Hardcoded per PRD §3.2 ("70%").
const SwitchThreshold = 0.70

// nopQuotaChecker is the zero-value fallback — every Usage call
// returns 0,0 so Router always picks the default.
type nopQuotaChecker struct{}

func (nopQuotaChecker) Usage(_ context.Context, _ string) (QuotaUsage, error) {
	return QuotaUsage{}, nil
}
