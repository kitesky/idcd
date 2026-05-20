package probe

import (
	"net"

	"golang.org/x/net/icmp"
)

// listenICMP4 opens an IPv4 ICMP PacketConn on the loopback wildcard, preferring
// an unprivileged datagram socket ("udp4" / SOCK_DGRAM + IPPROTO_ICMP) and
// falling back to a raw socket ("ip4:icmp" / SOCK_RAW + IPPROTO_ICMP).
//
// The datagram path is what lets the agent run real ICMP on macOS without root
// and on Linux without CAP_NET_RAW — on Darwin it is the default and unprivileged
// by design, and on Linux the systemd unit pre-configures
// `net.ipv4.ping_group_range` to cover the agent's gid. The raw fallback is
// kept for hardened containers / older kernels where datagram ICMP is denied.
//
// Returns the connection and a writeAddr factory because WriteTo wants a
// different concrete type per mode: `*net.UDPAddr{IP: dst}` for udp4,
// `*net.IPAddr{IP: dst}` for ip4:icmp. Callers that ReadFrom the same
// connection should use peerIP to extract the source IP — peer is reported
// as the matching concrete type and a naive `peer.(*net.IPAddr)` would silently
// drop every datagram-mode packet.
func listenICMP4() (conn *icmp.PacketConn, writeAddr func(net.IP) net.Addr, err error) {
	if c, e := icmp.ListenPacket("udp4", "0.0.0.0"); e == nil {
		return c, func(ip net.IP) net.Addr { return &net.UDPAddr{IP: ip} }, nil
	}
	c, e := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if e != nil {
		return nil, nil, e
	}
	return c, func(ip net.IP) net.Addr { return &net.IPAddr{IP: ip} }, nil
}

// listenICMP6 is the IPv6 analogue of listenICMP4.
func listenICMP6() (conn *icmp.PacketConn, writeAddr func(net.IP) net.Addr, err error) {
	if c, e := icmp.ListenPacket("udp6", "::"); e == nil {
		return c, func(ip net.IP) net.Addr { return &net.UDPAddr{IP: ip} }, nil
	}
	c, e := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	if e != nil {
		return nil, nil, e
	}
	return c, func(ip net.IP) net.Addr { return &net.IPAddr{IP: ip} }, nil
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
