// Package watermark provides HMAC-based watermark signing for probe results.
package watermark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Sign creates an HMAC-SHA256 watermark for the given probe result parameters.
// Format: HMAC-SHA256(node_id + ":" + task_id + ":" + target + ":" + unix_ts, secret_key)
func Sign(nodeID, taskID, target string, ts time.Time, secretKey []byte) string {
	payload := fmt.Sprintf("%s:%s:%s:%d", nodeID, taskID, target, ts.Unix())
	h := hmac.New(sha256.New, secretKey)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify checks if the watermark is valid for the given parameters.
// Returns true if the watermark matches the expected signature.
func Verify(watermark, nodeID, taskID, target string, ts time.Time, secretKey []byte) bool {
	expected := Sign(nodeID, taskID, target, ts, secretKey)
	return hmac.Equal([]byte(watermark), []byte(expected))
}

// VerifyWithSkew verifies a watermark with a time tolerance.
// Allows for clock drift up to skewSeconds in either direction.
func VerifyWithSkew(watermark, nodeID, taskID, target string, ts time.Time, secretKey []byte, skewSeconds int64) bool {
	// Try exact timestamp first
	if Verify(watermark, nodeID, taskID, target, ts, secretKey) {
		return true
	}

	// Try with skew tolerance
	for offset := int64(-skewSeconds); offset <= skewSeconds; offset++ {
		if offset == 0 {
			continue // Already tried above
		}
		adjustedTs := ts.Add(time.Duration(offset) * time.Second)
		if Verify(watermark, nodeID, taskID, target, adjustedTs, secretKey) {
			return true
		}
	}

	return false
}

// ParseWatermarkPayload extracts components from a signed payload.
// Returns an error if the payload format is invalid.
func ParseWatermarkPayload(payload string) (nodeID, taskID, target string, ts time.Time, err error) {
	parts := strings.Split(payload, ":")
	if len(parts) != 4 {
		err = fmt.Errorf("invalid payload format: expected 4 parts, got %d", len(parts))
		return
	}

	nodeID = parts[0]
	taskID = parts[1]
	target = parts[2]

	timestamp, parseErr := strconv.ParseInt(parts[3], 10, 64)
	if parseErr != nil {
		err = fmt.Errorf("invalid timestamp: %w", parseErr)
		return
	}

	ts = time.Unix(timestamp, 0)
	return
}