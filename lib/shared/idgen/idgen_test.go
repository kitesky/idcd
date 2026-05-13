package idgen_test

import (
	"strings"
	"testing"

	"github.com/kite365/idcd/lib/shared/idgen"
)

func TestNew_prefix(t *testing.T) {
	id := idgen.User()
	if !strings.HasPrefix(id, "u_") {
		t.Fatalf("expected u_ prefix, got %q", id)
	}
	if len(id) != len("u_")+12 {
		t.Fatalf("expected length %d, got %d", len("u_")+12, len(id))
	}
}

func TestNew_unique(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		id := idgen.User()
		if seen[id] {
			t.Fatalf("collision on %q", id)
		}
		seen[id] = true
	}
}

func TestAPISecret(t *testing.T) {
	secret, err := idgen.APISecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "idc_live_") {
		t.Fatalf("expected idc_live_ prefix, got %q", secret)
	}
	// idc_live_ (9 chars) + 64 hex chars
	if len(secret) != 9+64 {
		t.Fatalf("expected len %d, got %d", 9+64, len(secret))
	}
}

func TestAPIKeyPrefix(t *testing.T) {
	secret, _ := idgen.APISecret()
	prefix := idgen.APIKeyPrefix(secret)
	if !strings.HasPrefix(prefix, "idc_live_") {
		t.Fatalf("prefix should start with idc_live_, got %q", prefix)
	}
	if len(prefix) != 9+8 {
		t.Fatalf("expected len %d, got %d", 9+8, len(prefix))
	}
}

func TestNode(t *testing.T) {
	id := idgen.Node("jp", "tk", 1, "vultr")
	if id != "nd_jp_tk_01_vultr" {
		t.Fatalf("unexpected node ID: %q", id)
	}
}

func TestAPIKeyPrefix_short(t *testing.T) {
	// Input shorter than expected — must return the input unchanged (no panic)
	got := idgen.APIKeyPrefix("short")
	if got != "short" {
		t.Errorf("unexpected result for short input: %q", got)
	}
}

type prefixGen struct {
	fn     func() string
	prefix string
}

func checkPrefixes(t *testing.T, gens []prefixGen) {
	t.Helper()
	for _, g := range gens {
		id := g.fn()
		if !strings.HasPrefix(id, g.prefix) {
			t.Errorf("expected prefix %q, got %q", g.prefix, id)
		}
	}
}

// TestAllIDPrefixes smoke-tests every entity ID generator (S1 + S2/S3) to
// ensure each returns the correct prefix. All delegate to New(), which is
// stress-tested in TestNew_unique; this test exists for line coverage.
func TestAllIDPrefixes(t *testing.T) {
	checkPrefixes(t, []prefixGen{
		// S1 entities
		{idgen.Team, "t_"},
		{idgen.APIKey, "ak_"},
		{idgen.Session, "s_"},
		{idgen.Monitor, "m_"},
		{idgen.MonitorCheck, "mc_"},
		{idgen.AlertEvent, "ae_"},
		{idgen.AlertPolicy, "ap_"},
		{idgen.Channel, "ch_"},
		{idgen.StatusPage, "sp_"},
		{idgen.StatusComponent, "sc_"},
		{idgen.StatusIncident, "inc_"},
		{idgen.ProbeTask, "pt_"},
		{idgen.Report, "r_"},
		{idgen.Order, "ord_"},
		{idgen.Invoice, "inv_"},
		{idgen.Subscription, "sub_"},
		{idgen.PaymentMethod, "pm_"},
		{idgen.Refund, "rf_"},
		{idgen.Ticket, "tk_"},
		{idgen.AuditLog, "al_"},
		{idgen.WebhookEndpoint, "we_"},
		{idgen.Dashboard, "db_"},
		{idgen.UserOTP, "otp_"},
		// S2/S3 entities
		{idgen.VerdictOrder, "v_"},
		{idgen.VerdictReport, "vr_"},
		{idgen.AttestationRecord, "att_"},
		{idgen.TSAResponse, "tsa_"},
		{idgen.KeyCeremonyLog, "kc_"},
		{idgen.MCPSession, "mcps_"},
		{idgen.MCPToolCall, "mctc_"},
		{idgen.MCPToken, "mcpt_"},
		{idgen.AgentObsMonitor, "aom_"},
		{idgen.AgentObsEvent, "aoe_"},
		{idgen.ComplianceSubscription, "cs_"},
	})
}
