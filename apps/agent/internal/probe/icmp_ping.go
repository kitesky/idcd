// Package probe — ICMPPingSender provides real ICMP Echo-based ping.
// Requires CAP_NET_RAW (systemd service is pre-configured with AmbientCapabilities=CAP_NET_RAW).
// Falls back silently to SimplePingSender when raw socket permission is denied.
package probe

import (
	"fmt"
	"math"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// ICMPPingSender sends real ICMP Echo requests.
// It implements the PingSender interface.
type ICMPPingSender struct{}

// SendPing sends ICMP Echo requests to target and returns statistics.
func (s *ICMPPingSender) SendPing(target string, timeout time.Duration, count int) (PingStats, error) {
	// Resolve to IP address
	addrs, err := net.LookupHost(target)
	if err != nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("resolve %q: %w", target, err)
	}
	if len(addrs) == 0 {
		return PingStats{PacketsSent: count}, fmt.Errorf("no addresses for %q", target)
	}

	ip := net.ParseIP(addrs[0])
	if ip == nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("invalid IP %q", addrs[0])
	}

	if ip.To4() != nil {
		return pingIPv4(ip, timeout, count)
	}
	return pingIPv6(ip, timeout, count)
}

// pingIPv4 sends ICMP Echo Requests using IPv4 raw socket.
func pingIPv4(ip net.IP, timeout time.Duration, count int) (PingStats, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("open raw socket: %w", err)
	}
	defer conn.Close()

	pid := os.Getpid() & 0xffff
	var rtts []time.Duration
	sent := 0
	received := 0

	perTimeout := timeout / time.Duration(count)

	for seq := 0; seq < count; seq++ {
		msg := &icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   pid,
				Seq:  seq + 1,
				Data: []byte("idcd-probe"),
			},
		}
		b, err := msg.Marshal(nil)
		if err != nil {
			sent++
			continue
		}

		start := time.Now()
		sent++

		if _, err := conn.WriteTo(b, &net.IPAddr{IP: ip}); err != nil {
			continue
		}

		conn.SetReadDeadline(time.Now().Add(perTimeout))
		reply := make([]byte, 1500)
		n, _, err := conn.ReadFrom(reply)
		if err != nil {
			continue
		}

		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), reply[:n])
		if err != nil {
			continue
		}
		if rm.Type == ipv4.ICMPTypeEchoReply {
			if echo, ok := rm.Body.(*icmp.Echo); ok && echo.ID == pid {
				rtt := time.Since(start)
				rtts = append(rtts, rtt)
				received++
			}
		}
	}

	return buildPingStats(sent, received, rtts), nil
}

// pingIPv6 sends ICMPv6 Echo Requests.
func pingIPv6(ip net.IP, timeout time.Duration, count int) (PingStats, error) {
	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("open raw ipv6 socket: %w", err)
	}
	defer conn.Close()

	pid := os.Getpid() & 0xffff
	var rtts []time.Duration
	sent := 0
	received := 0

	perTimeout := timeout / time.Duration(count)

	for seq := 0; seq < count; seq++ {
		msg := &icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
			Code: 0,
			Body: &icmp.Echo{
				ID:   pid,
				Seq:  seq + 1,
				Data: []byte("idcd-probe"),
			},
		}
		b, err := msg.Marshal(nil)
		if err != nil {
			sent++
			continue
		}

		start := time.Now()
		sent++

		if _, err := conn.WriteTo(b, &net.IPAddr{IP: ip}); err != nil {
			continue
		}

		conn.SetReadDeadline(time.Now().Add(perTimeout))
		reply := make([]byte, 1500)
		n, _, err := conn.ReadFrom(reply)
		if err != nil {
			continue
		}

		rm, err := icmp.ParseMessage(ipv6.ICMPTypeEchoReply.Protocol(), reply[:n])
		if err != nil {
			continue
		}
		if rm.Type == ipv6.ICMPTypeEchoReply {
			if echo, ok := rm.Body.(*icmp.Echo); ok && echo.ID == pid {
				rtt := time.Since(start)
				rtts = append(rtts, rtt)
				received++
			}
		}
	}

	return buildPingStats(sent, received, rtts), nil
}

// buildPingStats computes aggregate statistics from individual RTT samples.
func buildPingStats(sent, received int, rtts []time.Duration) PingStats {
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

	stats.MinRTT = rtts[0]
	stats.MaxRTT = rtts[0]
	var sum time.Duration
	for _, r := range rtts {
		if r < stats.MinRTT {
			stats.MinRTT = r
		}
		if r > stats.MaxRTT {
			stats.MaxRTT = r
		}
		sum += r
	}
	stats.AvgRTT = sum / time.Duration(len(rtts))

	if len(rtts) > 1 {
		avgMs := stats.AvgRTT.Seconds() * 1000
		var variance float64
		for _, r := range rtts {
			d := r.Seconds()*1000 - avgMs
			variance += d * d
		}
		variance /= float64(len(rtts))
		stats.StdDevRTT = time.Duration(math.Sqrt(variance)) * time.Millisecond
	}
	return stats
}
