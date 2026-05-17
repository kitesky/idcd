package tools

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Maximum lengths for common MCP tool inputs. Generous enough that they
// don't reject legitimate hostnames / URLs, but small enough that a
// malicious caller can't shovel megabytes of junk through a JSON-RPC
// payload (the protocol layer caps the body at 1 MiB; per-field caps
// here stop a single oversized argument from saturating downstream
// API calls / logs).
const (
	maxTargetLen = 253 // RFC 1035 hostname max
	maxURLLen    = 2048
	maxTextLen   = 4096
	maxCount     = 100
)

// validateTarget normalises and rejects a hostname / IP / URL argument
// before it reaches the downstream API. Catches:
//   - empty / whitespace-only values
//   - CR/LF or other control chars (would let a caller smuggle extra
//     headers into the apiclient HTTP request)
//   - lengths beyond the cap
//   - leading/trailing whitespace (silently trimmed)
func validateTarget(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", errors.New("target is required")
	}
	if len(v) > maxTargetLen {
		return "", fmt.Errorf("target too long (max %d chars)", maxTargetLen)
	}
	for _, r := range v {
		// Reject all control chars + DEL + Unicode line/paragraph separators.
		if r == 0x7F || unicode.IsControl(r) {
			return "", errors.New("target contains control characters")
		}
	}
	return v, nil
}

// validateURL is like validateTarget but allows the longer URL length cap.
func validateURL(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", errors.New("url is required")
	}
	if len(v) > maxURLLen {
		return "", fmt.Errorf("url too long (max %d chars)", maxURLLen)
	}
	for _, r := range v {
		if r == 0x7F || unicode.IsControl(r) {
			return "", errors.New("url contains control characters")
		}
	}
	return v, nil
}

// validateCount clamps an integer-valued argument to [1, maxCount]. Returns
// the clamped value; never errors.
func validateCount(raw float64, def, max int) int {
	if max <= 0 {
		max = maxCount
	}
	if raw <= 0 {
		return def
	}
	n := int(raw)
	if n > max {
		return max
	}
	return n
}

// validateText caps a free-form text argument (e.g. an HTTP body).
func validateText(raw string) (string, error) {
	if len(raw) > maxTextLen {
		return "", fmt.Errorf("text too long (max %d chars)", maxTextLen)
	}
	return raw, nil
}
