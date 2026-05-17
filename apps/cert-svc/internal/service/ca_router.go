package service

import (
	"errors"
	"fmt"
	"sort"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
)

// Router selects an AcmeCA implementation for an order. S1 only knew
// Let's Encrypt; S2 introduces a registry keyed by ca.Name() so multiple
// CA adapters (Let's Encrypt, ZeroSSL, Buypass) can coexist and per-order
// dispatch can be driven from cert.orders.ca. S3 will additionally branch
// on tier / reseller channel for paid DV / OV / EV via ResellerCA.
type Router struct {
	cas         map[string]ca.AcmeCA // keyed by ca.Name()
	defaultName string               // ca.Name() of the fallback
}

// ErrNoCA is returned when Router has nothing wired (nil router, nil
// default, or empty registry).
var ErrNoCA = errors.New("ca router: no CA configured")

// ErrUnknownCA is returned when an order asks for a CA name that was
// never registered. We refuse rather than silently fall through to the
// default — issuing from the wrong CA would violate the user's contract
// and could break CAA / billing assumptions downstream.
var ErrUnknownCA = errors.New("ca router: unknown ca for order")

// NewRouter constructs a Router. defaultCA is the fallback used when an
// order's CA field is empty or unknown; it must be non-nil. extras may
// be empty and any nil entries are silently dropped so cmd/server and
// cmd/worker can conditionally pass optional adapters without
// allocating wrapper slices.
func NewRouter(defaultCA ca.AcmeCA, extras ...ca.AcmeCA) *Router {
	if defaultCA == nil {
		// Returning nil here lets the caller's Pick() short-circuit to
		// ErrNoCA via the (*Router)(nil) receiver guard. The tests
		// rely on this contract.
		return nil
	}
	cas := map[string]ca.AcmeCA{defaultCA.Name(): defaultCA}
	for _, c := range extras {
		if c == nil {
			continue
		}
		cas[c.Name()] = c
	}
	return &Router{cas: cas, defaultName: defaultCA.Name()}
}

// Pick selects the CA for an order. Returns the CA registered under
// order.CA when that field is non-empty and registered; otherwise the
// default. order may be nil (renewal probe / health path) — in that
// case the default is returned.
func (r *Router) Pick(order *repo.Order) (ca.AcmeCA, error) {
	if r == nil || len(r.cas) == 0 {
		return nil, ErrNoCA
	}
	if order != nil && order.CA != "" {
		if c, ok := r.cas[order.CA]; ok {
			return c, nil
		}
		// Unknown CA → caller's data is bad; refuse rather than
		// silently fall through to the default (would issue from the
		// wrong CA and trip CAA / billing reconciliation).
		return nil, fmt.Errorf("%w: %q", ErrUnknownCA, order.CA)
	}
	return r.cas[r.defaultName], nil
}

// Names returns the registered CA identifiers in alphabetical order.
// Used by /healthz and admin dashboards to surface which adapters are
// wired in the running process.
func (r *Router) Names() []string {
	if r == nil || len(r.cas) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.cas))
	for name := range r.cas {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
