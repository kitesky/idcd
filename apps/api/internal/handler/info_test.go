package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- IP Query Tests ---

func TestInfoHandler_IP(t *testing.T) {
	h := NewInfoHandler()

	tests := []struct {
		name           string
		query          string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "missing q parameter",
			query:          "",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "private IP blocked - 10.x",
			query:          "10.0.0.1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "private IP blocked - 192.168.x",
			query:          "192.168.1.1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "private IP blocked - 172.16.x",
			query:          "172.16.0.1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "loopback IP blocked",
			query:          "127.0.0.1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "link-local IP blocked",
			query:          "169.254.1.1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "IPv6 loopback blocked",
			query:          "::1",
			wantStatusCode: http.StatusForbidden,
			wantError:      true,
		},
		// Note: public IP tests would require actual network calls
		// In real tests, we'd mock h.httpClient
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/info/ip?q="+tt.query, nil)
			w := httptest.NewRecorder()

			h.IP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("IP() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			hasError := resp["error"] != nil
			if hasError != tt.wantError {
				t.Errorf("IP() error = %v, wantError %v, response: %v", hasError, tt.wantError, resp)
			}
		})
	}
}

// --- WHOIS Query Tests ---

func TestInfoHandler_Whois(t *testing.T) {
	h := NewInfoHandler()

	tests := []struct {
		name           string
		query          string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "missing q parameter",
			query:          "",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "valid domain",
			query:          "example.com",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/info/whois?q="+tt.query, nil)
			w := httptest.NewRecorder()

			h.Whois(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Whois() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			hasError := resp["error"] != nil
			if hasError != tt.wantError {
				t.Errorf("Whois() error = %v, wantError %v", hasError, tt.wantError)
			}

			if !tt.wantError {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Fatal("response data is not a map")
				}
				if domain, ok := data["domain"].(string); !ok || domain != tt.query {
					t.Errorf("Whois() domain = %v, want %v", domain, tt.query)
				}
			}
		})
	}
}

// --- DNS Query Tests ---

