// Package probe — ICMPPingSender provides real ICMP Echo-based ping.
// Requires CAP_NET_RAW (systemd service is pre-configured with AmbientCapabilities=CAP_NET_RAW).
// Falls back silently to SimplePingSender when raw socket permission is denied.
package probe

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// randICMPID returns a random 16-bit ICMP Echo identifier.
// Using os.Getpid() is shared across all concurrent probes in the same process,
// causing reply cross-contamination. A random ID is unique per probe invocation.
func randICMPID() int {
	var b [2]byte
	_, _ = rand.Read(b[:])
	return int(binary.BigEndian.Uint16(b[:]))
}

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

// pingIPv4 sends ICMP Echo Requests using an IPv4 ICMP socket.
// listenICMP4 picks raw when available (production: setcap on the binary)
// and dgram otherwise (dev / hardened containers). isRaw tells us which
// path opened so we can match echo.id correctly per mode — see the inline
// comment on the EchoReply branch for why dgram intentionally skips it.
func pingIPv4(ip net.IP, timeout time.Duration, count int) (PingStats, error) {
	conn, writeAddr, isRaw, err := listenICMP4()
	if err != nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("open icmp socket: %w", err)
	}
	defer conn.Close()

	probeID := randICMPID()
	var rtts []time.Duration
	sent := 0
	received := 0

	perTimeout := timeout / time.Duration(count)

	for seq := 0; seq < count; seq++ {
		msg := &icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID:   probeID,
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

		if _, err := conn.WriteTo(b, writeAddr(ip)); err != nil {
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
		if rm.Type != ipv4.ICMPTypeEchoReply {
			continue
		}
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		// Raw socket: many processes can share raw reads, so echo.id is what
		// disambiguates our reply from someone else's.
		// Dgram socket: Linux's kernel rewrites echo.id on send to
		// sk->inet_sport and filters incoming replies on the same value, so
		// every packet landing on this socket IS ours by construction. But
		// userspace can only see the kernel-assigned value, never our
		// probeID, so an `echo.ID == probeID` check is guaranteed to fail.
		// Trusting kernel filtering here is what fixes the "packets_received=0"
		// regression — see M8 in docs/MCP-TEST-REPORT-2026-05-20.md.
		if isRaw && echo.ID != probeID {
			continue
		}
		rtts = append(rtts, time.Since(start))
		received++
	}

	return buildPingStats(sent, received, rtts), nil
}

// pingIPv6 sends ICMPv6 Echo Requests. See pingIPv4 for the raw/dgram echo.id
// rationale — same kernel behaviour applies to IPv6.
func pingIPv6(ip net.IP, timeout time.Duration, count int) (PingStats, error) {
	conn, writeAddr, isRaw, err := listenICMP6()
	if err != nil {
		return PingStats{PacketsSent: count}, fmt.Errorf("open icmp6 socket: %w", err)
	}
	defer conn.Close()

	probeID := randICMPID()
	var rtts []time.Duration
	sent := 0
	received := 0

	perTimeout := timeout / time.Duration(count)

	for seq := 0; seq < count; seq++ {
		msg := &icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
			Code: 0,
			Body: &icmp.Echo{
				ID:   probeID,
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

		if _, err := conn.WriteTo(b, writeAddr(ip)); err != nil {
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
		if rm.Type != ipv6.ICMPTypeEchoReply {
			continue
		}
		echo, ok := rm.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		if isRaw && echo.ID != probeID {
			continue
		}
		rtts = append(rtts, time.Since(start))
		received++
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
