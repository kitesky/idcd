package probe

import (
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"syscall"
	"time"
)

// SimplePingSender provides a simplified ping implementation without raw sockets
type SimplePingSender struct{}

// Execute performs a ping probe. Tries real ICMP first (needs CAP_NET_RAW);
// falls back to TCP-connect simulation when the raw socket is unavailable.
func (p *PingProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	if p.Sender == nil {
		// Try ICMP first; fall back to SimplePingSender if permission denied.
		icmpSender := &ICMPPingSender{}
		if _, err := icmpSender.SendPing("127.0.0.1", 500*time.Millisecond, 1); err != nil &&
			isPermissionError(err) {
			p.Sender = &SimplePingSender{}
		} else {
			p.Sender = icmpSender
		}
	}

	// Parse options
	count := getIntOption(options, "count", 5)

	// Perform ping
	stats, err := p.Sender.SendPing(target, timeout, count)

	data := map[string]any{
		"packets_sent":     stats.PacketsSent,
		"packets_received": stats.PacketsReceived,
		"packet_loss":      stats.PacketLoss,
	}

	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("ping failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	if stats.PacketsReceived > 0 {
		data["min_ms"] = stats.MinRTT.Milliseconds()
		data["avg_ms"] = stats.AvgRTT.Milliseconds()
		data["max_ms"] = stats.MaxRTT.Milliseconds()
		data["stddev_ms"] = stats.StdDevRTT.Milliseconds()
	}

	// Consider ping successful if we get any responses
	success := stats.PacketsReceived > 0

	return &Result{
		Success:    success,
		Data:       data,
		Timestamp:  start,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// SendPing sends simplified "ping" using TCP connections as a proxy
func (s *SimplePingSender) SendPing(target string, timeout time.Duration, count int) (PingStats, error) {
	var rtts []time.Duration
	packetsSent := count
	packetsReceived := 0

	// Try both common ports to increase success chance
	testPorts := []string{"80", "443", "53"}

	for i := 0; i < count; i++ {
		start := time.Now()
		success := false

		// Try connecting to different ports until one succeeds
		for _, port := range testPorts {
			address := net.JoinHostPort(target, port)
			conn, err := net.DialTimeout("tcp", address, timeout/time.Duration(len(testPorts)))
			if err == nil {
				conn.Close()
				rtt := time.Since(start)
				rtts = append(rtts, rtt)
				packetsReceived++
				success = true
				break
			}
		}

		// If none of the ports worked, still count the timing
		if !success {
			// Just for consistent timing
			time.Sleep(timeout / time.Duration(count))
		}

		// Sleep between attempts (except last one)
		if i < count-1 {
			time.Sleep(time.Second)
		}
	}

	return calculatePingStats(packetsSent, packetsReceived, rtts), nil
}

// calculatePingStats computes statistics from ping RTT measurements.
func calculatePingStats(sent, received int, rtts []time.Duration) PingStats {
	stats := PingStats{
		PacketsSent:     sent,
		PacketsReceived: received,
	}

	if sent > 0 {
		stats.PacketLoss = float64(sent-received) / float64(sent) * 100
	}

	if len(rtts) == 0 {
		return stats
	}

	// Calculate min, max, avg
	stats.MinRTT = rtts[0]
	stats.MaxRTT = rtts[0]
	var sum time.Duration
	for _, rtt := range rtts {
		if rtt < stats.MinRTT {
			stats.MinRTT = rtt
		}
		if rtt > stats.MaxRTT {
			stats.MaxRTT = rtt
		}
		sum += rtt
	}
	stats.AvgRTT = sum / time.Duration(len(rtts))

	// Calculate standard deviation
	if len(rtts) > 1 {
		var variance float64
		avgMs := stats.AvgRTT.Seconds() * 1000
		for _, rtt := range rtts {
			diff := rtt.Seconds()*1000 - avgMs
			variance += diff * diff
		}
		variance /= float64(len(rtts))
		stddevMs := math.Sqrt(variance)
		stats.StdDevRTT = time.Duration(stddevMs) * time.Millisecond
	}

	return stats
}

// isPermissionError returns true when the error is an OS permission denial,
// meaning raw socket access (CAP_NET_RAW on Linux, root on macOS) is
// unavailable. icmp.ListenPacket wraps the EPERM inside net.OpError →
// os.SyscallError → syscall.Errno, so we Unwrap the whole chain. The old
// version only matched *os.PathError and silently returned false on macOS,
// leaving the ICMP path active even though every Echo request would fail
// with "operation not permitted" — visible to users as packets_sent=N /
// packets_received=0 with an unhelpful "open raw socket" error.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		return true
	}
	var pe *os.PathError
	if errors.As(err, &pe) && os.IsPermission(pe.Err) {
		return true
	}
	var se *os.SyscallError
	if errors.As(err, &se) && os.IsPermission(se.Err) {
		return true
	}
	var ne *net.OpError
	if errors.As(err, &ne) && ne.Err != nil {
		return isPermissionError(ne.Err)
	}
	return false
}

func getIntOption(options map[string]any, key string, defaultValue int) int {
	if v, ok := options[key]; ok {
		if i, ok := v.(int); ok {
			return i
		}
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return defaultValue
}