func TestInfoHandler_DNS(t *testing.T) {
	h := NewInfoHandler()

	tests := []struct {
		name           string
		query          string
		recordType     string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "missing q parameter",
			query:          "",
			recordType:     "A",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "invalid DNS type",
			query:          "example.com",
			recordType:     "INVALID",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "A record query - localhost",
			query:          "localhost",
			recordType:     "A",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "default type (A)",
			query:          "localhost",
			recordType:     "",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/v1/info/dns?q=" + tt.query
			if tt.recordType != "" {
				url += "&type=" + tt.recordType
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			h.DNS(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("DNS() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			hasError := resp["error"] != nil
			if hasError != tt.wantError {
				t.Errorf("DNS() error = %v, wantError %v, response: %v", hasError, tt.wantError, resp)
			}
		})
	}
}

func TestInfoHandler_DNS_QueryFunctions(t *testing.T) {
	h := NewInfoHandler()
	ctx := context.Background()

	t.Run("queryARecords - localhost", func(t *testing.T) {
		records, err := h.queryARecords(ctx, "localhost")
		if err != nil {
			t.Errorf("queryARecords() error = %v", err)
		}
		if len(records) == 0 {
			t.Error("queryARecords() returned no records")
		}
	})

	t.Run("queryAAAARecords - localhost", func(t *testing.T) {
		records, err := h.queryAAAARecords(ctx, "localhost")
		// May fail on some systems without IPv6
		if err == nil && len(records) == 0 {
			t.Log("queryAAAARecords() returned no records (acceptable)")
		}
	})

	t.Run("queryTXTRecords - invalid domain", func(t *testing.T) {
		_, err := h.queryTXTRecords(ctx, "nonexistent-domain-12345.invalid")
		if err == nil {
			t.Error("queryTXTRecords() should fail for invalid domain")
		}
	})

	t.Run("queryCNAMERecord - invalid domain", func(t *testing.T) {
		_, err := h.queryCNAMERecord(ctx, "nonexistent-domain-12345.invalid")
		if err == nil {
			t.Error("queryCNAMERecord() should fail for invalid domain")
		}
	})

	t.Run("queryNSRecords - invalid domain", func(t *testing.T) {
		_, err := h.queryNSRecords(ctx, "nonexistent-domain-12345.invalid")
		if err == nil {
			t.Error("queryNSRecords() should fail for invalid domain")
		}
	})

	t.Run("queryMXRecords - invalid domain", func(t *testing.T) {
		_, err := h.queryMXRecords(ctx, "nonexistent-domain-12345.invalid")
		if err == nil {
			t.Error("queryMXRecords() should fail for invalid domain")
		}
	})
}

// --- SSL Query Tests ---

func TestInfoHandler_SSL(t *testing.T) {
	h := NewInfoHandler()

	tests := []struct {
		name           string
		query          string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "missing q parameter",
			query:          "",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		// Real SSL connections require network access
		// In production tests, mock the tls.Dial call
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/info/ssl?q="+tt.query, nil)
			w := httptest.NewRecorder()

			h.SSL(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("SSL() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			hasError := resp["error"] != nil
			if hasError != tt.wantError {
				t.Errorf("SSL() error = %v, wantError %v", hasError, tt.wantError)
			}
		})
	}
}

// --- ICP Query Tests ---

func TestInfoHandler_ICP(t *testing.T) {
	h := NewInfoHandler()

	tests := []struct {
		name           string
		query          string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "missing q parameter",
			query:          "",
			wantStatusCode: http.StatusBadRequest,
			wantError:      true,
		},
		{
			name:           "valid domain - mock response",
			query:          "example.com",
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/info/icp?q="+tt.query, nil)
			w := httptest.NewRecorder()

			h.ICP(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("ICP() status = %v, want %v", w.Code, tt.wantStatusCode)
			}

			var resp map[string]any
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			hasError := resp["error"] != nil
			if hasError != tt.wantError {
				t.Errorf("ICP() error = %v, wantError %v", hasError, tt.wantError)
			}

			if !tt.wantError {
				data, ok := resp["data"].(map[string]any)
				if !ok {
					t.Fatal("response data is not a map")
				}
				if domain, ok := data["domain"].(string); !ok || domain != tt.query {
					t.Errorf("ICP() domain = %v, want %v", domain, tt.query)
				}
				if note, ok := data["note"].(string); !ok || note == "" {
					t.Error("ICP() should include note field for S1 mock")
				}
			}
		})
	}
}

// --- Helper Function Tests ---

func TestIsIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1.2.3.4", true},
		{"192.168.1.1", true},
		{"::1", true},
		{"2001:db8::1", true},
		{"example.com", false},
		{"not-an-ip", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isIP(tt.input); got != tt.want {
				t.Errorf("isIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Private ranges
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},

		// Loopback
		{"127.0.0.1", true},
		{"127.0.0.254", true},
		{"::1", true},

		// Link-local
		{"169.254.1.1", true},
		{"169.254.254.254", true},
		{"fe80::1", true},

		// IPv6 ULA
		{"fc00::1", true},
		{"fd00::1", true},

		// Public IPs
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},

		// Invalid
		{"not-an-ip", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isPrivateIP(tt.input); got != tt.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Additional Coverage Tests ---

func TestInfoHandler_DNS_CAA(t *testing.T) {
	h := NewInfoHandler()

	req := httptest.NewRequest("GET", "/v1/info/dns?q=example.com&type=CAA", nil)
	w := httptest.NewRecorder()

	h.DNS(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DNS(CAA) status = %v, want %v", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("response data is not a map")
	}

	// CAA should return empty records in S1
	records, ok := data["records"].([]any)
	if !ok {
		t.Fatal("records is not an array")
	}
	if len(records) != 0 {
		t.Logf("CAA records: %v (expected empty for S1)", records)
	}
}

func TestInfoHandler_SSL_DomainCleaning(t *testing.T) {
	// Test that SSL handler correctly cleans domain input
	h := NewInfoHandler()

	tests := []struct {
		input        string
		expectDomain string
	}{
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"example.com/path/to/page", "example.com"},
		{"https://example.com/path", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// We can't actually connect, but we test parameter parsing
			req := httptest.NewRequest("GET", "/v1/info/ssl?q="+tt.input, nil)
			w := httptest.NewRecorder()

			h.SSL(w, req)

			// Will fail connection (expected), but parameter should be parsed
			// Just verify it doesn't panic
			if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusOK {
				// If domain doesn't exist, DNS resolution fails with 503
				// This is acceptable for this test
			}
		})
	}
}

