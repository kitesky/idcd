package probe

import (
	"net"

	"golang.org/x/net/icmp"
)

// listenICMP4 opens an IPv4 ICMP PacketConn, preferring a raw socket
// ("ip4:icmp" / SOCK_RAW + IPPROTO_ICMP) and falling back to an unprivileged
// datagram socket ("udp4" / SOCK_DGRAM + IPPROTO_ICMP).
//
// Why raw-first (changed from dgram-first):
//
//	Linux's unprivileged dgram ICMP socket has two trapdoors that break our
//	probes and can ONLY be worked around by also having raw access available:
//	  1. echo.id is rewritten by the kernel on send (sk->inet_sport overwrites
//	     whatever userspace put there). Receivers see the kernel-assigned
//	     value, never the original probeID, so any `echo.ID == probeID` match
//	     fails. Worked around in callers via the isRaw return — see pingIPv4
//	     and probeHop.
//	  2. ICMP TimeExceeded is NOT delivered to dgram sockets. ping.c's
//	     ping_rcv() in the Linux kernel only routes ICMP_ECHOREPLY through.
//	     This makes traceroute FUNDAMENTALLY broken in dgram mode — there is
//	     no application-level fix. Traceroute degrades to single-hop TCP
//	     reachability when only dgram is available.
//
// Production agents are expected to have file capability `cap_net_raw=eip`
// applied to the binary (see Dockerfile), so non-root uid 10001 still
// gets effective CAP_NET_RAW and the raw path opens cleanly.
//
// macOS dev environments stay on the dgram path: BSD raw sockets need root,
// so the raw attempt fails fast (EPERM) and we fall back to dgram. macOS
// dgram does NOT rewrite echo.id, so ping behaviour is correct even there
// — but the dgram fix in callers is still needed to keep Linux dev / hardened
// containers honest.
//
// Returns:
//   - conn:      the icmp PacketConn (caller closes)
//   - writeAddr: factory that wraps an IP into the correct net.Addr for WriteTo
//     (*net.IPAddr for raw, *net.UDPAddr for dgram)
//   - isRaw:     true iff we opened a raw socket — callers use this to decide
//     whether echo.id is trustworthy and whether TimeExceeded packets
//     can be observed
//   - err:       non-nil only if BOTH raw and dgram fail
func listenICMP4() (conn *icmp.PacketConn, writeAddr func(net.IP) net.Addr, isRaw bool, err error) {
	if c, e := icmp.ListenPacket("ip4:icmp", "0.0.0.0"); e == nil {
		return c, func(ip net.IP) net.Addr { return &net.IPAddr{IP: ip} }, true, nil
	}
	c, e := icmp.ListenPacket("udp4", "0.0.0.0")
	if e != nil {
		return nil, nil, false, e
	}
	return c, func(ip net.IP) net.Addr { return &net.UDPAddr{IP: ip} }, false, nil
}

// listenICMP6 is the IPv6 analogue of listenICMP4.
func listenICMP6() (conn *icmp.PacketConn, writeAddr func(net.IP) net.Addr, isRaw bool, err error) {
	if c, e := icmp.ListenPacket("ip6:ipv6-icmp", "::"); e == nil {
		return c, func(ip net.IP) net.Addr { return &net.IPAddr{IP: ip} }, true, nil
	}
	c, e := icmp.ListenPacket("udp6", "::")
	if e != nil {
		return nil, nil, false, e
	}
	return c, func(ip net.IP) net.Addr { return &net.UDPAddr{IP: ip} }, false, nil
}

// peerIP extracts the source IP from a net.Addr returned by ICMP ReadFrom,
// transparently handling both datagram (*net.UDPAddr) and raw (*net.IPAddr) modes.
// Returns nil if the address type is unrecognised.
func peerIP(addr net.Addr) net.IP {
	switch a := addr.(type) {
	case *net.IPAddr:
		return a.IP
	case *net.UDPAddr:
		return a.IP
	}
	return nil
}

// SupportsRawICMP returns true when the current process can open a raw ICMP
// socket. Use at startup to surface a clear "missing CAP_NET_RAW" warning in
// logs: ping still works via the dgram fallback, but traceroute degrades to
// a single-hop TCP reachability probe because dgram sockets do not receive
// ICMP TimeExceeded (see listenICMP4 doc).
func SupportsRawICMP() bool {
	c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
