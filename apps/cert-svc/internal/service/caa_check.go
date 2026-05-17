package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	mdns "github.com/miekg/dns"
)

// Sentinel errors returned by CheckCAA. Handlers branch on these via
// errors.Is to map onto user-facing HTTP responses.
var (
	// ErrCAAForbidden means at least one of the requested SANs has a
	// CAA record set that does NOT permit the target CA. The order
	// must be rejected — issuing it anyway would burn quota at the CA
	// only to be denied at validation.
	ErrCAAForbidden = errors.New("service: CAA forbids target CA")
	// ErrCAACheckFailed signals a transient DNS lookup failure. Callers
	// SHOULD NOT block order creation on this — the CA will re-check
	// CAA at validation time. Surface as a log line + 200, not 4xx.
	ErrCAACheckFailed = errors.New("service: CAA lookup failed")
)

// caaCAToTag maps cert-svc's internal CA identifier (the same string
// stored on cert.orders.ca) to the canonical CAA "issue" / "issuewild"
// tag value. Update this map whenever a new CA is wired up.
var caaCAToTag = map[string]string{
	"letsencrypt":   "letsencrypt.org",
	"lets-encrypt":  "letsencrypt.org",
	"zerossl":       "sectigo.com",
	"buypass":       "buypass.com",
	"gts":           "pki.goog",
	"google":        "pki.goog",
}

// caaLookupTimeout caps each individual DNS query so a slow / dead NS
// cannot stall the order-creation path. 3s is the same budget the lego
// client uses for its propagation probe.
const caaLookupTimeout = 3 * time.Second

// caaCacheTTL is how long an allow / deny verdict for (domain, ca) is
// cached in Redis. Five minutes balances "user just changed CAA, please
// retry" against "stop hammering NS every order".
const caaCacheTTL = 5 * time.Minute

// caaLookupFunc is the seam tests use to substitute a deterministic
// lookup. Production code uses defaultCAALookup, which dispatches to the
// authoritative NS for each candidate parent zone.
type caaLookupFunc func(ctx context.Context, fqdn string) (records []caaRecord, hasAny bool, err error)

// caaRecord is the bit of an RFC 8659 CAA RR we actually use. flags is
// kept for future critical-flag handling but is not consulted in S1.
type caaRecord struct {
	flags uint8
	tag   string
	value string
}

// CheckCAA validates that every supplied domain either has no CAA records
// in the lookup chain or has at least one issue / issuewild record that
// allows the target CA. Wildcards (*.example.com) consult issuewild
// first; falling back to issue if issuewild is absent (RFC 8659 §4.3).
//
// Returns:
//   - nil — issuance allowed for every domain
//   - ErrCAAForbidden wrapping the first offending domain + observed tags
//   - ErrCAACheckFailed wrapping the underlying DNS error; handler logs +
//     continues (the CA re-checks CAA itself, so we are safe)
func (s *Service) CheckCAA(ctx context.Context, domains []string, caID string) error {
	if len(domains) == 0 {
		return nil
	}
	wanted, ok := caaCAToTag[strings.ToLower(strings.TrimSpace(caID))]
	if !ok {
		// Unknown CA: be conservative and skip — return CheckFailed so
		// the handler logs + continues. The order may still be created
		// and the CA will perform its own CAA check.
		return fmt.Errorf("%w: unknown CA %q", ErrCAACheckFailed, caID)
	}
	lookup := s.caaLookup()
	for _, d := range domains {
		if err := s.checkCAAOne(ctx, d, caID, wanted, lookup); err != nil {
			return err
		}
	}
	return nil
}

