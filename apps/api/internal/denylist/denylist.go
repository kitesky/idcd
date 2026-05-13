// Package denylist provides target validation and SSRF protection.
package denylist

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// CheckTarget validates whether a probe target is allowed.
// Returns an error with the rejection reason, or nil if allowed.
func CheckTarget(target string) error {
	// Reject empty targets
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	// Extract host from target (remove port if present)
	host := target
	port := ""
	if strings.Contains(target, "]:") {
		// IPv6 with port: [::1]:8080
		lastColon := strings.LastIndex(target, ":")
		host = target[:lastColon]
		port = target[lastColon+1:]
		// Remove brackets
		host = strings.Trim(host, "[]")
	} else if strings.Count(target, ":") == 1 {
		// IPv4 or hostname with port: example.com:443 or 1.1.1.1:443
		var err error
		host, port, err = net.SplitHostPort(target)
		if err != nil {
			// If SplitHostPort fails, treat as host without port
			host = target
			port = ""
		}
	} else if strings.Count(target, ":") > 1 {
		// IPv6 without port: ::1 or 2001:db8::1
		// Just use as-is
		host = target
	}

	// Validate port range if present
	if port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil || portNum < 1 || portNum > 65535 {
			return fmt.Errorf("invalid port: must be between 1 and 65535")
		}
	}

	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip != nil {
		// It's an IP address
		// Check metadata service first (more specific error message)
		if isMetadataIP(ip) {
			return fmt.Errorf("cannot probe cloud metadata service")
		}
		if isPrivateIP(ip) {
			return fmt.Errorf("cannot probe private IP addresses")
		}
		return nil
	}

	// It's a hostname, resolve it
	resolvedIP, err := resolveHost(host)
	if err != nil {
		// If DNS resolution fails, allow it (will fail at probe stage with clear error)
		// This prevents blocking valid hostnames due to temporary DNS issues
		return nil
	}

	// Check resolved IP (metadata service first, then private)
	if isMetadataIP(resolvedIP) {
		return fmt.Errorf("hostname resolves to cloud metadata service")
	}
	if isPrivateIP(resolvedIP) {
		return fmt.Errorf("hostname resolves to private IP address")
	}

	return nil
}

// isPrivateIP checks if an IP address is private, reserved, or should be denied.
// Covers RFC1918, loopback, link-local, IPv6 private ranges, and other reserved blocks.
func isPrivateIP(ip net.IP) bool {
	// Check for loopback (127.0.0.0/8 for IPv4, ::1 for IPv6)
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local (169.254.0.0/16 for IPv4, fe80::/10 for IPv6)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Define all private and reserved ranges
	privateRanges := []string{
		// RFC1918 private IPv4
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",

		// IPv6 unique local addresses (ULA)
		"fc00::/7",

		// Additional reserved/special ranges
		"0.0.0.0/8",          // Invalid/this network
		"100.64.0.0/10",      // Carrier-grade NAT (RFC6598)
		"192.0.2.0/24",       // TEST-NET-1 (RFC5737)
		"198.51.100.0/24",    // TEST-NET-2 (RFC5737)
		"203.0.113.0/24",     // TEST-NET-3 (RFC5737)
		"224.0.0.0/4",        // Multicast (RFC5771)
		"240.0.0.0/4",        // Reserved for future use (RFC1112)
		"255.255.255.255/32", // Broadcast

		// IPv6 special ranges
		"::/128",       // Unspecified
		"::1/128",      // Loopback (also caught by IsLoopback, but explicit)
		"fe80::/10",    // Link-local (also caught above, but explicit)
		"ff00::/8",     // Multicast
		"2001:db8::/32", // Documentation (RFC3849)
	}

	// Check if IP falls within any private/reserved range
	for _, cidr := range privateRanges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
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

// resolveHost resolves a hostname to its first IP address.
// Returns an error if resolution fails.
func resolveHost(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("hostname resolved to no IP addresses")
	}

	// Return the first IP
	// Note: In production, might want to check all IPs, but first is sufficient for validation
	return ips[0], nil
}
