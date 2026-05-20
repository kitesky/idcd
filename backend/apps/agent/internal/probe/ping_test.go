package probe

import (
	"errors"
	"testing"
	"time"
)

// MockPingSender implements PingSender for testing
type MockPingSender struct {
	shouldFail bool
	stats      PingStats
	err        error
}

func (m *MockPingSender) SendPing(target string, timeout time.Duration, count int) (PingStats, error) {
	if m.shouldFail {
		return PingStats{}, m.err
	}
	return m.stats, nil
}

func TestPingProbe_Execute(t *testing.T) {
	// Test successful ping
	t.Run("successful ping", func(t *testing.T) {
		mockStats := PingStats{
			PacketsSent:     5,
			PacketsReceived: 5,
			PacketLoss:      0.0,
			MinRTT:          10 * time.Millisecond,
			AvgRTT:          15 * time.Millisecond,
			MaxRTT:          20 * time.Millisecond,
			StdDevRTT:       3 * time.Millisecond,
		}

		probe := &PingProbe{
			Sender: &MockPingSender{
				shouldFail: false,
				stats:      mockStats,
			},
		}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["packets_sent"] != 5 {
			t.Errorf("Expected packets_sent 5, got %v", result.Data["packets_sent"])
		}

		if result.Data["packets_received"] != 5 {
			t.Errorf("Expected packets_received 5, got %v", result.Data["packets_received"])
		}

		if result.Data["packet_loss"] != 0.0 {
			t.Errorf("Expected packet_loss 0.0, got %v", result.Data["packet_loss"])
		}

		if result.Data["min_ms"] != int64(10) {
			t.Errorf("Expected min_ms 10, got %v", result.Data["min_ms"])
		}

		if result.Data["avg_ms"] != int64(15) {
			t.Errorf("Expected avg_ms 15, got %v", result.Data["avg_ms"])
		}

		if result.Data["max_ms"] != int64(20) {
			t.Errorf("Expected max_ms 20, got %v", result.Data["max_ms"])
		}

		if result.Data["stddev_ms"] != int64(3) {
			t.Errorf("Expected stddev_ms 3, got %v", result.Data["stddev_ms"])
		}
	})

	// Test ping with packet loss
	t.Run("ping with packet loss", func(t *testing.T) {
		mockStats := PingStats{
			PacketsSent:     5,
			PacketsReceived: 3,
			PacketLoss:      40.0,
			MinRTT:          10 * time.Millisecond,
			AvgRTT:          15 * time.Millisecond,
			MaxRTT:          20 * time.Millisecond,
			StdDevRTT:       3 * time.Millisecond,
		}

		probe := &PingProbe{
			Sender: &MockPingSender{
				shouldFail: false,
				stats:      mockStats,
			},
		}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if !result.Success {
			t.Errorf("Expected success despite packet loss, got failure: %s", result.Error)
		}

		if result.Data["packet_loss"] != 40.0 {
			t.Errorf("Expected packet_loss 40.0, got %v", result.Data["packet_loss"])
		}
	})

	// Test complete packet loss (should be considered failure)
	t.Run("complete packet loss", func(t *testing.T) {
		mockStats := PingStats{
			PacketsSent:     5,
			PacketsReceived: 0,
			PacketLoss:      100.0,
		}

		probe := &PingProbe{
			Sender: &MockPingSender{
				shouldFail: false,
				stats:      mockStats,
			},
		}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if result.Success {
			t.Error("Expected failure for complete packet loss")
		}

		if result.Data["packet_loss"] != 100.0 {
			t.Errorf("Expected packet_loss 100.0, got %v", result.Data["packet_loss"])
		}
	})

	// Test ping failure
	t.Run("ping failure", func(t *testing.T) {
		probe := &PingProbe{
			Sender: &MockPingSender{
				shouldFail: true,
				err:        errors.New("network unreachable"),
			},
		}

		result := probe.Execute("example.com", 10*time.Second, map[string]any{})

		if result.Success {
			t.Error("Expected failure for ping error")
		}

		if result.Error == "" {
			t.Error("Expected error message")
		}

		// Should still have packet count data
		if result.Data["packets_sent"] == nil {
			t.Error("Expected packets_sent field even for failed ping")
		}
	})

	// Test custom ping count
	t.Run("custom ping count", func(t *testing.T) {
		mockStats := PingStats{
			PacketsSent:     10,
			PacketsReceived: 8,
			PacketLoss:      20.0,
			MinRTT:          5 * time.Millisecond,
			AvgRTT:          10 * time.Millisecond,
			MaxRTT:          15 * time.Millisecond,
		}

		probe := &PingProbe{
			Sender: &MockPingSender{
				shouldFail: false,
				stats:      mockStats,
			},
		}

		options := map[string]any{
			"count": 10,
		}

		result := probe.Execute("example.com", 10*time.Second, options)

		if !result.Success {
			t.Errorf("Expected success, got failure: %s", result.Error)
		}

		if result.Data["packets_sent"] != 10 {
			t.Errorf("Expected packets_sent 10, got %v", result.Data["packets_sent"])
		}
	})

	// Test nil sender (should create default)
	t.Run("nil sender", func(t *testing.T) {
		probe := &PingProbe{
			Sender: nil, // Will create RealPingSender
		}

		// This will likely fail since RealPingSender needs privileges,
		// but we're testing that it doesn't panic
		result := probe.Execute("127.0.0.1", 1*time.Second, map[string]any{})

		// Should not panic and should have basic data structure
		if result.Data == nil {
			t.Error("Expected data map even for failed ping")
		}
	})
}