// checkCAAOne handles a single SAN. Result is cached in Redis to amortise
// the per-order DNS round-trip; SETNX EX semantics let us treat both the
// allow and deny verdicts as cacheable for caaCacheTTL.
func (s *Service) checkCAAOne(ctx context.Context, san, caID, wantedTag string, lookup caaLookupFunc) error {
	wildcard := strings.HasPrefix(san, "*.")
	base := strings.TrimPrefix(san, "*.")
	cacheKey := "cert:caa:" + base + ":" + caID
	if wildcard {
		cacheKey += ":wild"
	}

	if s.cfg.Redis != nil {
		v, err := s.cfg.Redis.Get(ctx, cacheKey).Result()
		if err == nil {
			switch v {
			case "ok":
				return nil
			case "forbid":
				return fmt.Errorf("%w: %s", ErrCAAForbidden, san)
			}
		} else if !errors.Is(err, redis.Nil) {
			// Cache lookup error is non-fatal; fall through to live.
		}
	}

	verdict := s.classifyCAA(ctx, base, wildcard, wantedTag, lookup)
	if errors.Is(verdict, ErrCAACheckFailed) {
		// Don't cache transient failures.
		return fmt.Errorf("%w: %s", ErrCAACheckFailed, san)
	}

	cacheVal := "ok"
	if verdict != nil {
		cacheVal = "forbid"
	}
	if s.cfg.Redis != nil {
		_ = s.cfg.Redis.Set(ctx, cacheKey, cacheVal, caaCacheTTL).Err()
	}
	if verdict != nil {
		return fmt.Errorf("%w: %s", ErrCAAForbidden, san)
	}
	return nil
}

// classifyCAA walks the candidate parent zones bottom-up. The first zone
// with at least one CAA record wins; we evaluate issue / issuewild
// against that single rrset. Above the highest covered zone we
// short-circuit to "allow" (CAA default-open).
func (s *Service) classifyCAA(ctx context.Context, base string, wildcard bool, wantedTag string, lookup caaLookupFunc) error {
	candidates := caaCandidates(base)
	for _, cand := range candidates {
		lookCtx, cancel := context.WithTimeout(ctx, caaLookupTimeout)
		records, hasAny, err := lookup(lookCtx, cand)
		cancel()
		if err != nil {
			return ErrCAACheckFailed
		}
		if !hasAny {
			continue
		}
		// First level with records wins. Decide allow / deny from this
		// rrset and return.
		if recordsAllow(records, wildcard, wantedTag) {
			return nil
		}
		return ErrCAAForbidden
	}
	// No level had any records → allow.
	return nil
}

// recordsAllow implements the RFC 8659 §4 evaluation: wildcard SANs
// prefer "issuewild"; non-wildcard use "issue". When issuewild is absent
// the wildcard query falls back to "issue".
func recordsAllow(records []caaRecord, wildcard bool, wantedTag string) bool {
	var issue, issuewild []caaRecord
	for _, r := range records {
		switch strings.ToLower(r.tag) {
		case "issue":
			issue = append(issue, r)
		case "issuewild":
			issuewild = append(issuewild, r)
		}
	}
	var pool []caaRecord
	if wildcard {
		if len(issuewild) > 0 {
			pool = issuewild
		} else {
			pool = issue
		}
	} else {
		pool = issue
	}
	if len(pool) == 0 {
		// "issue" missing entirely means no CAs permitted (RFC 8659 §4.2).
		// Wildcard with issuewild missing falls back to issue (handled).
		return false
	}
	for _, r := range pool {
		// Value is "ca-domain.example" or "ca-domain.example; param=…".
		ca := strings.TrimSpace(strings.SplitN(r.value, ";", 2)[0])
		ca = strings.ToLower(strings.Trim(ca, `"`))
		if ca == "" {
			// Empty issue value is RFC-defined as "no CA permitted".
			continue
		}
		if ca == wantedTag {
			return true
		}
	}
	return false
}

// caaCandidates returns the bottom-up list of zone names we should query
// CAA on, starting with the SAN itself. We stop at the registrable
// boundary the CA actually checks — for S1 we accept "all labels up to
// the TLD" which is RFC-correct but slightly more aggressive than the
// usual PSL-aware stop.
func caaCandidates(base string) []string {
	base = strings.TrimSuffix(base, ".")
	if base == "" {
		return nil
	}
	labels := strings.Split(base, ".")
	out := make([]string, 0, len(labels))
	for i := 0; i < len(labels)-1; i++ {
		out = append(out, strings.Join(labels[i:], "."))
	}
	return out
}

// caaLookup returns the lookup function to use; tests may override via
// caaLookupOverride (a package-level var).
func (s *Service) caaLookup() caaLookupFunc {
	caaLookupMu.RLock()
	defer caaLookupMu.RUnlock()
	if caaLookupOverride != nil {
		return caaLookupOverride
	}
	return defaultCAALookup
}

// caaLookupOverride lets tests substitute a fake lookup. Guarded by a
// mutex because Go tests may run in parallel.
var (
	caaLookupMu       sync.RWMutex
	caaLookupOverride caaLookupFunc
)

