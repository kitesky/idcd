package probe

import (
	"errors"
	"testing"
	"time"
)

// MockDNSResolver implements DNSResolver for testing
type MockDNSResolver struct {
	hostResults   []string
	mxResults     []MXRecord
	txtResults    []string
	cnameResult   string
	nsResults     []string
	shouldFailA   bool
	shouldFailMX  bool
	shouldFailTXT bool
	shouldFailCNAME bool
	shouldFailNS  bool
	err           error
}

func (m *MockDNSResolver) LookupHost(name string) ([]string, error) {
	if m.shouldFailA {
		return nil, m.err
	}
	return m.hostResults, nil
}

func (m *MockDNSResolver) LookupMX(name string) ([]MXRecord, error) {
	if m.shouldFailMX {
		return nil, m.err
	}
	return m.mxResults, nil
}

func (m *MockDNSResolver) LookupTXT(name string) ([]string, error) {
	if m.shouldFailTXT {
		return nil, m.err
	}
	return m.txtResults, nil
}

func (m *MockDNSResolver) LookupCNAME(name string) (string, error) {
	if m.shouldFailCNAME {
		return "", m.err
	}
	return m.cnameResult, nil
}

func (m *MockDNSResolver) LookupNS(name string) ([]string, error) {
	if m.shouldFailNS {
		return nil, m.err
	}
	return m.nsResults, nil
}

func TestDNSProbe_Execute(t *testing.T) {
	// Test successful DNS resolution
	t.Run("successful resolution", func(t *testing.T) {
		mockResolver := &MockDNSResolver{
			hostResults: []string{"192.168.1.1", "192.168.1.2"},
			mxResults: []MXRecord{
				{Host: "mail1.example.com", Priority: 10},
				{Host: "mail2.example.com", Priority: 20},
			},
			txtResults:  []string{"v=spf1 include:_spf.example.com ~all"},
			cnameResult: "www.example.com",
			nsResults:   []string{"ns1.example.com", "ns2.example.com"},
		}

		probe := &DNSProbe{Resolver: mockResolver}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		// Check resolve timing
		if result.Data["resolve_ms"] == nil {
			t.Error("Expected resolve_ms field")
		}

		// Check A records
		records, ok := result.Data["records"].(map[string]any)
		if !ok {
			t.Fatal("Expected records field to be map")
		}

		aRecords, ok := records["A"].([]string)
		if !ok {
			t.Fatal("Expected A records to be []string")
		}

		if len(aRecords) != 2 {
			t.Errorf("Expected 2 A records, got %d", len(aRecords))
		}

		if aRecords[0] != "192.168.1.1" || aRecords[1] != "192.168.1.2" {
			t.Errorf("Unexpected A record values: %v", aRecords)
		}

		// Check MX records
		mxRecords, ok := records["MX"].([]MXRecord)
		if !ok {
			t.Fatal("Expected MX records to be []MXRecord")
		}

		if len(mxRecords) != 2 {
			t.Errorf("Expected 2 MX records, got %d", len(mxRecords))
		}

		// Check TXT records
		txtRecords, ok := records["TXT"].([]string)
		if !ok {
			t.Fatal("Expected TXT records to be []string")
		}

		if len(txtRecords) != 1 || txtRecords[0] != "v=spf1 include:_spf.example.com ~all" {
			t.Errorf("Unexpected TXT record values: %v", txtRecords)
		}

		// Check CNAME record
		cnameRecord, ok := records["CNAME"].(string)
		if !ok {
			t.Fatal("Expected CNAME record to be string")
		}

		if cnameRecord != "www.example.com" {
			t.Errorf("Expected CNAME www.example.com, got %s", cnameRecord)
		}

		// Check NS records
		nsRecords, ok := records["NS"].([]string)
		if !ok {
			t.Fatal("Expected NS records to be []string")
		}

		if len(nsRecords) != 2 {
			t.Errorf("Expected 2 NS records, got %d", len(nsRecords))
		}
	})

	// Test DNS resolution failure
	t.Run("resolution failure", func(t *testing.T) {
		mockResolver := &MockDNSResolver{
			shouldFailA: true,
			err:         errors.New("domain not found"),
		}

		probe := &DNSProbe{Resolver: mockResolver}

		result := probe.Execute("nonexistent.example.com", 10*time.Second, map[string]any{})

		if result.Success {
			t.Error("Expected failure for DNS resolution error")
		}

		if result.Error == "" {
			t.Error("Expected error message for DNS failure")
		}
	})

	// Test partial DNS resolution success
	t.Run("partial resolution success", func(t *testing.T) {
		mockResolver := &MockDNSResolver{
			hostResults:   []string{"192.168.1.1"},
			shouldFailMX:  true,
			shouldFailTXT: true,
			err:           errors.New("record not found"),
		}

		probe := &DNSProbe{Resolver: mockResolver}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success despite partial failure, got: %s", result.Error)
		}

		// Should have A records but not MX/TXT
		records, ok := result.Data["records"].(map[string]any)
		if !ok {
			t.Fatal("Expected records field to be map")
		}

		if records["A"] == nil {
			t.Error("Expected A records to be present")
		}

		// MX and TXT should not be present due to lookup failures
		if records["MX"] != nil {
			t.Error("Expected MX records to be absent due to lookup failure")
		}

		if records["TXT"] != nil {
			t.Error("Expected TXT records to be absent due to lookup failure")
		}
	})

	// Test nil resolver (should create default)
	t.Run("nil resolver", func(t *testing.T) {
		probe := &DNSProbe{Resolver: nil}

		// This will use the real DNS resolver
		result := probe.Execute("example.com", 1*time.Second, map[string]any{})

		// Should not panic and should have basic data structure
		if result.Data == nil {
			t.Error("Expected data map even for DNS probe with real resolver")
		}

		// May succeed or fail depending on network, but shouldn't crash
		t.Logf("DNS probe result: success=%t, error=%s", result.Success, result.Error)
	})
}

