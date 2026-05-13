// Package watermark provides HMAC-based watermark signing for probe results.
package watermark

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// Sign generates a watermark signature for probe result parameters.
// signature = HMAC-SHA256(secret, "node_id:task_id:target:timestamp_unix")
func Sign(nodeID, taskID, target, secret string) string {
	ts := Timestamp()
	payload := fmt.Sprintf("%s:%s:%s:%s", nodeID, taskID, target, ts)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// SignWithTimestamp generates a watermark with a specific timestamp.
// This is useful for verification where you need to use the original timestamp.
func SignWithTimestamp(nodeID, taskID, target, secret, timestamp string) string {
	payload := fmt.Sprintf("%s:%s:%s:%s", nodeID, taskID, target, timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// Verify checks if the watermark is valid for the given parameters.
// Returns true if the signature matches.
func Verify(nodeID, taskID, target, secret, signature string) bool {
	expected := Sign(nodeID, taskID, target, secret)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// VerifyWithTimestamp verifies a watermark using a specific timestamp.
// This allows verification of watermarks created at different times.
func VerifyWithTimestamp(nodeID, taskID, target, secret, timestamp, signature string) bool {
	expected := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// Timestamp returns the current Unix timestamp as a string (seconds precision).
func Timestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
