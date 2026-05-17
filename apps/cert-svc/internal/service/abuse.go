package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// ErrAbuseBlocked is returned by AbuseDetector.Check when an order would
// trip one of the rate / reputation rules in PRD §12.2. The handler maps
// this onto 403 + CERT_ABUSE_BLOCKED.
var ErrAbuseBlocked = errors.New("service: abuse rule blocked order")

// AbuseDetector applies the three S1 anti-abuse rules in front of order
// creation:
//
//  1. Domain blocklist — hard-coded list of government / mega-corp roots
//     we refuse to issue for. Catches operator-error and trademark abuse.
//
//  2. Burst limit — ≥ 5 distinct root domains from the same account in
//     the last 1 hour. PRD §12.2 says this should pin manual review;
//     S1 simplifies to "reject" so we don't leak a review queue.
//
//  3. Per-root sustained — ≥ 10 orders against the same root domain in
//     the last 7 days from the same account. Catches scripted retry
//     loops that would otherwise quietly burn Let's Encrypt quota.
//
// The detector reads from cert.orders.ListByAccount; it does NOT span
// schemas, in line with D1.
type AbuseDetector struct {
	repos     *repo.Repos
	blocklist []string
	now       func() time.Time
	logger    *slog.Logger

	burstWindow      time.Duration
	burstDistinctMax int
	sustainedWindow  time.Duration
	sustainedMax     int
	lookbackLimit    int
}

// defaultBlocklist is the hard-coded S1 list. Suffix match — entries
// that start with "*." are converted to the bare root and matched as
// "domain == root || domain endsWith .root".
var defaultBlocklist = []string{
	"gov.cn",
	"gov",
	"mil",
	"taobao.com",
	"tmall.com",
	"alipay.com",
	"wechat.com",
	"weixin.com",
	"qq.com",
	"baidu.com",
	"google.com",
	"microsoft.com",
	"apple.com",
	"bank.com",
	"paypal.com",
	"amazon.com",
}

// AbuseOption configures the detector. Used by tests to shrink windows
// or substitute the clock; production passes none.
type AbuseOption func(*AbuseDetector)

// WithAbuseBlocklist overrides the hard-coded blocklist. Empty slice
// disables the blocklist check entirely (the burst / sustained rules
// still apply).
func WithAbuseBlocklist(list []string) AbuseOption {
	return func(a *AbuseDetector) { a.blocklist = list }
}

// WithAbuseClock pins time.Now for deterministic tests.
func WithAbuseClock(now func() time.Time) AbuseOption {
	return func(a *AbuseDetector) { a.now = now }
}

// WithAbuseBurst overrides the burst window + distinct-root threshold.
func WithAbuseBurst(window time.Duration, maxDistinct int) AbuseOption {
	return func(a *AbuseDetector) {
		a.burstWindow = window
		a.burstDistinctMax = maxDistinct
	}
}

// WithAbuseSustained overrides the sustained-per-root window + threshold.
func WithAbuseSustained(window time.Duration, maxOrders int) AbuseOption {
	return func(a *AbuseDetector) {
		a.sustainedWindow = window
		a.sustainedMax = maxOrders
	}
}

// WithAbuseLogger plumbs a slog logger; the detector logs each block at
// WARN with the reason + account id.
func WithAbuseLogger(l *slog.Logger) AbuseOption {
	return func(a *AbuseDetector) { a.logger = l }
}