func TestRealDNSResolver(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real DNS resolver test in short mode")
	}

	resolver := NewRealDNSResolver()

	// Test A record lookup
	t.Run("A record lookup", func(t *testing.T) {
		ips, err := resolver.LookupHost("example.com")
		if err != nil {
			t.Logf("Could not resolve example.com: %v", err)
		} else {
			t.Logf("Resolved example.com to: %v", ips)
			if len(ips) == 0 {
				t.Error("Expected at least one IP address")
			}
		}
	})

	// Test MX record lookup
	t.Run("MX record lookup", func(t *testing.T) {
		mxRecords, err := resolver.LookupMX("example.com")
		if err != nil {
			t.Logf("Could not get MX records for example.com: %v", err)
		} else {
			t.Logf("MX records for example.com: %v", mxRecords)
		}
	})

	// Test TXT record lookup
	t.Run("TXT record lookup", func(t *testing.T) {
		txtRecords, err := resolver.LookupTXT("example.com")
		if err != nil {
			t.Logf("Could not get TXT records for example.com: %v", err)
		} else {
			t.Logf("TXT records for example.com: %v", txtRecords)
		}
	})

	// Test CNAME record lookup
	t.Run("CNAME record lookup", func(t *testing.T) {
		cname, err := resolver.LookupCNAME("www.example.com")
		if err != nil {
			t.Logf("Could not get CNAME for www.example.com: %v", err)
		} else {
			t.Logf("CNAME for www.example.com: %s", cname)
		}
	})

	// Test NS record lookup
	t.Run("NS record lookup", func(t *testing.T) {
		nsRecords, err := resolver.LookupNS("example.com")
		if err != nil {
			t.Logf("Could not get NS records for example.com: %v", err)
		} else {
			t.Logf("NS records for example.com: %v", nsRecords)
		}
	})
}

func TestDNSProbe_ExecuteIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	probe := &DNSProbe{} // Will use real resolver

	// Test resolving a real domain
	t.Run("real domain", func(t *testing.T) {
		result := probe.Execute("google.com", 5*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Could not resolve google.com: %s", result.Error)
		} else {
			t.Logf("Successfully resolved google.com in %v ms", result.Data["resolve_ms"])

			records, ok := result.Data["records"].(map[string]any)
			if !ok {
				t.Fatal("Expected records field to be map")
			}

			aRecords, ok := records["A"].([]string)
			if !ok {
				t.Fatal("Expected A records to be []string")
			}

			if len(aRecords) == 0 {
				t.Error("Expected at least one A record for google.com")
			}

			t.Logf("A records for google.com: %v", aRecords)
		}
	})

	// Test resolving non-existent domain (integration, DNS resolver-dependent)
	t.Run("non-existent domain", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping DNS integration test in short mode")
		}
		result := probe.Execute("this-domain-should-not-exist-12345.invalid", 5*time.Second, map[string]any{})
		// Some resolvers may still respond; just verify the probe ran
		t.Logf("Non-existent domain result: success=%v error=%s", result.Success, result.Error)
	})
}