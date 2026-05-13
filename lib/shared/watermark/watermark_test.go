package watermark

import (
	"testing"
)

func TestSign(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"

	sig := Sign(nodeID, taskID, target, secret)

	// Should return non-empty hex string
	if sig == "" {
		t.Error("Sign() returned empty string")
	}

	// Should be valid hex (64 chars for SHA256)
	if len(sig) != 64 {
		t.Errorf("Sign() returned signature of length %d, expected 64", len(sig))
	}
}

func TestSignWithTimestamp(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	// Should return non-empty hex string
	if sig == "" {
		t.Error("SignWithTimestamp() returned empty string")
	}

	// Should be valid hex (64 chars for SHA256)
	if len(sig) != 64 {
		t.Errorf("SignWithTimestamp() returned signature of length %d, expected 64", len(sig))
	}

	// Same inputs should produce same signature (deterministic)
	sig2 := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	if sig != sig2 {
		t.Error("SignWithTimestamp() produced different signatures for same inputs")
	}
}

func TestVerifyWithTimestamp_Success(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if !VerifyWithTimestamp(nodeID, taskID, target, secret, timestamp, sig) {
		t.Error("VerifyWithTimestamp() failed with correct parameters")
	}
}

func TestVerifyWithTimestamp_WrongNodeID(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if VerifyWithTimestamp("wrong-node", taskID, target, secret, timestamp, sig) {
		t.Error("VerifyWithTimestamp() succeeded with wrong node ID")
	}
}

func TestVerifyWithTimestamp_WrongTaskID(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if VerifyWithTimestamp(nodeID, "wrong-task", target, secret, timestamp, sig) {
		t.Error("VerifyWithTimestamp() succeeded with wrong task ID")
	}
}

func TestVerifyWithTimestamp_WrongTarget(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if VerifyWithTimestamp(nodeID, taskID, "wrong-target", secret, timestamp, sig) {
		t.Error("VerifyWithTimestamp() succeeded with wrong target")
	}
}

func TestVerifyWithTimestamp_WrongSecret(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if VerifyWithTimestamp(nodeID, taskID, target, "wrong-secret", timestamp, sig) {
		t.Error("VerifyWithTimestamp() succeeded with wrong secret")
	}
}

func TestVerifyWithTimestamp_WrongTimestamp(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if VerifyWithTimestamp(nodeID, taskID, target, secret, "1609459300", sig) {
		t.Error("VerifyWithTimestamp() succeeded with wrong timestamp")
	}
}

func TestVerifyWithTimestamp_WrongSignature(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	if VerifyWithTimestamp(nodeID, taskID, target, secret, timestamp, "invalid-signature") {
		t.Error("VerifyWithTimestamp() succeeded with invalid signature")
	}
}

func TestTimestamp(t *testing.T) {
	ts := Timestamp()

	// Should return non-empty string
	if ts == "" {
		t.Error("Timestamp() returned empty string")
	}

	// Should be parseable as integer
	if _, err := parseInt(ts); err != nil {
		t.Errorf("Timestamp() returned non-numeric value: %s", ts)
	}

	// Should be reasonable (after year 2000, before year 2100)
	tsInt, _ := parseInt(ts)
	year2000 := int64(946684800)  // 2000-01-01 00:00:00 UTC
	year2100 := int64(4102444800) // 2100-01-01 00:00:00 UTC
	if tsInt < year2000 || tsInt > year2100 {
		t.Errorf("Timestamp() returned unreasonable value: %d", tsInt)
	}
}

func TestSignDeterministic(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	secret := "test-secret-key"
	timestamp := "1609459200"

	// Same inputs should always produce same output
	sig1 := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	sig2 := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)
	sig3 := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if sig1 != sig2 || sig2 != sig3 {
		t.Error("Sign is not deterministic for same inputs")
	}
}

func TestSignDifferentInputs(t *testing.T) {
	secret := "test-secret-key"
	timestamp := "1609459200"

	sig1 := SignWithTimestamp("node-1", "task-1", "target-1", secret, timestamp)
	sig2 := SignWithTimestamp("node-2", "task-1", "target-1", secret, timestamp)
	sig3 := SignWithTimestamp("node-1", "task-2", "target-1", secret, timestamp)
	sig4 := SignWithTimestamp("node-1", "task-1", "target-2", secret, timestamp)

	// Different inputs should produce different signatures
	if sig1 == sig2 || sig1 == sig3 || sig1 == sig4 {
		t.Error("Different inputs produced same signature")
	}
}

func TestEmptyInputs(t *testing.T) {
	// Sign should work even with empty inputs
	sig := SignWithTimestamp("", "", "", "", "")

	if sig == "" {
		t.Error("Sign failed with empty inputs")
	}

	// Verify should work with empty inputs
	if !VerifyWithTimestamp("", "", "", "", "", sig) {
		t.Error("Verify failed with empty inputs")
	}
}

func TestSpecialCharacters(t *testing.T) {
	nodeID := "node:with:colons"
	taskID := "task-with-dashes"
	target := "https://example.com:8080/path?query=value"
	secret := "secret!@#$%^&*()"
	timestamp := "1609459200"

	sig := SignWithTimestamp(nodeID, taskID, target, secret, timestamp)

	if !VerifyWithTimestamp(nodeID, taskID, target, secret, timestamp, sig) {
		t.Error("Sign/Verify failed with special characters")
	}
}

// Helper function to parse int
func parseInt(s string) (int64, error) {
	var result int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		result = result*10 + int64(c-'0')
	}
	return result, nil
}

type parseError struct {
	s string
}

func (e *parseError) Error() string {
	return "invalid integer: " + e.s
}
