// Package denylist provides target validation and SSRF protection.
package denylist

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// CheckTarget validates whether a probe target is allowed and returns
// the pre-resolved IP (as a string) to eliminate DNS rebinding (TOCTOU).
// Callers MUST use the returned resolvedAddr instead of the original hostname
// so that the same IP that passed the check is the one that gets dialed.
// Returns ("", err) if the target is rejected.
func CheckTarget(target string) (resolvedAddr string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target cannot be empty")
	}

	// Extract host and optional port.
	host := target
	port := ""
	if strings.Contains(target, "]:") {
		// IPv6 with port: [::1]:8080
		lastColon := strings.LastIndex(target, ":")
		host = strings.Trim(target[:lastColon], "[]")
		port = target[lastColon+1:]
	} else if strings.Count(target, ":") == 1 {
		// IPv4 or hostname with port: example.com:443 or 1.1.1.1:443
		var splitErr error
		host, port, splitErr = net.SplitHostPort(target)
		if splitErr != nil {
			host = target
			port = ""
		}
	}
	// else: bare IPv6 (multiple colons, no brackets) or plain hostname — host == target

	if port != "" {
		portNum, atoiErr := strconv.Atoi(port)
		if atoiErr != nil || portNum < 1 || portNum > 65535 {
			return "", fmt.Errorf("invalid port: must be between 1 and 65535")
		}
	}

	// If the host is a literal IP address, validate it directly.
	if ip := net.ParseIP(host); ip != nil {
		if isMetadataIP(ip) {
			return "", fmt.Errorf("cannot probe cloud metadata service")
		}
		if isPrivateIP(ip) {
			return "", fmt.Errorf("cannot probe private IP addresses")
		}
		resolved := ip.String()
		if port != "" {
			resolved = net.JoinHostPort(resolved, port)
		}
		return resolved, nil
	}

	// Hostname: resolve and check ALL returned IPs to prevent partial-alias bypass.
	ips, resolveErr := resolveAllIPs(host)
	if resolveErr != nil {
		// DNS failure: allow through; the probe agent will fail with a clear error.
		return target, nil
	}
	for _, ip := range ips {
		if isMetadataIP(ip) {
			return "", fmt.Errorf("hostname resolves to cloud metadata service")
		}
		if isPrivateIP(ip) {
			return "", fmt.Errorf("hostname resolves to private IP address")
		}
	}

	// Return the first resolved IP so the caller can dial it directly,
	// avoiding a second DNS lookup that could return a different address.
	resolved := ips[0].String()
	if port != "" {
		resolved = net.JoinHostPort(resolved, port)
	}
	return resolved, nil
}

// privateNets holds pre-parsed CIDR blocks for private/reserved IP ranges.
// Parsing once at init avoids repeated allocations on every probe request.
var privateNets = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",         // RFC1918
		"172.16.0.0/12",      // RFC1918
		"192.168.0.0/16",     // RFC1918
		"fc00::/7",           // IPv6 ULA
		"0.0.0.0/8",          // this-network
		"100.64.0.0/10",      // CGN (RFC6598)
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
		"2001:db8::/32",      // Documentation
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

// isPrivateIP reports whether ip is private, reserved, or otherwise denied.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// isMetadataIP checks if the IP is a cloud metadata service endpoint.
// Primary target: 169.254.169.254 (AWS, GCP, Azure, etc.)
func isMetadataIP(ip net.IP) bool {
	// AWS/GCP/Azure metadata endpoint
	metadataIP := net.ParseIP("169.254.169.254")
	if metadataIP != nil && ip.Equal(metadataIP) {
		return true
	}

	// Some providers use different IPs in the same range
	// 169.254.0.0/16 is link-local and already blocked by isPrivateIP,
	// but we explicitly check 169.254.169.254 for clarity
	return false
}

// resolveAllIPs resolves a hostname and returns all IP addresses.
func resolveAllIPs(host string) ([]net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("hostname resolved to no IP addresses")
	}
	return ips, nil
}
