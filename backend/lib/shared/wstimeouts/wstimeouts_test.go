package wstimeouts_test

import (
	"testing"

	"github.com/kite365/idcd/lib/shared/wstimeouts"
)

// 这些不变量护栏防止调整时把 ping/pong 关系改坏导致连接被反复误判掉线。
func TestTimeoutInvariants(t *testing.T) {
	if wstimeouts.PingInterval >= wstimeouts.PongTimeout {
		t.Fatalf("PingInterval (%v) must be < PongTimeout (%v), or pong will time out before next ping arrives",
			wstimeouts.PingInterval, wstimeouts.PongTimeout)
	}
	if wstimeouts.WriteTimeout <= 0 {
		t.Fatalf("WriteTimeout must be positive, got %v", wstimeouts.WriteTimeout)
	}
	if wstimeouts.BackoffMin > wstimeouts.BackoffMax {
		t.Fatalf("BackoffMin (%v) must be <= BackoffMax (%v)",
			wstimeouts.BackoffMin, wstimeouts.BackoffMax)
	}
	if wstimeouts.MaxMessageBytes <= 0 {
		t.Fatalf("MaxMessageBytes must be positive, got %d", wstimeouts.MaxMessageBytes)
	}
}
