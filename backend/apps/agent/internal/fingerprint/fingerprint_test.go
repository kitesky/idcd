package fingerprint_test

import (
	"testing"

	"github.com/kite365/idcd/apps/agent/internal/fingerprint"
)

func TestCollect_ReturnsNonEmpty(t *testing.T) {
	fp, err := fingerprint.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if fp.Hostname == "" {
		t.Error("expected non-empty hostname")
	}
	if fp.OS == "" {
		t.Error("expected non-empty OS")
	}
	if fp.Arch == "" {
		t.Error("expected non-empty arch")
	}
}

func TestCollect_Deterministic(t *testing.T) {
	fp1, err := fingerprint.Collect()
	if err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	fp2, err := fingerprint.Collect()
	if err != nil {
		t.Fatalf("second Collect: %v", err)
	}
	if !fp1.Equal(fp2) {
		t.Errorf("Collect should return identical results on same machine: %s", fp1.Diff(fp2))
	}
}

func TestEqual_NilSafety(t *testing.T) {
	fp, _ := fingerprint.Collect()
	if fp.Equal(nil) {
		t.Error("non-nil fingerprint should not equal nil")
	}
	var nilFP *fingerprint.Fingerprint
	if nilFP.Equal(fp) {
		t.Error("nil fingerprint should not equal non-nil")
	}
	if !nilFP.Equal(nil) {
		t.Error("nil should equal nil")
	}
}

func TestDiff_DetectsChanges(t *testing.T) {
	base := &fingerprint.Fingerprint{
		Hostname: "server-1",
		OS:       "linux",
		Arch:     "amd64",
		Kernel:   "5.15.0",
		MAC:      "aa:bb:cc:dd:ee:ff",
		CPUModel: "Intel Xeon",
	}
	changed := &fingerprint.Fingerprint{
		Hostname: "server-2",        // changed
		OS:       "linux",
		Arch:     "amd64",
		Kernel:   "5.15.0",
		MAC:      "11:22:33:44:55:66", // changed
		CPUModel: "Intel Xeon",
	}
	diff := changed.Diff(base)
	if diff == "" {
		t.Error("expected non-empty diff for changed fingerprint")
	}
	// Both changes should be mentioned
	if !contains(diff, "hostname") {
		t.Errorf("diff should mention hostname change, got: %q", diff)
	}
	if !contains(diff, "mac") {
		t.Errorf("diff should mention mac change, got: %q", diff)
	}
}

func TestDiff_NoDiff(t *testing.T) {
	fp, _ := fingerprint.Collect()
	if d := fp.Diff(fp); d != "" {
		t.Errorf("identical fingerprints should have empty diff, got %q", d)
	}
}

func TestDiff_NilPrev(t *testing.T) {
	fp, _ := fingerprint.Collect()
	d := fp.Diff(nil)
	if d == "" {
		t.Error("diff against nil should return a descriptive message")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
