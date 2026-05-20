package quota

import (
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Limits() — plan table sanity checks
// ─────────────────────────────────────────────────────────────────────────────

func TestLimits_KnownPlans(t *testing.T) {
	cases := []struct {
		plan            string
		wantMaxMonitors int
		wantMinInterval int
		wantMaxNodes    int
		wantMaxChannels int
		wantMaxAPI      int
	}{
		{"free", 3, 300, 1, 1, 100},
		{"pro", 50, 60, 5, 5, 5000},
		{"team", 200, 60, 10, 20, 50000},
		{"business", 0, 30, 0, 0, 0},
	}
	for _, tc := range cases {
		l := Limits(tc.plan)
		if l.MaxMonitors != tc.wantMaxMonitors {
			t.Errorf("plan=%s MaxMonitors: got %d want %d", tc.plan, l.MaxMonitors, tc.wantMaxMonitors)
		}
		if l.MinIntervalS != tc.wantMinInterval {
			t.Errorf("plan=%s MinIntervalS: got %d want %d", tc.plan, l.MinIntervalS, tc.wantMinInterval)
		}
		if l.MaxNodes != tc.wantMaxNodes {
			t.Errorf("plan=%s MaxNodes: got %d want %d", tc.plan, l.MaxNodes, tc.wantMaxNodes)
		}
		if l.MaxChannels != tc.wantMaxChannels {
			t.Errorf("plan=%s MaxChannels: got %d want %d", tc.plan, l.MaxChannels, tc.wantMaxChannels)
		}
		if l.MaxAPIDailyReqs != tc.wantMaxAPI {
			t.Errorf("plan=%s MaxAPIDailyReqs: got %d want %d", tc.plan, l.MaxAPIDailyReqs, tc.wantMaxAPI)
		}
	}
}

func TestLimits_UnknownPlanFallsBackToFree(t *testing.T) {
	l := Limits("enterprise_unknown")
	free := Limits("free")
	if l != free {
		t.Errorf("unknown plan should fall back to free limits, got %+v", l)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckMonitorCount
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckMonitorCount_Free_AtLimit(t *testing.T) {
	// 3 monitors already → adding 4th should fail
	if err := CheckMonitorCount("free", 3); err == nil {
		t.Error("expected quota error for free plan at limit=3, current=3")
	}
}

func TestCheckMonitorCount_Free_BelowLimit(t *testing.T) {
	// 2 monitors → can add 3rd
	if err := CheckMonitorCount("free", 2); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorCount_Free_Zero(t *testing.T) {
	if err := CheckMonitorCount("free", 0); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorCount_Pro_AtLimit(t *testing.T) {
	if err := CheckMonitorCount("pro", 50); err == nil {
		t.Error("expected quota error for pro plan at limit=50, current=50")
	}
}

func TestCheckMonitorCount_Pro_BelowLimit(t *testing.T) {
	if err := CheckMonitorCount("pro", 49); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorCount_Team_AtLimit(t *testing.T) {
	if err := CheckMonitorCount("team", 200); err == nil {
		t.Error("expected quota error for team plan at limit=200, current=200")
	}
}

func TestCheckMonitorCount_Team_BelowLimit(t *testing.T) {
	if err := CheckMonitorCount("team", 199); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorCount_Business_Unlimited(t *testing.T) {
	// business has MaxMonitors=0 → unlimited
	for _, n := range []int{0, 100, 1000, 999999} {
		if err := CheckMonitorCount("business", n); err != nil {
			t.Errorf("business plan should be unlimited, got error at count=%d: %v", n, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckMonitorInterval
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckMonitorInterval_Free_Valid(t *testing.T) {
	if err := CheckMonitorInterval("free", 300); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorInterval_Free_TooShort(t *testing.T) {
	if err := CheckMonitorInterval("free", 60); err == nil {
		t.Error("expected quota error: free plan min interval 300s, requested 60s")
	}
}

func TestCheckMonitorInterval_Free_LongerOK(t *testing.T) {
	if err := CheckMonitorInterval("free", 1800); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorInterval_Pro_60sOK(t *testing.T) {
	if err := CheckMonitorInterval("pro", 60); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorInterval_Pro_TooShort(t *testing.T) {
	if err := CheckMonitorInterval("pro", 30); err == nil {
		t.Error("expected quota error: pro plan min interval 60s, requested 30s")
	}
}

func TestCheckMonitorInterval_Business_30sOK(t *testing.T) {
	if err := CheckMonitorInterval("business", 30); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckMonitorInterval_Business_TooShort(t *testing.T) {
	if err := CheckMonitorInterval("business", 29); err == nil {
		t.Error("expected quota error: business plan min interval 30s, requested 29s")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckNodeCount
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckNodeCount_Free_AtLimit(t *testing.T) {
	if err := CheckNodeCount("free", 1); err != nil {
		t.Errorf("free plan allows exactly 1 node, got error: %v", err)
	}
}

func TestCheckNodeCount_Free_Over(t *testing.T) {
	if err := CheckNodeCount("free", 2); err == nil {
		t.Error("expected quota error: free plan max nodes=1, requested=2")
	}
}

func TestCheckNodeCount_Pro_AtLimit(t *testing.T) {
	if err := CheckNodeCount("pro", 5); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckNodeCount_Pro_Over(t *testing.T) {
	if err := CheckNodeCount("pro", 6); err == nil {
		t.Error("expected quota error: pro plan max nodes=5, requested=6")
	}
}

func TestCheckNodeCount_Business_Unlimited(t *testing.T) {
	for _, n := range []int{1, 10, 100} {
		if err := CheckNodeCount("business", n); err != nil {
			t.Errorf("business plan unlimited nodes, got error at n=%d: %v", n, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckChannelCount
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckChannelCount_Free_AtLimit(t *testing.T) {
	if err := CheckChannelCount("free", 1); err == nil {
		t.Error("expected quota error: free plan max channels=1, current=1")
	}
}

func TestCheckChannelCount_Free_BelowLimit(t *testing.T) {
	if err := CheckChannelCount("free", 0); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckChannelCount_Pro_AtLimit(t *testing.T) {
	if err := CheckChannelCount("pro", 5); err == nil {
		t.Error("expected quota error: pro plan max channels=5, current=5")
	}
}

func TestCheckChannelCount_Pro_BelowLimit(t *testing.T) {
	if err := CheckChannelCount("pro", 4); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckChannelCount_Business_Unlimited(t *testing.T) {
	for _, n := range []int{0, 50, 1000} {
		if err := CheckChannelCount("business", n); err != nil {
			t.Errorf("business plan unlimited channels, got error at n=%d: %v", n, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CheckStatusPageCount
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckStatusPageCount_Free_NoStatusPages(t *testing.T) {
	// free plan has 0 status pages allowed
	if err := CheckStatusPageCount("free", 0); err == nil {
		t.Error("expected quota error: free plan has no status pages")
	}
}

func TestCheckStatusPageCount_Pro_BelowLimit(t *testing.T) {
	if err := CheckStatusPageCount("pro", 2); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckStatusPageCount_Pro_AtLimit(t *testing.T) {
	if err := CheckStatusPageCount("pro", 3); err == nil {
		t.Error("expected quota error: pro plan max status pages=3, current=3")
	}
}

func TestCheckStatusPageCount_Team_BelowLimit(t *testing.T) {
	if err := CheckStatusPageCount("team", 9); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckStatusPageCount_Team_AtLimit(t *testing.T) {
	if err := CheckStatusPageCount("team", 10); err == nil {
		t.Error("expected quota error: team plan max status pages=10, current=10")
	}
}

func TestCheckStatusPageCount_Business_Unlimited(t *testing.T) {
	for _, n := range []int{0, 50, 1000} {
		if err := CheckStatusPageCount("business", n); err != nil {
			t.Errorf("business plan unlimited status pages, got error at n=%d: %v", n, err)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Error type assertions
// ─────────────────────────────────────────────────────────────────────────────

func TestCheckMonitorCount_ErrorCode(t *testing.T) {
	err := CheckMonitorCount("free", 3)
	if err == nil {
		t.Fatal("expected error")
	}
	// Just confirm error message is non-empty.
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}

func TestCheckMonitorCount_ErrorContainsPlanName(t *testing.T) {
	err := CheckMonitorCount("pro", 50)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
}

func TestCheckMonitorInterval_ErrorContainsInterval(t *testing.T) {
	err := CheckMonitorInterval("free", 60)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
}
