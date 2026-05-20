package constants_test

import (
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/constants"
)

// TestSLAConstants pins SLA durations so accidental edits surface as test failures.
// The numbers correspond to docs/prd/DECISIONS.md D12. Change DECISIONS.md before
// changing this file.
func TestSLAConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"VerdictAutoTimeout", constants.VerdictAutoTimeout, 0},
		{"VerdictCriticalP0SLA", constants.VerdictCriticalP0SLA, time.Hour},
		{"VerdictRoutineSLA", constants.VerdictRoutineSLA, 24 * time.Hour},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// TestTokenTTLConstants pins MCP token TTL ceilings (D2).
// D2 hard rule: no permanent tokens, 90d ceiling.
func TestTokenTTLConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"MCPTokenPersonalTTL", constants.MCPTokenPersonalTTL, 24 * time.Hour},
		{"MCPTokenWorkspaceTTL", constants.MCPTokenWorkspaceTTL, 90 * 24 * time.Hour},
		{"MCPTokenServiceTTL", constants.MCPTokenServiceTTL, 90 * 24 * time.Hour},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	// Sanity: no token may exceed the 90d ceiling.
	ceiling := 90 * 24 * time.Hour
	for _, d := range []time.Duration{
		constants.MCPTokenPersonalTTL,
		constants.MCPTokenWorkspaceTTL,
		constants.MCPTokenServiceTTL,
	} {
		if d > ceiling {
			t.Errorf("token TTL %v exceeds D2 ceiling %v", d, ceiling)
		}
	}
}

func TestRetentionConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"ProbeResultHotRetention", constants.ProbeResultHotRetention, 7 * 24 * time.Hour},
		{"ProbeResultColdRetention", constants.ProbeResultColdRetention, 90 * 24 * time.Hour},
		{"CertOrderRetention", constants.CertOrderRetention, 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	// Sanity: cold retention must outlive hot retention.
	if constants.ProbeResultColdRetention <= constants.ProbeResultHotRetention {
		t.Error("ProbeResultColdRetention must be strictly greater than ProbeResultHotRetention")
	}
}

func TestTimeoutConstants(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"WebAuthnChallengeTTL", constants.WebAuthnChallengeTTL, 5 * time.Minute},
		{"StreamConsumerClaimMinIdle", constants.StreamConsumerClaimMinIdle, 5 * time.Minute},
		{"MonitorFlapThreshold", constants.MonitorFlapThreshold, 5 * time.Minute},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
