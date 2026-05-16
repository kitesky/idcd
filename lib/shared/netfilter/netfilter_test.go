package netfilter_test

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/shared/netfilter"
)

// stubResolver returns a fixed map of host → IPs (or an error).
func stubResolver(mapping map[string][]string, errs map[string]error) func(string) ([]net.IP, error) {
	return func(host string) ([]net.IP, error) {
		if err, ok := errs[host]; ok {
			return nil, err
		}
		raws, ok := mapping[host]
		if !ok {
			return nil, errors.New("NXDOMAIN: " + host)
		}
		out := make([]net.IP, 0, len(raws))
		for _, r := range raws {
			ip := net.ParseIP(r)
			if ip == nil {
				return nil, errors.New("bad stub IP: " + r)
			}
			out = append(out, ip)
		}
		return out, nil
	}
}

func TestIsBlocked_IPLiteralsBlocked(t *testing.T) {
	cases := []struct {
		ip      string
		want    bool
		reason  string // substring expected in reason when blocked
	}{
		// Cloud metadata IPs — all four major clouds we registered.
		{"169.254.169.254", true, "metadata"},
		{"100.100.100.200", true, "metadata"},
		{"192.0.0.192", true, "metadata"},
		{"fd00:ec2::254", true, "metadata"},
		{"fe80::a9fe:a9fe", true, "metadata"},

		// RFC 1918 private.
		{"10.0.0.1", true, "private"},
		{"172.16.0.1", true, "private"},
		{"172.31.255.254", true, "private"},
		{"192.168.1.1", true, "private"},

		// Loopback / unspecified.
		{"127.0.0.1", true, "private"},
		{"0.0.0.0", true, "private"},
		{"::", true, "private"},
		{"::1", true, "private"},

		// Link-local (besides metadata).
		{"169.254.42.42", true, "private"},
		{"fe80::1", true, "private"},

		// CGNAT (besides Alibaba metadata).
		{"100.64.0.1", true, "private"},
		{"100.127.255.255", true, "private"},

		// IPv6 ULA.
		{"fc00::1", true, "private"},
		{"fd12:3456::1", true, "private"},

		// Multicast / broadcast / reserved.
		{"224.0.0.1", true, "private"},
		{"239.0.0.1", true, "private"},
		{"255.255.255.255", true, "private"},
		{"240.0.0.1", true, "private"},

		// Public addresses MUST NOT be blocked.
		{"1.1.1.1", false, ""},
		{"8.8.8.8", false, ""},
		{"93.184.216.34", false, ""},
		{"2606:4700:4700::1111", false, ""},
		{"2001:4860:4860::8888", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			blocked, reason := netfilter.IsBlocked(tc.ip)
			if blocked != tc.want {
				t.Fatalf("IsBlocked(%q) = (%v, %q), want blocked=%v", tc.ip, blocked, reason, tc.want)
			}
			if tc.want && tc.reason != "" && !strings.Contains(reason, tc.reason) {
				t.Errorf("reason %q does not contain %q", reason, tc.reason)
			}
		})
	}
}

func TestIsBlocked_BracketedIPv6(t *testing.T) {
	// Some callers pass bare bracketed IPv6 like "[::1]"; we should still block it.
	blocked, reason := netfilter.IsBlocked("[::1]")
	if !blocked {
		t.Fatalf("expected [::1] to be blocked, got reason=%q", reason)
	}
}

func TestIsBlocked_EmptyHost(t *testing.T) {
	blocked, reason := netfilter.IsBlocked("")
	if !blocked {
		t.Fatal("expected empty host to be blocked")
	}
	if !strings.Contains(reason, "empty") {
		t.Errorf("reason %q should mention empty host", reason)
	}

	blocked, _ = netfilter.IsBlocked("   ")
	if !blocked {
		t.Fatal("expected whitespace-only host to be blocked")
	}
}

