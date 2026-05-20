package service

import (
	"context"
	"fmt"
	"time"
)

// CAQuotaCeilings is the set of rate-limit budgets a CA accepts before
// the upstream returns rateLimited. Values come from the CA's published
// docs and apply per platform-wide account (we do not run per-tenant
// accounts at the CA level).
type CAQuotaCeilings struct {
	// NewOrdersPer3h is the rolling 3-hour newOrder cap (LE: 300).
	NewOrdersPer3h int
	// CertsPerRegisteredDomainPerWeek is the per-RD weekly issuance
	// cap (LE: 50).
	CertsPerRegisteredDomainPerWeek int
}

// DefaultCeilings reflects the rate limits each free CA publishes. ZeroSSL
// and Buypass do not publish hard newOrder/RD caps for ACME accounts, so
// we leave both fields zero (their Usage rows always read 0; only the
// default LE row drives fallover).
var DefaultCeilings = map[string]CAQuotaCeilings{
	"lets-encrypt": {NewOrdersPer3h: 300, CertsPerRegisteredDomainPerWeek: 50},
}

// OrdersCounter is satisfied by *repo.OrdersRepo. Kept narrow so the
// quota checker can be tested without a full Repos wiring.
type OrdersCounter interface {
	CountByCASince(ctx context.Context, caName string, since time.Time) (int, error)
}

// CertsDomainPeaker is satisfied by *repo.CertsRepo. Used for the
// per-registered-domain ceiling probe.
type CertsDomainPeaker interface {
	MaxCertsPerRegisteredDomainSince(ctx context.Context, issuer string, since time.Time) (int, error)
}

// RepoQuotaChecker computes CA usage from the cert.orders and cert.certs
// tables. now() is injectable so tests can pin the rolling window.
type RepoQuotaChecker struct {
	orders   OrdersCounter
	certs    CertsDomainPeaker
	ceilings map[string]CAQuotaCeilings
	now      func() time.Time
}

// NewRepoQuotaChecker wires the Router to live DB counts. Pass nil for
// ceilings to use DefaultCeilings; pass a custom map to override on a
// per-CA basis.
func NewRepoQuotaChecker(orders OrdersCounter, certs CertsDomainPeaker, ceilings map[string]CAQuotaCeilings) *RepoQuotaChecker {
	if ceilings == nil {
		ceilings = DefaultCeilings
	}
	return &RepoQuotaChecker{
		orders:   orders,
		certs:    certs,
		ceilings: ceilings,
		now:      time.Now,
	}
}

// Usage implements QuotaChecker. CAs without configured ceilings always
// report zero usage — Router then keeps the default CA, which is the
// correct behaviour when we have nothing to compare against.
func (q *RepoQuotaChecker) Usage(ctx context.Context, caName string) (QuotaUsage, error) {
	ceil, ok := q.ceilings[caName]
	if !ok || (ceil.NewOrdersPer3h == 0 && ceil.CertsPerRegisteredDomainPerWeek == 0) {
		return QuotaUsage{}, nil
	}

	now := q.now().UTC()
	out := QuotaUsage{}

	if ceil.NewOrdersPer3h > 0 {
		n, err := q.orders.CountByCASince(ctx, caName, now.Add(-3*time.Hour))
		if err != nil {
			return QuotaUsage{}, fmt.Errorf("orders count: %w", err)
		}
		out.PerAccount3h = clampRatio(float64(n) / float64(ceil.NewOrdersPer3h))
	}

	if ceil.CertsPerRegisteredDomainPerWeek > 0 {
		n, err := q.certs.MaxCertsPerRegisteredDomainSince(ctx, caName, now.Add(-7*24*time.Hour))
		if err != nil {
			return QuotaUsage{}, fmt.Errorf("certs peak: %w", err)
		}
		out.PerRegisteredDomain = clampRatio(float64(n) / float64(ceil.CertsPerRegisteredDomainPerWeek))
	}

	return out, nil
}

func clampRatio(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}
