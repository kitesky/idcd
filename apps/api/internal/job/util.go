package job

import (
	"encoding/json"
	"log/slog"
	"runtime/debug"
	"time"
)

// recoverPanic absorbs a panic from a probe goroutine and logs it so a single
// rogue probe (DNS resolver crash, transport bug, third-party http roundtripper
// going haywire) cannot kill the whole api process. Intended use:
//
//	go func() {
//	    defer wg.Done()
//	    defer recoverPanic(c.logger, "probeServices")
//	    …
//	}()
func recoverPanic(log *slog.Logger, where string) {
	if r := recover(); r != nil {
		if log == nil {
			log = slog.Default()
		}
		log.Error("status collector panic recovered",
			"where", where,
			"panic", r,
			"stack", string(debug.Stack()),
		)
	}
}

// classifyNodeAge returns the status SMALLINT + last_seen age in seconds for
// a node, given the node's last_seen_at (may be NULL) and the current time.
// NULL last_seen_at → outage with ageSec = -1.
func classifyNodeAge(lastSeen *time.Time, now time.Time) (int16, int64) {
	if lastSeen == nil {
		return StatusOutage, -1
	}
	age := now.Sub(*lastSeen)
	ageSec := int64(age.Seconds())
	switch {
	case age < nodeDegradedThreshold:
		return StatusOperational, ageSec
	case age < nodeOutageThreshold:
		return StatusDegraded, ageSec
	default:
		return StatusOutage, ageSec
	}
}

// mustMarshal returns a JSON encoding of v or "{}" if v is empty/unencodable.
// Used by the status collector's JSONB insert — a fallback empty object is
// always valid JSONB, so we never want a single weird detail field to fail
// the whole transaction.
func mustMarshal(v any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