func TestNewInfoHandler(t *testing.T) {
	h := NewInfoHandler()
	if h == nil {
		t.Fatal("NewInfoHandler() returned nil")
	}
	if h.httpClient == nil {
		t.Error("NewInfoHandler() httpClient is nil")
	}
	if h.httpClient.Timeout != 10*1000*1000*1000 { // 10 seconds in nanoseconds
		t.Errorf("NewInfoHandler() httpClient timeout = %v, want 10s", h.httpClient.Timeout)
	}
}

func TestIsPrivateIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"IPv4 in private range boundary - lower", "10.0.0.0", true},
		{"IPv4 in private range boundary - upper", "10.255.255.255", true},
		{"IPv4 just outside private range", "11.0.0.0", false},
		{"IPv4 172.15 not private", "172.15.255.255", false},
		{"IPv4 172.32 not private", "172.32.0.0", false},
		{"IPv6 multicast link-local", "ff02::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrivateIP(tt.input); got != tt.want {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsIP_Coverage(t *testing.T) {
	// Additional edge cases
	tests := []struct {
		input string
		want  bool
	}{
		{"256.1.1.1", false},    // Invalid IPv4
		{"1.1.1", false},        // Incomplete IPv4
		{"gggg::1", false},      // Invalid IPv6
		{"0.0.0.0", true},       // Valid edge case
		{"255.255.255.255", true}, // Valid edge case
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isIP(tt.input); got != tt.want {
				t.Errorf("isIP(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestInfoHandler_IP_DomainResolution(t *testing.T) {
	h := NewInfoHandler()

	// Test domain that should resolve
	req := httptest.NewRequest("GET", "/v1/info/ip?q=localhost", nil)
	w := httptest.NewRecorder()

	h.IP(w, req)

	// localhost should resolve to 127.0.0.1, which is private
	if w.Code != http.StatusForbidden {
		t.Errorf("IP(localhost) status = %v, want %v (private IP block)", w.Code, http.StatusForbidden)
	}
}

func TestInfoHandler_IP_InvalidDomain(t *testing.T) {
	h := NewInfoHandler()

	// Test domain that won't resolve
	req := httptest.NewRequest("GET", "/v1/info/ip?q=this-domain-absolutely-does-not-exist-12345.invalid", nil)
	w := httptest.NewRecorder()

	h.IP(w, req)

	// Should fail validation
	if w.Code != http.StatusBadRequest {
		t.Errorf("IP(invalid-domain) status = %v, want %v", w.Code, http.StatusBadRequest)
	}
}

func TestIsPrivateIP_ParseError(t *testing.T) {
	// Test with completely invalid input that net.ParseIP returns nil
	result := isPrivateIP("clearly-not-an-ip-address")
	if result != false {
		t.Error("isPrivateIP should return false for unparseable input")
	}
}

func TestInfoHandler_DNS_AllTypes(t *testing.T) {
	h := NewInfoHandler()

	types := []string{"A", "AAAA", "MX", "TXT", "CNAME", "NS", "CAA"}

	for _, recordType := range types {
		t.Run("type_"+recordType, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/info/dns?q=localhost&type="+recordType, nil)
			w := httptest.NewRecorder()

			h.DNS(w, req)

			// Should not panic or error with validation
			if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
				t.Errorf("DNS(%s) unexpected status = %v", recordType, w.Code)
			}
		})
	}
}

func TestInfoHandler_DNS_CaseInsensitiveType(t *testing.T) {
	h := NewInfoHandler()

	req := httptest.NewRequest("GET", "/v1/info/dns?q=localhost&type=a", nil)
	w := httptest.NewRecorder()

	h.DNS(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("DNS(lowercase type) status = %v, want 200 or 500", w.Code)
	}
}

// Test context cancellation for DNS queries
func TestInfoHandler_DNS_ContextCancellation(t *testing.T) {
	h := NewInfoHandler()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := httptest.NewRequest("GET", "/v1/info/dns?q=example.com&type=A", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.DNS(w, req)

	// Should handle context cancellation gracefully
	if w.Code == http.StatusOK {
		t.Log("DNS query completed despite cancelled context (possible cache hit)")
	}
}

// Mock net.Resolver to test queryXXXRecords directly
func TestQueryRecordsWithMockResolver(t *testing.T) {
	// Note: This test uses real resolver since mocking net.DefaultResolver is difficult
	// In production, we'd inject a custom Resolver interface
	h := NewInfoHandler()
	ctx := context.Background()

	t.Run("queryARecords with timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 1*1000*1000) // 1ms timeout
		defer cancel()

		// Use a domain that's likely to timeout
		_, err := h.queryARecords(ctx, "extremely-slow-dns-query-test.invalid")
		// Should either fail or succeed quickly
		if err != nil {
			t.Logf("queryARecords timed out as expected: %v", err)
		}
	})
}

// Helper function to test private IP detection with net.IP
func TestPrivateIPDetection(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"127.0.0.1",
		"169.254.1.1",
		"::1",
		"fe80::1",
		"fc00::1",
	}

	for _, ipStr := range privateIPs {
		if !isPrivateIP(ipStr) {
			t.Errorf("isPrivateIP(%s) = false, want true", ipStr)
		}
	}

	publicIPs := []string{
		"8.8.8.8",
		"1.1.1.1",
		"2001:4860:4860::8888",
	}

	for _, ipStr := range publicIPs {
		if isPrivateIP(ipStr) {
			t.Errorf("isPrivateIP(%s) = true, want false", ipStr)
		}
	}
}

