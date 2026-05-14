// Package netutil provides shared network utility helpers.
package netutil

import "net"

// privateNets holds pre-parsed CIDR blocks for all private/reserved IP ranges.
// Parsed once at init to avoid per-call allocations.
var privateNets = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",         // RFC 1918
		"172.16.0.0/12",      // RFC 1918
		"192.168.0.0/16",     // RFC 1918
		"fc00::/7",           // IPv6 ULA
		"0.0.0.0/8",          // this-network
		"100.64.0.0/10",      // CGNAT (RFC 6598)
		"192.0.2.0/24",       // TEST-NET-1
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // Multicast
		"240.0.0.0/4",        // Reserved
		"255.255.255.255/32", // Broadcast
		"::/128",             // Unspecified
		"::1/128",            // IPv6 loopback
		"fe80::/10",          // IPv6 link-local
		"ff00::/8",           // IPv6 multicast
		"2001:db8::/32",      // Documentation (RFC 3849)
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			nets = append(nets, ipNet)
		}
	}
	return nets
}()

// IsPrivateIP reports whether ip is private, reserved, or otherwise blocked
// from public internet access. Covers RFC 1918, CGNAT, link-local, loopback,
// multicast, IPv6 ULA, and IANA-reserved documentation ranges.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
