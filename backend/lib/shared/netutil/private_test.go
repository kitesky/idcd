package netutil_test

import (
	"net"
	"testing"

	"github.com/kite365/idcd/lib/shared/netutil"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
		desc string
	}{
		// RFC 1918 private ranges
		{"10.0.0.1", true, "RFC1918 10/8"},
		{"10.255.255.255", true, "RFC1918 10/8 top"},
		{"172.16.0.1", true, "RFC1918 172.16/12"},
		{"172.31.255.255", true, "RFC1918 172.16/12 top"},
		{"192.168.0.1", true, "RFC1918 192.168/16"},
		{"192.168.255.255", true, "RFC1918 192.168/16 top"},

		// Loopback
		{"127.0.0.1", true, "IPv4 loopback"},
		{"127.255.255.255", true, "IPv4 loopback range"},
		{"::1", true, "IPv6 loopback"},

		// Link-local
		{"169.254.0.1", true, "IPv4 link-local"},
		{"169.254.169.254", true, "metadata service IP"},
		{"fe80::1", true, "IPv6 link-local"},

		// CGNAT (RFC 6598)
		{"100.64.0.1", true, "CGNAT"},
		{"100.127.255.255", true, "CGNAT top"},

		// Multicast
		{"224.0.0.1", true, "IPv4 multicast"},
		{"239.255.255.255", true, "IPv4 multicast top"},
		{"ff02::1", true, "IPv6 multicast"},

		// Documentation ranges
		{"192.0.2.1", true, "TEST-NET-1"},
		{"198.51.100.1", true, "TEST-NET-2"},
		{"203.0.113.1", true, "TEST-NET-3"},
		{"2001:db8::1", true, "IPv6 documentation"},

		// Reserved / special
		{"0.0.0.0", true, "unspecified"},
		{"255.255.255.255", true, "broadcast"},
		{"240.0.0.1", true, "reserved"},

		// IPv6 ULA
		{"fc00::1", true, "IPv6 ULA fc00"},
		{"fd00::1", true, "IPv6 ULA fd00"},

		// Public addresses — must NOT be blocked
		{"1.1.1.1", false, "Cloudflare DNS"},
		{"8.8.8.8", false, "Google DNS"},
		{"8.8.4.4", false, "Google DNS alt"},
		{"93.184.216.34", false, "example.com"},
		{"2606:4700:4700::1111", false, "Cloudflare DNS IPv6"},
		{"2001:4860:4860::8888", false, "Google DNS IPv6"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tt.ip)
			}
			got := netutil.IsPrivateIP(ip)
			if got != tt.want {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
