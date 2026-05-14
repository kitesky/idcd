package channel

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockedHosts lists hostnames that must never be used as webhook targets,
// regardless of whether they parse as IPs.
var blockedHosts = []string{
	"metadata.google.internal",
	"169.254.169.254",
	"fd00:ec2::254",
	"localhost",
}

// privateRanges4 holds pre-parsed RFC-1918 + CGNAT CIDR blocks.
// Parsed once at init to avoid per-call allocations inside isPrivateOrReserved.
var privateRanges4 = func() []net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10", // CGNAT
	}
	blocks := make([]net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, block, _ := net.ParseCIDR(c)
		if block != nil {
			blocks = append(blocks, *block)
		}
	}
	return blocks
}()

// validateWebhookURL rejects URLs that could be used for SSRF attacks.
// Rules:
//  1. Scheme must be "https" (plain http is not allowed for production webhooks).
//  2. The resolved host must not be a loopback, link-local, private, or unspecified address.
//
// Returns a descriptive error if the URL is unsafe or malformed.
func validateWebhookURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("webhook url must not be empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("webhook url parse error: %w", err)
	}

	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("webhook url scheme must be https, got %q", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook url missing host")
	}

	// Reject bare IP literals that are in private/reserved ranges.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrReserved(ip) {
			return fmt.Errorf("webhook url resolves to a private/reserved address: %s", host)
		}
	}

	// Block well-known cloud-metadata hostnames by name as a defence-in-depth measure.
	lower := strings.ToLower(host)
	for _, b := range blockedHosts {
		if lower == b {
			return fmt.Errorf("webhook url host %q is blocked", host)
		}
	}

	return nil
}

// isPrivateOrReserved returns true for addresses that should never be
// reachable over the public internet.
func isPrivateOrReserved(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	for _, block := range privateRanges4 {
		if block.Contains(ip) {
			return true
		}
	}

	// fc00::/7 — ULA (Unique Local IPv6)
	if ip6 := ip.To16(); ip6 != nil && ip.To4() == nil {
		if ip6[0]&0xfe == 0xfc {
			return true
		}
	}

	return false
}
