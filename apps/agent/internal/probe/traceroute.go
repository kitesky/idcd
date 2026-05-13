package probe

import (
	"fmt"
	"net"
	"time"
)

const maxHops = 10 // Reduced for simplified implementation

// Execute performs a simplified traceroute probe.
func (p *TracerouteProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	// Resolve target IP
	targetIP, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("resolve target: %v", err),
			Data:       map[string]any{},
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	hops, err := p.performSimpleTraceroute(targetIP.IP, timeout)

	data := map[string]any{
		"target_ip": targetIP.IP.String(),
		"hops":      hops,
	}

	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("traceroute failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	// Consider traceroute successful if we got at least one hop
	success := len(hops) > 0

	return &Result{
		Success:    success,
		Data:       data,
		Timestamp:  start,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// performSimpleTraceroute executes a simplified traceroute without raw sockets
func (p *TracerouteProbe) performSimpleTraceroute(targetIP net.IP, timeout time.Duration) ([]TracerouteHop, error) {
	var hops []TracerouteHop

	// This is a very simplified traceroute implementation
	// In a real implementation, we would need raw sockets and proper ICMP handling
	// For now, we'll just attempt to connect to the target directly

	hop := TracerouteHop{
		Hop:     1,
		IP:      targetIP.String(),
		Timeout: true,
	}

	start := time.Now()

	// Try to connect to common ports to test reachability
	testPorts := []string{"80", "443", "53"}
	for _, port := range testPorts {
		address := net.JoinHostPort(targetIP.String(), port)
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			conn.Close()
			hop.RTT = time.Since(start)
			hop.Timeout = false
			break
		}
	}

	// Try to resolve hostname
	if names, err := net.LookupAddr(targetIP.String()); err == nil && len(names) > 0 {
		hop.Hostname = names[0]
	}

	hops = append(hops, hop)

	return hops, nil
}