func TestCalculatePingStats(t *testing.T) {
	// Test with no RTT measurements
	t.Run("no measurements", func(t *testing.T) {
		stats := calculatePingStats(5, 0, []time.Duration{})

		if stats.PacketsSent != 5 {
			t.Errorf("Expected PacketsSent 5, got %d", stats.PacketsSent)
		}

		if stats.PacketsReceived != 0 {
			t.Errorf("Expected PacketsReceived 0, got %d", stats.PacketsReceived)
		}

		if stats.PacketLoss != 100.0 {
			t.Errorf("Expected PacketLoss 100.0, got %f", stats.PacketLoss)
		}

		if stats.MinRTT != 0 {
			t.Errorf("Expected MinRTT 0, got %v", stats.MinRTT)
		}
	})

	// Test with measurements
	t.Run("with measurements", func(t *testing.T) {
		rtts := []time.Duration{
			10 * time.Millisecond,
			15 * time.Millisecond,
			20 * time.Millisecond,
			5 * time.Millisecond,
		}

		stats := calculatePingStats(5, 4, rtts)

		if stats.PacketsSent != 5 {
			t.Errorf("Expected PacketsSent 5, got %d", stats.PacketsSent)
		}

		if stats.PacketsReceived != 4 {
			t.Errorf("Expected PacketsReceived 4, got %d", stats.PacketsReceived)
		}

		if stats.PacketLoss != 20.0 {
			t.Errorf("Expected PacketLoss 20.0, got %f", stats.PacketLoss)
		}

		if stats.MinRTT != 5*time.Millisecond {
			t.Errorf("Expected MinRTT 5ms, got %v", stats.MinRTT)
		}

		if stats.MaxRTT != 20*time.Millisecond {
			t.Errorf("Expected MaxRTT 20ms, got %v", stats.MaxRTT)
		}

		expectedAvgMs := float64(10+15+20+5) / 4 // 12.5ms
		expectedAvg := time.Duration(expectedAvgMs * float64(time.Millisecond))
		if stats.AvgRTT != expectedAvg {
			t.Errorf("Expected AvgRTT %.1fms, got %v", expectedAvgMs, stats.AvgRTT)
		}

		// Standard deviation should be calculated
		if stats.StdDevRTT == 0 {
			t.Error("Expected non-zero standard deviation")
		}
	})

	// Test with single measurement
	t.Run("single measurement", func(t *testing.T) {
		rtts := []time.Duration{15 * time.Millisecond}

		stats := calculatePingStats(1, 1, rtts)

		if stats.MinRTT != 15*time.Millisecond {
			t.Errorf("Expected MinRTT 15ms, got %v", stats.MinRTT)
		}

		if stats.MaxRTT != 15*time.Millisecond {
			t.Errorf("Expected MaxRTT 15ms, got %v", stats.MaxRTT)
		}

		if stats.AvgRTT != 15*time.Millisecond {
			t.Errorf("Expected AvgRTT 15ms, got %v", stats.AvgRTT)
		}

		if stats.PacketLoss != 0.0 {
			t.Errorf("Expected PacketLoss 0.0, got %f", stats.PacketLoss)
		}

		// StdDev should be 0 for single measurement
		if stats.StdDevRTT != 0 {
			t.Errorf("Expected StdDevRTT 0 for single measurement, got %v", stats.StdDevRTT)
		}
	})
}

func TestGetIntOption(t *testing.T) {
	options := map[string]any{
		"int_key":   42,
		"float_key": 3.14,
		"string_key": "not_a_number",
	}

	// Test existing int
	if got := getIntOption(options, "int_key", 0); got != 42 {
		t.Errorf("getIntOption(int_key) = %d, want 42", got)
	}

	// Test float conversion
	if got := getIntOption(options, "float_key", 0); got != 3 {
		t.Errorf("getIntOption(float_key) = %d, want 3", got)
	}

	// Test missing key (should return default)
	if got := getIntOption(options, "missing_key", 100); got != 100 {
		t.Errorf("getIntOption(missing_key) = %d, want 100", got)
	}

	// Test wrong type (should return default)
	if got := getIntOption(options, "string_key", 50); got != 50 {
		t.Errorf("getIntOption(string_key) = %d, want 50", got)
	}
}