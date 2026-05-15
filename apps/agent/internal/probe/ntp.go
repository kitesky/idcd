package probe

import (
	"encoding/binary"
	"net"
	"time"
)

const ntpPort = "123"

// ntpDelta is the difference in seconds between NTP epoch (Jan 1, 1900)
// and Unix epoch (Jan 1, 1970).
const ntpDelta = 2208988800

// Execute performs an NTP server query.
func (p *NTPProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	addr := net.JoinHostPort(target, ntpPort)
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return &Result{
			Type:       TaskNTP,
			Target:     target,
			Success:    false,
			Error:      err.Error(),
			Data:       map[string]any{},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// NTP request: 48 bytes, first byte = LI=0, VN=4, Mode=3 (client)
	req := make([]byte, 48)
	req[0] = 0x23 // LI=0, VN=4, Mode=3

	if _, err := conn.Write(req); err != nil {
		return &Result{
			Type:       TaskNTP,
			Target:     target,
			Success:    false,
			Error:      "write failed: " + err.Error(),
			Data:       map[string]any{},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}

	resp := make([]byte, 48)
	if _, err := conn.Read(resp); err != nil {
		return &Result{
			Type:       TaskNTP,
			Target:     target,
			Success:    false,
			Error:      "read failed: " + err.Error(),
			Data:       map[string]any{},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}

	// Transmit timestamp is at bytes 40-47
	secs := binary.BigEndian.Uint32(resp[40:44])
	unixSecs := int64(secs) - ntpDelta
	serverTime := time.Unix(unixSecs, 0).UTC()

	durationMs := time.Since(start).Milliseconds()
	offsetMs := serverTime.UnixMilli() - time.Now().UnixMilli()

	return &Result{
		Type:    TaskNTP,
		Target:  target,
		Success: true,
		Data: map[string]any{
			"server_time": serverTime.Format(time.RFC3339),
			"offset_ms":   offsetMs,
		},
		DurationMs: durationMs,
		Timestamp:  time.Now(),
	}
}
