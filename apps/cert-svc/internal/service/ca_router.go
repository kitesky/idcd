package service

import (
	"errors"

	"github.com/kite365/idcd/lib/cert/ca"
)

// Router selects an AcmeCA implementation for an order. S1 only knows
// Let's Encrypt; S3 will branch on order.Tier (paid DV / OV / EV) and the
// reseller channel column to dispatch into the ResellerCA interface.
type Router struct {
	le ca.AcmeCA
}

// ErrNoCA is returned when Router has nothing wired.
var ErrNoCA = errors.New("ca router: no CA configured")

// NewRouter constructs a Router. le must be non-nil in S1 — the only
// supported issuance path is ACME / Let's Encrypt.
func NewRouter(le ca.AcmeCA) *Router {
	return &Router{le: le}
}

// Pick returns the CA to use for the next order. S1 always returns the
// configured Let's Encrypt adapter; future signatures will accept the
// order for dispatch.
func (r *Router) Pick() (ca.AcmeCA, error) {
	if r == nil || r.le == nil {
		return nil, ErrNoCA
	}
	return r.le, nil
}
