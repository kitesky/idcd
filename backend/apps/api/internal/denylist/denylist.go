// Package denylist provides target validation and SSRF protection.
package denylist

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/kite365/idcd/lib/shared/netutil"
)

// CheckTarget validates whether a probe target is allowed. Returns the
// original (trimmed) target unchanged on success, and ("", err) on rejection.
//
// Earlier versions returned the first resolved IP to defeat DNS rebinding
// (TOCTOU). That broke every probe that needed the original hostname or URL:
//   - HTTP/Speedtest got a bare IPv6 address with no brackets and no scheme
//     ("2409:…:59ca"), then url.Parse blew up on the implicit "port".
//   - DNS got an IP literal instead of a name and dutifully looked up the
//     A record of "2409:…", returning an empty record set with success=true.
//
// The DNS rebinding window is closed inside the agent: lib/shared/netfilter
// re-checks the host immediately before dial, so a rebind would have to flip
// IPs in the microseconds between LookupIP and Dial — acceptable for a public
// reachability tool.
//
// Accepts full URLs (http://… or https://…), bare hostnames, or host:port pairs.
func CheckTarget(target string) (validatedTarget string, err error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target cannot be empty")
	}

	// hostPort is what the rest of the function inspects. We keep `target`
	// untouched so the original URL / hostname / host:port flows back to the
	// caller verbatim.
	hostPort := target
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		u, parseErr := url.Parse(target)
		if parseErr != nil {
			return "", fmt.Errorf("invalid URL: %w", parseErr)
		}
		hostPort = u.Host
		if hostPort == "" {
			return "", fmt.Errorf("URL has no host")
		}
	}

	// Extract host and optional port.
	host := hostPort
	port := ""
	if strings.Contains(hostPort, "]:") {
		// IPv6 with port: [::1]:8080
		lastColon := strings.LastIndex(hostPort, ":")
		host = strings.Trim(hostPort[:lastColon], "[]")
		port = hostPort[lastColon+1:]
	} else if strings.Count(hostPort, ":") == 1 {
		// IPv4 or hostname with port: example.com:443 or 1.1.1.1:443
		var splitErr error
		host, port, splitErr = net.SplitHostPort(hostPort)
		if splitErr != nil {
			host = hostPort
			port = ""
		}
	}
	// else: bare IPv6 (multiple colons, no brackets) or plain hostname — host == hostPort

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
		return target, nil
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

	return target, nil
}

// isPrivateIP reports whether ip is private, reserved, or otherwise denied.
// Delegates to netutil.IsPrivateIP which covers all RFC 1918, CGNAT, and
// reserved ranges in a single pre-parsed table.
func isPrivateIP(ip net.IP) bool {
	return netutil.IsPrivateIP(ip)
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