// SetCAALookupForTest is exported only so handler / integration tests
// can wire a deterministic CAA fixture. Production callers must not use
// it; the override is package-global so the test must restore it.
//
// The test passes a function returning (issueValues, hasAny, err). The
// receiver builds CAA records labelled "issue" / "issuewild" — both tags
// take their value from the same set so callers can keep the fixture
// shape small.
func SetCAALookupForTest(fn caaLookupFunc) (restore func()) {
	caaLookupMu.Lock()
	prev := caaLookupOverride
	caaLookupOverride = fn
	caaLookupMu.Unlock()
	return func() {
		caaLookupMu.Lock()
		caaLookupOverride = prev
		caaLookupMu.Unlock()
	}
}

// FakeCAALookup constructs an opaque lookup function for tests living in
// other packages. zoneRecords maps zone name → list of "issue" tag
// values (already-permitted CA domains). For wildcard / mixed-tag setups
// keep using the in-package SetCAALookupForTest helper.
func FakeCAALookup(zoneRecords map[string][]string) func(ctx context.Context, zone string) ([]caaRecord, bool, error) {
	return func(_ context.Context, zone string) ([]caaRecord, bool, error) {
		zone = strings.TrimSuffix(zone, ".")
		vals, ok := zoneRecords[zone]
		if !ok {
			return nil, false, nil
		}
		out := make([]caaRecord, 0, len(vals))
		for _, v := range vals {
			out = append(out, caaRecord{tag: "issue", value: v})
		}
		return out, len(out) > 0, nil
	}
}

// FakeCAAErr returns a lookup function that always reports a DNS failure,
// useful when the handler test wants to exercise the "continue on
// transient error" branch.
func FakeCAAErr() func(ctx context.Context, zone string) ([]caaRecord, bool, error) {
	return func(_ context.Context, _ string) ([]caaRecord, bool, error) {
		return nil, false, errors.New("caa: simulated dns error")
	}
}

// defaultCAALookup queries one of the zone's authoritative NS directly
// to avoid resolver caching. Falls back to the system resolver via
// net.LookupCNAME / mdns.Exchange when LookupNS yields nothing.
func defaultCAALookup(ctx context.Context, zone string) ([]caaRecord, bool, error) {
	nsRecords, err := net.DefaultResolver.LookupNS(ctx, zone)
	if err != nil || len(nsRecords) == 0 {
		// Fall back to the local resolver. Cached answers are acceptable
		// here because losing a small race against a CAA edit is not
		// security-critical — the CA itself re-verifies at issuance.
		return queryCAA(ctx, zone, "")
	}
	var lastErr error
	for _, ns := range nsRecords {
		server := strings.TrimSuffix(ns.Host, ".") + ":53"
		records, hasAny, qerr := queryCAA(ctx, zone, server)
		if qerr == nil {
			return records, hasAny, nil
		}
		lastErr = qerr
	}
	if lastErr != nil {
		return nil, false, lastErr
	}
	return nil, false, nil
}

// queryCAA issues a single CAA question. server == "" means use the
// system resolver (via miekg/dns net.Resolver-style fallback).
func queryCAA(ctx context.Context, zone string, server string) ([]caaRecord, bool, error) {
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn(zone), mdns.TypeCAA)
	if server == "" {
		// No authoritative server — use the system resolver via a quick
		// UDP probe to the local resolver. net.Resolver doesn't expose
		// CAA, so we synthesise via the first /etc/resolv.conf entry.
		cfg, _ := mdns.ClientConfigFromFile("/etc/resolv.conf")
		if cfg == nil || len(cfg.Servers) == 0 {
			// No resolver wired — treat as "no records" rather than err
			// so the caller doesn't block order creation.
			return nil, false, nil
		}
		server = cfg.Servers[0] + ":" + cfg.Port
	}
	c := new(mdns.Client)
	c.Timeout = caaLookupTimeout
	resp, _, err := c.ExchangeContext(ctx, m, server)
	if err != nil {
		return nil, false, err
	}
	if resp == nil {
		return nil, false, nil
	}
	var out []caaRecord
	for _, ans := range resp.Answer {
		if r, ok := ans.(*mdns.CAA); ok {
			out = append(out, caaRecord{
				flags: r.Flag,
				tag:   r.Tag,
				value: r.Value,
			})
		}
	}
	return out, len(out) > 0, nil
}
