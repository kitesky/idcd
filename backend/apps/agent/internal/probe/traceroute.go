package probe

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	maxTracerouteHops = 30
	probesPerHop      = 3
	hopTimeout        = 3 * time.Second
)

// Execute performs a traceroute probe.
// Uses ICMP sockets (raw preferred, dgram fallback — see listenICMP4) to run a
// real TTL-walk. Drops back to a single TCP reachability hop when no raw
// socket is available, because Linux unprivileged dgram ICMP cannot receive
// ICMP TimeExceeded (the kernel only delivers ICMP_ECHOREPLY through
// ping_rcv); walking TTL=1..30 in dgram mode would burn perHopTimeout *
// maxHops (~90s) and return all-timeout hops anyway.
func (p *TracerouteProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	maxHops := getIntOption(options, "max_hops", maxTracerouteHops)

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

	hops, err := runTraceroute(targetIP.IP, maxHops, timeout)
	if err != nil {
		// Degraded mode: single TCP reachability hop (no ICMP socket at all)
		hops = tcpReachabilityHop(targetIP.IP, timeout)
	}

	if p.Geo != nil {
		annotateGeo(hops, p.Geo)
	}

	reached := len(hops) > 0 && !hops[len(hops)-1].Timeout && hops[len(hops)-1].IP == targetIP.IP.String()

	return &Result{
		Success:    reached,
		Data:       map[string]any{"target_ip": targetIP.IP.String(), "hops": hops},
		Timestamp:  start,
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// runTraceroute executes a real ICMP-based traceroute by incrementing TTL.
// Returns an error (so Execute degrades to tcpReachabilityHop) when raw ICMP
// is unavailable — dgram sockets cannot observe the TimeExceeded packets that
// every intermediate hop replies with, so the TTL walk would yield 30 timeout
// rows over 90+ seconds. Better to fail fast and let the caller surface a
// single-hop reachability result inside the original probe deadline.
func runTraceroute(dst net.IP, maxHops int, timeout time.Duration) ([]TracerouteHop, error) {
	recv, _, isRaw, err := listenICMP4()
	if err != nil {
		return nil, fmt.Errorf("open recv socket: %w", err)
	}
	defer recv.Close()
	if !isRaw {
		return nil, fmt.Errorf("traceroute requires raw ICMP socket (CAP_NET_RAW); dgram ICMP cannot receive TimeExceeded")
	}

	send, writeAddr, _, err := listenICMP4()
	if err != nil {
		return nil, fmt.Errorf("open send socket: %w", err)
	}
	defer send.Close()

	// Derive per-hop timeout from total timeout; floor at 500ms per hop.
	perHop := hopTimeout
	if timeout > 0 && maxHops > 0 {
		derived := timeout / time.Duration(maxHops)
		if derived > 500*time.Millisecond {
			perHop = derived
		}
	}

	probeID := randICMPID()
	var hops []TracerouteHop

	for ttl := 1; ttl <= maxHops; ttl++ {
		hop := probeHop(send, recv, dst, writeAddr, ttl, probeID, perHop)
		hops = append(hops, hop)

		// Stop when we reach the destination
		if !hop.Timeout && hop.IP == dst.String() {
			break
		}
	}

	return hops, nil
}

// probeHop sends probesPerHop ICMP Echo packets at a given TTL and returns
// the responding router IP and average RTT.
func probeHop(send, recv *icmp.PacketConn, dst net.IP, writeAddr func(net.IP) net.Addr, ttl, probeID int, perHopTimeout time.Duration) TracerouteHop {
	hop := TracerouteHop{Hop: ttl, Timeout: true}

	if err := send.IPv4PacketConn().SetTTL(ttl); err != nil {
		return hop
	}

	var rtts []time.Duration
	var hopIP string

	for probe := range probesPerHop {
		seq := ttl*100 + probe
		msg := &icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{ID: probeID, Seq: seq, Data: []byte("idcd-trace")},
		}
		b, err := msg.Marshal(nil)
		if err != nil {
			continue
		}

		start := time.Now()
		if _, err := send.WriteTo(b, writeAddr(dst)); err != nil {
			continue
		}

		recv.SetReadDeadline(time.Now().Add(perHopTimeout))
		reply := make([]byte, 1500)
		n, peer, err := recv.ReadFrom(reply)
		if err != nil {
			continue
		}
		rtt := time.Since(start)

		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), reply[:n])
		if err != nil {
			continue
		}

		switch rm.Type {
		case ipv4.ICMPTypeTimeExceeded:
			if ip := peerIP(peer); ip != nil {
				hopIP = ip.String()
			}
			rtts = append(rtts, rtt)
		case ipv4.ICMPTypeEchoReply:
			if echo, ok := rm.Body.(*icmp.Echo); ok && echo.ID == probeID {
				if ip := peerIP(peer); ip != nil {
					hopIP = ip.String()
				}
				rtts = append(rtts, rtt)
			}
		}
	}

	if len(rtts) == 0 {
		return hop
	}

	var sum time.Duration
	for _, r := range rtts {
		sum += r
	}
	hop.RTTMs = msFloat(sum / time.Duration(len(rtts)))
	hop.IP = hopIP
	hop.Timeout = false

	if hopIP != "" {
		if names, err := net.LookupAddr(hopIP); err == nil && len(names) > 0 {
			hop.Hostname = names[0]
		}
	}

	return hop
}

// tcpReachabilityHop is the no-permission fallback: a single hop showing
// whether the target is TCP-reachable.
func tcpReachabilityHop(ip net.IP, timeout time.Duration) []TracerouteHop {
	hop := TracerouteHop{Hop: 1, IP: ip.String(), Timeout: true}

	start := time.Now()
	for _, port := range []string{"80", "443", "53"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), port), timeout)
		if err == nil {
			conn.Close()
			hop.RTTMs = msFloat(time.Since(start))
			hop.Timeout = false
			if names, err := net.LookupAddr(ip.String()); err == nil && len(names) > 0 {
				hop.Hostname = names[0]
			}
			break
		}
	}
	return []TracerouteHop{hop}
}
