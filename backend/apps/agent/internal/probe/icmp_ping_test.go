package probe

import (
	"errors"
	"os"
	"testing"
	"time"
)

// buildPingStats is pure computation — fully testable.

func TestBuildPingStats_Empty(t *testing.T) {
	s := buildPingStats(5, 0, nil)
	if s.PacketsSent != 5 {
		t.Errorf("PacketsSent want 5, got %d", s.PacketsSent)
	}
	if s.PacketsReceived != 0 {
		t.Errorf("PacketsReceived want 0, got %d", s.PacketsReceived)
	}
	if s.PacketLoss != 100.0 {
		t.Errorf("PacketLoss want 100, got %f", s.PacketLoss)
	}
	if s.MinRTT != 0 || s.AvgRTT != 0 || s.MaxRTT != 0 {
		t.Error("RTTs should be zero when no replies received")
	}
}

func TestBuildPingStats_AllReceived(t *testing.T) {
	rtts := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond}
	s := buildPingStats(3, 3, rtts)

	if s.PacketLoss != 0 {
		t.Errorf("PacketLoss want 0, got %f", s.PacketLoss)
	}
	if s.MinRTT != 10*time.Millisecond {
		t.Errorf("MinRTT want 10ms, got %v", s.MinRTT)
	}
	if s.MaxRTT != 30*time.Millisecond {
		t.Errorf("MaxRTT want 30ms, got %v", s.MaxRTT)
	}
	if s.AvgRTT != 20*time.Millisecond {
		t.Errorf("AvgRTT want 20ms, got %v", s.AvgRTT)
	}
}

func TestBuildPingStats_PartialLoss(t *testing.T) {
	rtts := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	s := buildPingStats(4, 2, rtts)
	if s.PacketLoss != 50.0 {
		t.Errorf("PacketLoss want 50, got %f", s.PacketLoss)
	}
}

func TestBuildPingStats_StdDev(t *testing.T) {
	// Two equal RTTs → stddev = 0
	rtts := []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	s := buildPingStats(2, 2, rtts)
	if s.StdDevRTT != 0 {
		t.Errorf("StdDev should be 0 for identical RTTs, got %v", s.StdDevRTT)
	}
}

func TestIsPermissionError_True(t *testing.T) {
	pe := &os.PathError{Err: os.ErrPermission}
	if !isPermissionError(pe) {
		t.Error("expected isPermissionError to return true for PathError with ErrPermission")
	}
}

func TestIsPermissionError_False(t *testing.T) {
	if isPermissionError(errors.New("some other error")) {
		t.Error("expected isPermissionError to return false for non-permission error")
	}
}

func TestICMPPingSender_FallbackOnNoPermission(t *testing.T) {
	// In the test environment (no CAP_NET_RAW), ICMPPingSender.SendPing
	// will either succeed (if running as root) or fail with a permission/listen error.
	// Either way it must not panic.
	s := &ICMPPingSender{}
	stats, err := s.SendPing("127.0.0.1", 200*time.Millisecond, 1)
	if err != nil {
		// Permission denied is expected in most test environments — that's fine.
		t.Logf("SendPing returned error (expected in non-root env): %v", err)
		return
	}
	// If we got here, raw sockets are available (e.g. running as root or with caps).
	if stats.PacketsSent != 1 {
		t.Errorf("PacketsSent want 1, got %d", stats.PacketsSent)
	}
}