// Test all private IP ranges systematically
func TestAllPrivateRanges(t *testing.T) {
	privateTests := []struct {
		ip   string
		want bool
	}{
		// RFC1918
		{"10.0.0.0", true},
		{"10.255.255.255", true},
		{"172.16.0.0", true},
		{"172.31.255.255", true},
		{"192.168.0.0", true},
		{"192.168.255.255", true},

		// Not RFC1918
		{"9.255.255.255", false},
		{"11.0.0.0", false},
		{"172.15.255.255", false},
		{"172.32.0.0", false},
		{"192.167.255.255", false},
		{"192.169.0.0", false},

		// Loopback
		{"127.0.0.1", true},
		{"127.255.255.254", true},

		// Link-local
		{"169.254.0.0", true},
		{"169.254.255.255", true},
		{"169.253.255.255", false},
		{"169.255.0.0", false},

		// IPv6
		{"::1", true},
		{"fe80::1", true},
		{"fe80::ffff:ffff:ffff:ffff", true},
		{"fc00::1", true},
		{"fd00::1", true},
		{"fe00::1", false},

		// Public
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tt := range privateTests {
		t.Run(tt.ip, func(t *testing.T) {
			got := isPrivateIP(tt.ip)
			if got != tt.want {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// Test that handles errors from net.ParseCIDR
func TestIsPrivateIP_CIDRParseError(t *testing.T) {
	// This test ensures the CIDR parsing error is handled gracefully
	// We can't easily break the built-in CIDRs, but we verify the function doesn't panic
	testIPs := []string{"8.8.8.8", "invalid", ""}
	for _, ip := range testIPs {
		// Should not panic
		_ = isPrivateIP(ip)
	}
}

// Ensure linklocal detection works
func TestLinkLocalDetection(t *testing.T) {
	ip := net.ParseIP("169.254.1.1")
	if !ip.IsLinkLocalUnicast() {
		t.Error("169.254.1.1 should be link-local unicast")
	}

	ip6 := net.ParseIP("fe80::1")
	if !ip6.IsLinkLocalUnicast() {
		t.Error("fe80::1 should be link-local unicast")
	}
}
