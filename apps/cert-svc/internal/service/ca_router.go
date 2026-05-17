package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	// secondaryName is the CA to fall through to when default's
	// usage exceeds SwitchThreshold. Empty = no fallback.
	secondaryName string
	quota         QuotaChecker
	logger        *slog.Logger // optional, nil safe
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

// WithSecondary configures the fall-over CA used when default usage
// exceeds SwitchThreshold. name must match a registered ca.Name();
// otherwise the secondary is recorded but the fall-over short-circuits
// at Pick time and the default is returned (so an operator typo never
// blocks issuance). Returns r for fluent chaining; nil-safe.
func (r *Router) WithSecondary(name string) *Router {
	if r == nil {
		return nil
	}
	r.secondaryName = name
	return r
}

// WithQuota wires a QuotaChecker. Passing nil resets to the no-op
// checker so the default CA is always picked. Returns r for fluent
// chaining; nil-safe.
func (r *Router) WithQuota(qc QuotaChecker) *Router {
	if r == nil {
		return nil
	}
	r.quota = qc
	return r
}

// WithLogger wires an optional slog.Logger for fall-over events. nil
// disables logging. Returns r for fluent chaining; nil-safe.
func (r *Router) WithLogger(l *slog.Logger) *Router {
	if r == nil {
		return nil
	}
	r.logger = l
	return r
}

// Pick selects the CA for an order. Returns the CA registered under
// order.CA when that field is non-empty and registered; otherwise the
// default (possibly redirected to the secondary by quota policy).
// order may be nil (renewal probe / health path) — the default is
// returned in that case. This is the legacy entrypoint kept for
// callers that don't carry a context; it forwards to PickCtx with
// context.Background().
func (r *Router) Pick(order *repo.Order) (ca.AcmeCA, error) {
	return r.PickCtx(context.Background(), order)
}

// PickCtx is the context-aware variant of Pick. The ctx is forwarded
// to QuotaChecker.Usage so the hot issuance path can honor caller
// deadlines / cancellation when consulting Redis / Postgres for recent
// volume.
func (r *Router) PickCtx(ctx context.Context, order *repo.Order) (ca.AcmeCA, error) {
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

	def := r.cas[r.defaultName]

	// Fall-over short-circuits: no quota wired, no secondary
	// configured, or secondary not registered → just return default.
	if r.quota == nil || r.secondaryName == "" {
		return def, nil
	}
	sec, ok := r.cas[r.secondaryName]
	if !ok {
		return def, nil
	}

	usage, err := r.quota.Usage(ctx, r.defaultName)
	if err != nil {
		// Quota lookup failed — degrade open: keep issuing from
		// default and let the operator notice via logs. The hot
		// path must not block on monitoring infra.
		if r.logger != nil {
			r.logger.Warn("ca_quota_lookup_failed",
				slog.String("default", r.defaultName),
				slog.String("error", err.Error()),
			)
		}
		return def, nil
	}

	peak := usage.PerRegisteredDomain
	if usage.PerAccount3h > peak {
		peak = usage.PerAccount3h
	}
	if peak > SwitchThreshold {
		if r.logger != nil {
			r.logger.Info("ca_quota_fallover",
				slog.String("default", r.defaultName),
				slog.String("secondary", r.secondaryName),
				slog.Float64("per_registered_domain", usage.PerRegisteredDomain),
				slog.Float64("per_account_3h", usage.PerAccount3h),
				slog.Float64("threshold", SwitchThreshold),
			)
		}
		return sec, nil
	}
	return def, nil
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