// NewAbuseDetector constructs a detector with the S1 defaults. repos may
// be nil — the burst / sustained checks become no-ops and only the
// blocklist is enforced (useful for tests that don't want pgxmock).
func NewAbuseDetector(repos *repo.Repos, opts ...AbuseOption) *AbuseDetector {
	a := &AbuseDetector{
		repos:            repos,
		blocklist:        defaultBlocklist,
		now:              time.Now,
		logger:           slog.Default(),
		burstWindow:      time.Hour,
		burstDistinctMax: 5,
		sustainedWindow:  7 * 24 * time.Hour,
		sustainedMax:     10,
		lookbackLimit:    500, // enough headroom for the 24h quota path
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Check returns nil when the order is allowed, ErrAbuseBlocked (wrapping
// a human-readable reason) otherwise. sans must already be canonicalised
// to ASCII / Punycode; the detector lowercases defensively but does no
// IDN normalisation.
func (a *AbuseDetector) Check(ctx context.Context, accountID int64, sans []string) error {
	if a == nil || len(sans) == 0 {
		return nil
	}
	roots := uniqueRoots(sans)

	// Rule 1: blocklist.
	for _, root := range roots {
		if a.isBlocked(root) {
			a.log("abuse blocklist hit", accountID, root)
			return fmt.Errorf("%w: domain %q is on the static blocklist", ErrAbuseBlocked, root)
		}
	}

	if a.repos == nil || a.repos.Orders == nil {
		// No order history available — burst / sustained checks skipped.
		return nil
	}

	// Pull the recent history once and reuse for both windowed checks.
	history, err := a.repos.Orders.ListByAccount(ctx, accountID, nil, a.lookbackLimit, 0)
	if err != nil {
		// Don't fail-open silently: log + allow. Order creation is more
		// important than this defensive check; the CA still rate-limits.
		a.log("abuse history fetch failed", accountID, err.Error())
		return nil
	}

	now := a.now().UTC()
	burstCutoff := now.Add(-a.burstWindow)
	sustainedCutoff := now.Add(-a.sustainedWindow)

	// Rule 2: burst — distinct roots in last burstWindow ≥ burstDistinctMax.
	burstRoots := make(map[string]struct{})
	// Rule 3: sustained — count per root in last sustainedWindow.
	perRoot := make(map[string]int)
	for _, o := range history {
		if o.CreatedAt.Before(sustainedCutoff) {
			continue
		}
		for _, root := range uniqueRoots(o.SANs) {
			if o.CreatedAt.After(burstCutoff) {
				burstRoots[root] = struct{}{}
			}
			perRoot[root]++
		}
	}
	// Add the in-flight request to the windowed snapshots.
	for _, r := range roots {
		burstRoots[r] = struct{}{}
		perRoot[r]++
	}

	if len(burstRoots) > a.burstDistinctMax {
		a.log("abuse burst hit", accountID, fmt.Sprintf("distinct=%d", len(burstRoots)))
		return fmt.Errorf("%w: %d distinct root domains in last %s (limit %d)",
			ErrAbuseBlocked, len(burstRoots), a.burstWindow, a.burstDistinctMax)
	}

	for _, r := range roots {
		if perRoot[r] > a.sustainedMax {
			a.log("abuse sustained hit", accountID, fmt.Sprintf("root=%s count=%d", r, perRoot[r]))
			return fmt.Errorf("%w: %q used %d times in last %s (limit %d)",
				ErrAbuseBlocked, r, perRoot[r], a.sustainedWindow, a.sustainedMax)
		}
	}

	return nil
}

// isBlocked checks whether root matches one of the blocklist entries. A
// match is either exact or a parent suffix — e.g. "gov.cn" blocks both
// "moe.gov.cn" and "gov.cn" but does NOT block "notgov.cn".
func (a *AbuseDetector) isBlocked(root string) bool {
	root = strings.ToLower(strings.TrimSuffix(root, "."))
	for _, entry := range a.blocklist {
		e := strings.ToLower(strings.TrimPrefix(entry, "*."))
		if root == e {
			return true
		}
		if strings.HasSuffix(root, "."+e) {
			return true
		}
	}
	return false
}

// log emits a structured WARN line. The detector keeps this small + cheap
// so callers can leave logging enabled in prod.
func (a *AbuseDetector) log(msg string, accountID int64, detail string) {
	if a.logger == nil {
		return
	}
	a.logger.Warn(msg, "account_id", accountID, "detail", detail)
}

// uniqueRoots projects each SAN onto a "registrable" root. For S1 we
// strip "*." prefix and take the last two labels (so "a.b.example.com"
// → "example.com"). This is a deliberately small PSL substitute; the
// only known false-positive class is two-label public suffixes (.co.uk,
// .com.cn) which the static blocklist works around explicitly.
func uniqueRoots(sans []string) []string {
	seen := make(map[string]struct{}, len(sans))
	out := make([]string, 0, len(sans))
	for _, s := range sans {
		root := domainRoot(s)
		if root == "" {
			continue
		}
		if _, dup := seen[root]; dup {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	return out
}

// domainRoot is the minimal-PSL stand-in: strip wildcard prefix and keep
// the last 2 labels. Special-cases the two-label CN suffixes our static
// blocklist references (gov.cn, com.cn) so e.g. moe.gov.cn rolls up to
// gov.cn rather than collapsing to "gov.cn" via the last-2 rule.
func domainRoot(san string) string {
	s := strings.ToLower(strings.TrimSpace(san))
	s = strings.TrimPrefix(s, "*.")
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return ""
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return s
	}
	last2 := strings.Join(labels[len(labels)-2:], ".")
	// Two-label PSLs we know about: take last 3 labels instead.
	switch last2 {
	case "gov.cn", "com.cn", "net.cn", "org.cn", "edu.cn", "co.uk", "ac.uk":
		if len(labels) >= 3 {
			return strings.Join(labels[len(labels)-3:], ".")
		}
		return last2
	}
	return last2
}
