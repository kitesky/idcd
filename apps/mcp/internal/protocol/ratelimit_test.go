package protocol

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLimiter_AllowsUpToMax(t *testing.T) {
	l := NewMemoryLimiter(time.Minute, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		dec, err := l.Allow(ctx, "tok1")
		if err != nil {
			t.Fatalf("call %d: unexpected err: %v", i, err)
		}
		if !dec.Allowed {
			t.Fatalf("call %d: expected allowed", i)
		}
	}

	dec, err := l.Allow(ctx, "tok1")
	if err != nil {
		t.Fatalf("4th: unexpected err: %v", err)
	}
	if dec.Allowed {
		t.Fatal("4th call should be denied")
	}
	if dec.ResetAfter <= 0 {
		t.Fatalf("denied decision should report reset hint, got %s", dec.ResetAfter)
	}
}

func TestMemoryLimiter_PerKeyIsolation(t *testing.T) {
	l := NewMemoryLimiter(time.Minute, 1)
	ctx := context.Background()

	a, _ := l.Allow(ctx, "a")
	b, _ := l.Allow(ctx, "b")
	if !a.Allowed || !b.Allowed {
		t.Fatal("both first-time keys should be allowed")
	}

	a2, _ := l.Allow(ctx, "a")
	if a2.Allowed {
		t.Fatal("key a should be denied on second call")
	}
}

func TestMemoryLimiter_WindowExpiry(t *testing.T) {
	l := NewMemoryLimiter(50*time.Millisecond, 1)
	// fixed clock so the test stays deterministic
	base := time.Unix(1_700_000_000, 0)
	l.now = func() time.Time { return base }

	ctx := context.Background()
	dec, _ := l.Allow(ctx, "k")
	if !dec.Allowed {
		t.Fatal("first hit should be allowed")
	}
	dec, _ = l.Allow(ctx, "k")
	if dec.Allowed {
		t.Fatal("second hit within window should be denied")
	}

	// advance past the window
	l.now = func() time.Time { return base.Add(60 * time.Millisecond) }
	dec, _ = l.Allow(ctx, "k")
	if !dec.Allowed {
		t.Fatal("hit after window should be allowed again")
	}
}

func TestMemoryLimiter_DisabledWhenMaxZero(t *testing.T) {
	l := NewMemoryLimiter(time.Minute, 0)
	for i := 0; i < 100; i++ {
		dec, err := l.Allow(context.Background(), "k")
		if err != nil || !dec.Allowed {
			t.Fatalf("disabled limiter should always allow, got allowed=%v err=%v", dec.Allowed, err)
		}
	}
}

func TestMemoryLimiter_NilSafe(t *testing.T) {
	var l *MemoryLimiter
	dec, err := l.Allow(context.Background(), "k")
	if err != nil || !dec.Allowed {
		t.Fatalf("nil limiter should always allow, got allowed=%v err=%v", dec.Allowed, err)
	}
}