func TestIsBlockedWith_HostnameResolution(t *testing.T) {
	cfg := netfilter.Config{
		Resolver: stubResolver(map[string][]string{
			"metadata.google.internal": {"169.254.169.254"},
			"private.example":          {"10.0.0.5"},
			"public.example":           {"1.1.1.1", "8.8.8.8"},
			"mixed.example":            {"1.1.1.1", "10.0.0.5"}, // public + private => block
			"v6private.example":        {"fc00::1"},
			"v6public.example":         {"2606:4700:4700::1111"},
		}, nil),
	}

	cases := []struct {
		host    string
		want    bool
		needles []string
	}{
		{"metadata.google.internal", true, []string{"metadata", "169.254.169.254"}},
		{"private.example", true, []string{"private", "10.0.0.5"}},
		{"public.example", false, nil},
		{"mixed.example", true, []string{"10.0.0.5"}},
		{"v6private.example", true, []string{"private"}},
		{"v6public.example", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			blocked, reason := netfilter.IsBlockedWith(tc.host, cfg)
			if blocked != tc.want {
				t.Fatalf("IsBlockedWith(%q) = (%v, %q), want %v", tc.host, blocked, reason, tc.want)
			}
			for _, n := range tc.needles {
				if !strings.Contains(reason, n) {
					t.Errorf("reason %q missing substring %q", reason, n)
				}
			}
		})
	}
}

func TestIsBlockedWith_UnresolvableHostBlocked(t *testing.T) {
	cfg := netfilter.Config{
		Resolver: stubResolver(nil, map[string]error{
			"nx.example": errors.New("no such host"),
		}),
	}
	blocked, reason := netfilter.IsBlockedWith("nx.example", cfg)
	if !blocked {
		t.Fatalf("unresolvable host should be blocked, got reason=%q", reason)
	}
	if !strings.Contains(reason, "unresolvable") {
		t.Errorf("reason %q should mention 'unresolvable'", reason)
	}
}

func TestIsBlockedWith_EmptyResolverResultBlocked(t *testing.T) {
	cfg := netfilter.Config{
		Resolver: func(host string) ([]net.IP, error) { return nil, nil },
	}
	blocked, reason := netfilter.IsBlockedWith("anything.example", cfg)
	if !blocked {
		t.Fatal("empty resolver result should be blocked")
	}
	if !strings.Contains(reason, "no addresses") {
		t.Errorf("reason %q should mention 'no addresses'", reason)
	}
}

func TestIsBlockedWith_AllowListBypass(t *testing.T) {
	// Allow-list lets a specific CIDR through even if it would otherwise be private.
	_, allow, err := net.ParseCIDR("10.5.5.0/24")
	if err != nil {
		t.Fatal(err)
	}
	cfg := netfilter.Config{
		AllowList: []*net.IPNet{allow},
		Resolver: stubResolver(map[string][]string{
			"speedtest.internal":   {"10.5.5.10"}, // allowed by CIDR
			"other-private.example": {"10.6.0.1"}, // outside allow-list
		}, nil),
	}

	// IP-literal path: allow-list bypass for an explicit IP.
	blocked, _ := netfilter.IsBlockedWith("10.5.5.10", cfg)
	if blocked {
		t.Errorf("allow-listed IP 10.5.5.10 should not be blocked")
	}

	// Hostname path.
	blocked, _ = netfilter.IsBlockedWith("speedtest.internal", cfg)
	if blocked {
		t.Errorf("hostname resolving to allow-listed IP should not be blocked")
	}

	// Same allow-list does NOT cover an unrelated private host.
	blocked, reason := netfilter.IsBlockedWith("other-private.example", cfg)
	if !blocked {
		t.Errorf("private host outside allow-list should be blocked, reason=%q", reason)
	}

	// Allow-list cannot bypass metadata-IP if the IP is outside the allow CIDR.
	blocked, _ = netfilter.IsBlockedWith("169.254.169.254", cfg)
	if !blocked {
		t.Errorf("metadata IP outside allow-list should still be blocked")
	}
}

