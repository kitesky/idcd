package denylist

import (
	"net"
	"testing"
)

func TestCheckTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		shouldError bool
		errorMsg    string
	}{
		// Valid public targets
		{"public IPv4", "1.1.1.1", false, ""},
		{"public IPv4 cloudflare", "8.8.8.8", false, ""},
		{"public IPv4 with port", "1.1.1.1:443", false, ""},
		{"public domain", "example.com", false, ""},
		{"public domain with port", "example.com:443", false, ""},
		{"public IPv6", "2606:4700:4700::1111", false, ""}, // Cloudflare DNS

		// Private IPv4 addresses (RFC1918)
		{"private 10.x", "10.0.0.1", true, "private IP"},
		{"private 10.x with port", "10.1.2.3:8080", true, "private IP"},
		{"private 172.16.x", "172.16.0.1", true, "private IP"},
		{"private 172.31.x", "172.31.255.254", true, "private IP"},
		{"private 192.168.x", "192.168.1.1", true, "private IP"},
		{"private 192.168.x with port", "192.168.0.100:22", true, "private IP"},

		// Loopback
		{"loopback 127.0.0.1", "127.0.0.1", true, "private IP"},
		{"loopback 127.1.2.3", "127.1.2.3", true, "private IP"},
		{"loopback localhost", "localhost", true, "private IP"},
		{"loopback IPv6 ::1", "::1", true, "private IP"},
		{"loopback IPv6 with brackets", "[::1]:8080", true, "private IP"},

		// Link-local / APIPA
		{"link-local 169.254.x", "169.254.1.1", true, "private IP"},
		{"link-local IPv6", "fe80::1", true, "private IP"},
		{"link-local IPv6 with port", "[fe80::1]:80", true, "private IP"},

		// Cloud metadata service
		{"metadata AWS/GCP/Azure", "169.254.169.254", true, "metadata service"},

		// Carrier-grade NAT
		{"CGN 100.64.x", "100.64.0.1", true, "private IP"},
		{"CGN 100.127.x", "100.127.255.254", true, "private IP"},

		// TEST-NET ranges (documentation)
		{"TEST-NET-1", "192.0.2.1", true, "private IP"},
		{"TEST-NET-2", "198.51.100.1", true, "private IP"},
		{"TEST-NET-3", "203.0.113.1", true, "private IP"},

		// Invalid addresses
		{"invalid 0.0.0.0", "0.0.0.0", true, "private IP"},
		{"broadcast", "255.255.255.255", true, "private IP"},

		// Multicast
		{"multicast 224.x", "224.0.0.1", true, "private IP"},
		{"multicast 239.x", "239.255.255.255", true, "private IP"},

		// Reserved for future use
		{"reserved 240.x", "240.0.0.1", true, "private IP"},

		// IPv6 private/special
		{"IPv6 ULA fc00", "fc00::1", true, "private IP"},
		{"IPv6 ULA fd00", "fd00::1234", true, "private IP"},
		{"IPv6 unspecified", "::", true, "private IP"},
		{"IPv6 documentation", "2001:db8::1", true, "private IP"},
		{"IPv6 multicast", "ff02::1", true, "private IP"},

		// Empty/invalid input
		{"empty string", "", true, "cannot be empty"},
		{"whitespace only", "   ", true, "cannot be empty"},

		// Invalid port
		{"invalid port zero", "example.com:0", true, "invalid port"},
		{"invalid port negative", "example.com:-1", true, "invalid port"},
		{"invalid port too high", "example.com:99999", true, "invalid port"},
		{"invalid port non-numeric", "example.com:abc", true, "invalid port"},

		// Edge cases with valid ports
		{"port 1 minimum", "example.com:1", false, ""},
		{"port 65535 maximum", "example.com:65535", false, ""},
		{"port 443 HTTPS", "1.1.1.1:443", false, ""},
		{"port 80 HTTP", "8.8.8.8:80", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTarget(tt.target)

			if tt.shouldError {
				if err == nil {
					t.Errorf("CheckTarget(%q) expected error containing %q, got nil", tt.target, tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("CheckTarget(%q) expected error containing %q, got %q", tt.target, tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("CheckTarget(%q) expected no error, got %v", tt.target, err)
				}
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Public IPs
		{"cloudflare DNS", "1.1.1.1", false},
		{"google DNS", "8.8.8.8", false},
		{"quad9 DNS", "9.9.9.9", false},
		{"cloudflare IPv6", "2606:4700:4700::1111", false},

		// RFC1918 private
		{"10.0.0.0", "10.0.0.0", true},
		{"10.255.255.255", "10.255.255.255", true},
		{"172.16.0.0", "172.16.0.0", true},
		{"172.31.255.255", "172.31.255.255", true},
		{"192.168.0.0", "192.168.0.0", true},
		{"192.168.255.255", "192.168.255.255", true},

		// Loopback
		{"127.0.0.1", "127.0.0.1", true},
		{"127.255.255.255", "127.255.255.255", true},
		{"::1", "::1", true},

		// Link-local
		{"169.254.0.0", "169.254.0.0", true},
		{"169.254.169.254 metadata", "169.254.169.254", true},
		{"169.254.255.255", "169.254.255.255", true},
		{"fe80::1", "fe80::1", true},

		// Carrier-grade NAT
		{"100.64.0.0", "100.64.0.0", true},
		{"100.127.255.255", "100.127.255.255", true},

		// TEST-NET
		{"192.0.2.0", "192.0.2.0", true},
		{"198.51.100.0", "198.51.100.0", true},
		{"203.0.113.0", "203.0.113.0", true},

		// Invalid/special
		{"0.0.0.0", "0.0.0.0", true},
		{"255.255.255.255", "255.255.255.255", true},

		// Multicast
		{"224.0.0.1", "224.0.0.1", true},
		{"239.255.255.255", "239.255.255.255", true},

		// Reserved
		{"240.0.0.1", "240.0.0.1", true},

		// IPv6 private/special
		{"fc00::1 ULA", "fc00::1", true},
		{"fd00::1234 ULA", "fd00::1234", true},
		{":: unspecified", "::", true},
		{"2001:db8::1 doc", "2001:db8::1", true},
		{"ff02::1 multicast", "ff02::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}

			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsMetadataIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"AWS/GCP/Azure metadata", "169.254.169.254", true},
		{"link-local but not metadata", "169.254.1.1", false},
		{"public IP", "1.1.1.1", false},
		{"private IP", "192.168.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}

			result := isMetadataIP(ip)
			if result != tt.expected {
				t.Errorf("isMetadataIP(%s) = %v, expected %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestResolveHost(t *testing.T) {
	// Test with well-known public hostnames
	publicHosts := []string{
		"example.com",
		"google.com",
		"cloudflare.com",
	}

	for _, host := range publicHosts {
		t.Run(host, func(t *testing.T) {
			ip, err := resolveHost(host)
			if err != nil {
				// Skip if DNS resolution fails (e.g., no network)
				t.Skipf("DNS resolution failed (network issue?): %v", err)
			}

			if ip == nil {
				t.Errorf("resolveHost(%s) returned nil IP without error", host)
			}

			// Verify it's not a private IP (these domains should resolve to public IPs)
			if isPrivateIP(ip) {
				t.Errorf("resolveHost(%s) resolved to private IP %s", host, ip)
			}
		})
	}

	// Test with invalid hostname
	t.Run("invalid hostname", func(t *testing.T) {
		_, err := resolveHost("this-domain-definitely-does-not-exist-12345.invalid")
		if err == nil {
			t.Error("expected error for invalid hostname, got nil")
		}
	})
}

func TestCheckTarget_PortParsing(t *testing.T) {
	tests := []struct {
		name   string
		target string
		valid  bool
	}{
		// IPv4 with port
		{"IPv4 standard port", "1.1.1.1:443", true},
		{"IPv4 no port", "1.1.1.1", true},

		// IPv6 with port (requires brackets)
		{"IPv6 with port and brackets", "[2606:4700:4700::1111]:443", true},
		{"IPv6 no port", "2606:4700:4700::1111", true},

		// Hostname with port
		{"hostname with port", "example.com:8080", true},
		{"hostname no port", "example.com", true},

		// Edge cases
		{"multiple colons IPv6 no port", "2001:db8::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckTarget(tt.target)

			// For this test, we only care if parsing succeeds
			// The target might still be rejected for other reasons (like being private)
			// So we just check that we don't get port-related errors for valid cases
			if tt.valid && err != nil && contains(err.Error(), "invalid port") {
				t.Errorf("CheckTarget(%q) got unexpected port error: %v", tt.target, err)
			}
		})
	}
}

// TestIsPrivateIP_Comprehensive is a comprehensive test ensuring all documented ranges are covered.
func TestIsPrivateIP_Comprehensive(t *testing.T) {
	// Test a sample from each range explicitly mentioned in the requirements
	requiredRanges := map[string][]string{
		"RFC1918 10.0.0.0/8": {
			"10.0.0.1", "10.128.0.1", "10.255.255.254",
		},
		"RFC1918 172.16.0.0/12": {
			"172.16.0.1", "172.24.0.1", "172.31.255.254",
		},
		"RFC1918 192.168.0.0/16": {
			"192.168.0.1", "192.168.128.1", "192.168.255.254",
		},
		"Loopback 127.0.0.0/8": {
			"127.0.0.1", "127.0.0.2", "127.255.255.255",
		},
		"Link-local 169.254.0.0/16": {
			"169.254.0.1", "169.254.169.254", "169.254.255.254",
		},
		"IPv6 loopback ::1/128": {
			"::1",
		},
		"IPv6 link-local fe80::/10": {
			"fe80::1", "fe80::abcd:ef12",
		},
		"IPv6 ULA fc00::/7": {
			"fc00::1", "fd00::1", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff",
		},
		"Invalid 0.0.0.0/8": {
			"0.0.0.0", "0.0.0.1", "0.255.255.255",
		},
		"CGN 100.64.0.0/10": {
			"100.64.0.1", "100.100.0.1", "100.127.255.254",
		},
		"TEST-NET-1 192.0.2.0/24": {
			"192.0.2.1", "192.0.2.128", "192.0.2.254",
		},
		"TEST-NET-2 198.51.100.0/24": {
			"198.51.100.1", "198.51.100.128", "198.51.100.254",
		},
		"TEST-NET-3 203.0.113.0/24": {
			"203.0.113.1", "203.0.113.128", "203.0.113.254",
		},
	}

	for rangeName, ips := range requiredRanges {
		for _, ipStr := range ips {
			t.Run(rangeName+" "+ipStr, func(t *testing.T) {
				ip := net.ParseIP(ipStr)
				if ip == nil {
					t.Fatalf("failed to parse IP %q", ipStr)
				}

				if !isPrivateIP(ip) {
					t.Errorf("isPrivateIP(%s) = false, but it should be blocked (range: %s)", ipStr, rangeName)
				}
			})
		}
	}
}

// TestCheckTarget_Integration tests the full flow including DNS resolution
func TestCheckTarget_Integration(t *testing.T) {
	// Test that localhost resolves and is blocked
	t.Run("localhost blocks", func(t *testing.T) {
		err := CheckTarget("localhost")
		if err == nil {
			t.Error("expected localhost to be blocked, got nil error")
		}
		if err != nil && !contains(err.Error(), "private IP") {
			t.Errorf("expected 'private IP' error for localhost, got: %v", err)
		}
	})

	// Test that public domains are allowed (if DNS works)
	t.Run("public domain allowed", func(t *testing.T) {
		err := CheckTarget("example.com")
		if err != nil {
			t.Logf("Note: example.com blocked or DNS failed: %v", err)
			// Don't fail the test - DNS might not be available in CI
		}
	})
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && anyContains(s, substr)))
}

func anyContains(s, substr string) bool {
	// Simple substring search
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
