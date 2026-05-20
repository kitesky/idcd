package protocol

import (
	"context"
	"sync"
	"time"
)

// MemoryLimiter is a per-key in-memory sliding-window limiter. Suitable for a
// single MCP instance; for multi-instance deployments swap to a Redis-backed
// Limiter (the protocol.Limiter interface lets us substitute without changing
// the handlers).
//
// Memory bound: O(distinct keys * Max) — each request appends a timestamp; we
// trim entries older than Window on every Allow call. For abusive callers
// that's bounded by Max + 1 entries per key.
type MemoryLimiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int64
	hits   map[string][]time.Time
	now    func() time.Time
}

// NewMemoryLimiter constructs a per-key sliding-window limiter. window is the
// sliding window size; max is the maximum requests permitted within it. A
// zero or negative max disables the limiter (Allow always returns Allowed=true).
func NewMemoryLimiter(window time.Duration, max int64) *MemoryLimiter {
	return &MemoryLimiter{
		window: window,
		max:    max,
		hits:   make(map[string][]time.Time),
		now:    time.Now,
	}
}

// Allow records an attempt and reports whether it should be served.
func (l *MemoryLimiter) Allow(_ context.Context, key string) (RateLimitDecision, error) {
	if l == nil || l.max <= 0 || l.window <= 0 {
		return RateLimitDecision{Allowed: true}, nil
	}
	now := l.now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	hits := l.hits[key]
	// drop expired entries (they're already sorted ascending — earliest first)
	i := 0
	for ; i < len(hits); i++ {
		if hits[i].After(cutoff) {
			break
		}
	}
	hits = hits[i:]

	if int64(len(hits)) >= l.max {
		// reset after = (earliest in-window hit) + window − now
		resetAfter := hits[0].Add(l.window).Sub(now)
		if resetAfter < 0 {
			resetAfter = 0
		}
		l.hits[key] = hits
		return RateLimitDecision{
			Allowed:    false,
			Remaining:  0,
			ResetAfter: resetAfter,
		}, nil
	}

	hits = append(hits, now)
	l.hits[key] = hits
	return RateLimitDecision{
		Allowed:   true,
		Remaining: l.max - int64(len(hits)),
	}, nil
}