func TestIsBlockedWith_AllowListCoversMetadata(t *testing.T) {
	// Edge case: operator explicitly allows the metadata IP. We honour it
	// (allow-list is the operator's escape hatch), but the test documents
	// the behaviour so a future change is intentional.
	_, allow, err := net.ParseCIDR("169.254.169.254/32")
	if err != nil {
		t.Fatal(err)
	}
	cfg := netfilter.Config{AllowList: []*net.IPNet{allow}}
	blocked, reason := netfilter.IsBlockedWith("169.254.169.254", cfg)
	if blocked {
		t.Errorf("explicit allow-list entry should bypass even metadata IP, reason=%q", reason)
	}
}

func TestParseAllowList(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		nets, err := netfilter.ParseAllowList([]string{
			"10.0.0.0/8",
			"  192.168.1.0/24  ",
			"",
			"2001:db8::/32",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(nets) != 3 {
			t.Fatalf("got %d nets, want 3", len(nets))
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := netfilter.ParseAllowList([]string{"not-a-cidr"})
		if err == nil {
			t.Fatal("expected error for invalid CIDR")
		}
		if !strings.Contains(err.Error(), "invalid CIDR") {
			t.Errorf("error %v should mention 'invalid CIDR'", err)
		}
	})
}

func TestSetGlobalAndReset(t *testing.T) {
	// Save/restore so the test is hermetic against other tests in the package.
	defer netfilter.ResetGlobal()

	calls := 0
	netfilter.SetGlobal(netfilter.Config{
		Resolver: func(host string) ([]net.IP, error) {
			calls++
			return []net.IP{net.ParseIP("1.1.1.1")}, nil
		},
	})

	blocked, _ := netfilter.IsBlocked("public.example")
	if blocked {
		t.Errorf("expected stub-resolved public.example to NOT be blocked")
	}
	if calls != 1 {
		t.Errorf("stub resolver was called %d times, want 1", calls)
	}

	// After ResetGlobal, the default resolver is in place; IP-literal path
	// still works without DNS.
	netfilter.ResetGlobal()
	blocked, _ = netfilter.IsBlocked("169.254.169.254")
	if !blocked {
		t.Error("after reset, metadata IP should still be blocked")
	}
}

func TestSetGlobalNilResolverFallsBack(t *testing.T) {
	defer netfilter.ResetGlobal()
	// Passing a config without a Resolver should not panic; the package must
	// substitute the default resolver.
	netfilter.SetGlobal(netfilter.Config{})
	blocked, _ := netfilter.IsBlocked("8.8.8.8")
	if blocked {
		t.Errorf("public IP 8.8.8.8 should not be blocked under default config")
	}
}

func TestMetadataCIDRs_Snapshot(t *testing.T) {
	got := netfilter.MetadataCIDRs()
	if len(got) == 0 {
		t.Fatal("MetadataCIDRs returned empty list")
	}
	// Mutating the returned slice must not affect the package state.
	got[0] = "0.0.0.0/0"
	again := netfilter.MetadataCIDRs()
	if again[0] == "0.0.0.0/0" {
		t.Error("MetadataCIDRs returned a non-defensive copy")
	}
	// AWS IMDS must be in the list.
	found := false
	for _, c := range again {
		if c == "169.254.169.254/32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AWS IMDS 169.254.169.254/32 missing from MetadataCIDRs")
	}
}

func TestIsBlocked_InvalidHostnameFormat(t *testing.T) {
	// The default resolver will fail to resolve nonsense; we just confirm the
	// behaviour is "blocked", not "panic".
	blocked, _ := netfilter.IsBlocked("!!!not a hostname!!!")
	if !blocked {
		t.Error("malformed hostname should be blocked (unresolvable)")
	}
}
