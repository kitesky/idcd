// Package netfilter provides SSRF / metadata-IP protection for probe targets
// and any other code that takes a user-controlled host before dialing the network.
//
// It is intentionally conservative: any host that resolves to a private,
// reserved, loopback, link-local, CGNAT, or known cloud-metadata IP is blocked.
// Callers should treat a blocked host as a hard failure and refuse to perform
// any network I/O against it.
//
// The package builds on github.com/kite365/idcd/lib/shared/netutil.IsPrivateIP
// (RFC 1918 / CGNAT / link-local / ULA / documentation / multicast) and adds an
// explicit registry of cloud metadata endpoints so the reason string can pin
// down "this looked like an SSRF to the cloud metadata service" — useful for
// audit logs and incident response.
package netfilter

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/kite365/idcd/lib/shared/netutil"
)

// metadataCIDRs lists cloud metadata service endpoints. Most are already
// covered by netutil.IsPrivateIP (they live inside link-local / CGNAT / ULA
// blocks), but we keep an explicit registry so we can report the precise
// reason and so a future operator who tightens netutil cannot accidentally
// unblock them.
//
// Sources:
//   - AWS / GCP / Azure / DigitalOcean: 169.254.169.254 (IMDS, RFC 3927 link-local)
//   - Alibaba Cloud:                    100.100.100.200 (CGNAT range)
//   - Oracle Cloud:                     192.0.0.192     (IANA IETF assignments)
//   - AWS IPv6 IMDS:                    fd00:ec2::254   (IPv6 ULA)
//   - GCP IPv6 metadata:                fd00:ec2::254 / fe80::a9fe:a9fe
var metadataCIDRs = []string{
	"169.254.169.254/32",
	"100.100.100.200/32",
	"192.0.0.192/32",
	"fd00:ec2::254/128",
	"fe80::a9fe:a9fe/128",
}

// metadataNets is the parsed form of metadataCIDRs, built once at init.
var metadataNets = func() []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(metadataCIDRs))
	for _, cidr := range metadataCIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// Config carries optional tunables for the package-level resolver/check path.
// All fields are optional. The zero value is safe and matches the default
// conservative behaviour.
type Config struct {
	// AllowList is a list of CIDR blocks whose IPs are permitted even if they
	// would otherwise be blocked. Use it for tools (speedtest, mirrors, etc.)
	// that legitimately need to reach a fixed set of operator-controlled
	// public hosts whose IPs you want to pin against DNS rebinding. AllowList
	// takes precedence over every other rule — set it carefully.
	AllowList []*net.IPNet

	// Resolver overrides the hostname → IP lookup. Defaults to net.LookupIP.
	// Useful for tests and for callers that already hold a *net.Resolver.
	Resolver func(host string) ([]net.IP, error)
}

// defaultResolver wraps net.LookupIP so callers can swap it out in tests.
func defaultResolver(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

var (
	globalMu  sync.RWMutex
	globalCfg = Config{Resolver: defaultResolver}
)

// SetGlobal replaces the package-level config used by IsBlocked.
// The Resolver field falls back to net.LookupIP when nil.
//
// SetGlobal is intended to be called once at process startup. It is safe to
// call concurrently with IsBlocked, but the new config only takes effect for
// subsequent calls.
func SetGlobal(cfg Config) {
	if cfg.Resolver == nil {
		cfg.Resolver = defaultResolver
	}
	globalMu.Lock()
	globalCfg = cfg
	globalMu.Unlock()
}

// ResetGlobal restores the default conservative config. Mainly for tests.
func ResetGlobal() {
	SetGlobal(Config{})
}

// getGlobal returns a snapshot of the current global config.
func getGlobal() Config {
	globalMu.RLock()
	cfg := globalCfg
	globalMu.RUnlock()
	if cfg.Resolver == nil {
		cfg.Resolver = defaultResolver
	}
	return cfg
}

// ParseAllowList parses a slice of CIDR strings into a usable AllowList.
// Invalid CIDRs return an error rather than being silently dropped — operators
// should know if their allow-list is misconfigured.
func ParseAllowList(cidrs []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", raw, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// IsBlocked reports whether the host should be denied network access.
//
// host may be either an IP literal ("169.254.169.254", "::1") or a hostname
// ("metadata.google.internal", "example.com"). Hostnames are resolved via the
// global resolver and every returned IP is checked — a single private IP in
// the result is enough to block.
//
// When the host is blocked, the second return value is a short human-readable
// reason ("cloud metadata service", "private/reserved IP", "unresolvable").
// When not blocked, the reason is the empty string.
//
// Unresolvable hostnames are blocked by default: refusing is safer than
// letting the downstream Dial pick a (possibly different) IP on retry.
func IsBlocked(host string) (bool, string) {
	return IsBlockedWith(host, getGlobal())
}

// IsBlockedWith is like IsBlocked but uses the supplied Config instead of the
// process-global one. Useful when different subsystems (agent vs API) need
// different AllowLists.
func IsBlockedWith(host string, cfg Config) (bool, string) {
	host = strings.TrimSpace(host)
	if host == "" {
		return true, "empty host"
	}

	// Strip [..] brackets around literal IPv6, which net.ParseIP doesn't accept.
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}

	resolver := cfg.Resolver
	if resolver == nil {
		resolver = defaultResolver
	}

	// IP literal? Check it directly without DNS.
	if ip := net.ParseIP(host); ip != nil {
		if allowedByCIDR(ip, cfg.AllowList) {
			return false, ""
		}
		if blocked, reason := checkIP(ip); blocked {
			return true, reason
		}
		return false, ""
	}

	// Hostname: resolve and check every IP it points at.
	ips, err := resolver(host)
	if err != nil {
		return true, "unresolvable host: " + err.Error()
	}
	if len(ips) == 0 {
		return true, "host resolved to no addresses"
	}

	for _, ip := range ips {
		if allowedByCIDR(ip, cfg.AllowList) {
			continue
		}
		if blocked, reason := checkIP(ip); blocked {
			return true, fmt.Sprintf("hostname %q resolves to %s (%s)", host, ip.String(), reason)
		}
	}
	return false, ""
}

// checkIP returns (true, reason) if ip falls into any blocked range,
// (false, "") otherwise. AllowList must be checked by the caller; this
// function is unconditional.
func checkIP(ip net.IP) (bool, string) {
	if ip == nil {
		return true, "invalid IP"
	}
	// Metadata IPs first so the reason string is the specific one.
	for _, n := range metadataNets {
		if n.Contains(ip) {
			return true, "cloud metadata service"
		}
	}
	if netutil.IsPrivateIP(ip) {
		return true, "private/reserved IP"
	}
	return false, ""
}

// allowedByCIDR reports whether ip falls inside any allow-list CIDR.
func allowedByCIDR(ip net.IP, allowList []*net.IPNet) bool {
	for _, n := range allowList {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// MetadataCIDRs returns a copy of the configured metadata CIDR list, mainly
// for tests and diagnostics.
func MetadataCIDRs() []string {
	out := make([]string, len(metadataCIDRs))
	copy(out, metadataCIDRs)
	return out
}
