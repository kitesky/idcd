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

// Sign returns (signature, timestamp). Callers must persist the timestamp
// alongside the signature to verify later via VerifyWithTimestamp.
func Sign(nodeID, taskID, target, secret string) (signature, timestamp string) {
	timestamp = Timestamp()
	signature = SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	return
}

// SignWithTimestamp signs with a caller-supplied timestamp string.
func SignWithTimestamp(nodeID, taskID, target, secret, timestamp string) string {
	payload := fmt.Sprintf("%s:%s:%s:%s", nodeID, taskID, target, timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyWithTimestamp checks a signature against the original timestamp.
func VerifyWithTimestamp(nodeID, taskID, target, secret, timestamp, signature string) bool {
	expected := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	return hmac.Equal([]byte(signature), []byte(expected))
}

// Timestamp returns the current Unix timestamp as a string.
func Timestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}
