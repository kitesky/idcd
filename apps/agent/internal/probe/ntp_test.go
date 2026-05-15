package probe

import (
	"testing"
	"time"
)

func TestNTPProbe_badHost(t *testing.T) {
	p := &NTPProbe{}
	result := p.Execute("192.0.2.1", 200*time.Millisecond, map[string]any{})
	if result.Success {
		t.Error("expected Success=false for unreachable host")
	}
	if result.Error == "" {
		t.Error("expected non-empty Error for unreachable host")
	}
}
