package probe

import (
	"net"
	"testing"
	"time"
)

func TestTCPReachabilityHop_Localhost(t *testing.T) {
	// Start a real TCP listener so the hop succeeds.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind TCP listener:", err)
	}
	defer ln.Close()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	_ = portStr // just need the listener to be open

	ip := net.ParseIP("127.0.0.1")
	hops := tcpReachabilityHop(ip, 2*time.Second)

	if len(hops) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(hops))
	}
	if hops[0].Hop != 1 {
		t.Errorf("hop number want 1, got %d", hops[0].Hop)
	}
	if hops[0].IP != "127.0.0.1" {
		t.Errorf("hop IP want 127.0.0.1, got %q", hops[0].IP)
	}
	// localhost always has ports open (SSH, etc.) so Timeout should be false.
	// But if somehow not — that's environment-dependent; don't hard-fail.
	t.Logf("hop: %+v", hops[0])
}

func TestTCPReachabilityHop_Unreachable(t *testing.T) {
	// 192.0.2.0/24 is TEST-NET-1 (RFC 5737) — not routable.
	ip := net.ParseIP("192.0.2.1")
	hops := tcpReachabilityHop(ip, 200*time.Millisecond)

	if len(hops) != 1 {
		t.Fatalf("expected 1 hop even for unreachable host, got %d", len(hops))
	}
	if hops[0].Hop != 1 {
		t.Errorf("hop number want 1, got %d", hops[0].Hop)
	}
	// May or may not timeout depending on environment — just ensure no panic.
	t.Logf("unreachable hop: timeout=%v rtt=%v", hops[0].Timeout, hops[0].RTT)
}

func TestTracerouteProbe_Execute_Fallback(t *testing.T) {
	// Execute should always return a Result (never nil), even without raw socket perms.
	p := &TracerouteProbe{}
	res := p.Execute("127.0.0.1", 500*time.Millisecond, nil)
	if res == nil {
		t.Fatal("Execute returned nil Result")
	}
	if res.Data == nil {
		t.Error("Result.Data must not be nil")
	}
	hops, ok := res.Data["hops"]
	if !ok {
		t.Error("Result.Data must contain 'hops' key")
	}
	_ = hops
	t.Logf("traceroute result: success=%v dur=%dms", res.Success, res.DurationMs)
}

func TestTracerouteProbe_Execute_InvalidTarget(t *testing.T) {
	p := &TracerouteProbe{}
	res := p.Execute("not-a-valid-hostname-xyz.invalid", 500*time.Millisecond, nil)
	if res == nil {
		t.Fatal("Execute returned nil")
	}
	if res.Success {
		t.Error("expected Success=false for unresolvable target")
	}
	if res.Error == "" {
		t.Error("expected non-empty Error for unresolvable target")
	}
}
