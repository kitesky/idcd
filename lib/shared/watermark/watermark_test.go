package watermark

import (
	"strconv"
	"testing"
)

func TestSign(t *testing.T) {
	sig, ts := Sign("node-123", "task-456", "example.com", "test-secret-key")

	if sig == "" {
		t.Error("Sign() returned empty signature")
	}
	if len(sig) != 64 {
		t.Errorf("Sign() signature length = %d, want 64", len(sig))
	}
	if ts == "" {
		t.Error("Sign() returned empty timestamp")
	}

	// timestamp must be verifiable with the returned timestamp
	if !VerifyWithTimestamp("node-123", "task-456", "example.com", "test-secret-key", ts, sig) {
		t.Error("Sign() returned signature that fails VerifyWithTimestamp")
	}
}

func TestSignWithTimestamp(t *testing.T) {
	const ts = "1609459200"
	sig := SignWithTimestamp("node-123", "task-456", "example.com", "test-secret-key", ts)

	if len(sig) != 64 {
		t.Errorf("SignWithTimestamp() signature length = %d, want 64", len(sig))
	}

	sig2 := SignWithTimestamp("node-123", "task-456", "example.com", "test-secret-key", ts)
	if sig != sig2 {
		t.Error("SignWithTimestamp() is not deterministic")
	}
}

func TestVerifyWithTimestamp_Success(t *testing.T) {
	const ts = "1609459200"
	sig := SignWithTimestamp("node-123", "task-456", "example.com", "secret", ts)
	if !VerifyWithTimestamp("node-123", "task-456", "example.com", "secret", ts, sig) {
		t.Error("VerifyWithTimestamp() failed with correct parameters")
	}
}

func TestVerifyWithTimestamp_WrongFields(t *testing.T) {
	const ts = "1609459200"
	sig := SignWithTimestamp("node-123", "task-456", "example.com", "secret", ts)

	cases := []struct {
		name      string
		nodeID    string
		taskID    string
		target    string
		secret    string
		timestamp string
	}{
		{"wrong nodeID", "wrong", "task-456", "example.com", "secret", ts},
		{"wrong taskID", "node-123", "wrong", "example.com", "secret", ts},
		{"wrong target", "node-123", "task-456", "wrong", "secret", ts},
		{"wrong secret", "node-123", "task-456", "example.com", "wrong", ts},
		{"wrong timestamp", "node-123", "task-456", "example.com", "secret", "1609459300"},
		{"wrong signature", "node-123", "task-456", "example.com", "secret", ts},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			badSig := sig
			if tc.name == "wrong signature" {
				badSig = "invalid-signature"
			}
			if VerifyWithTimestamp(tc.nodeID, tc.taskID, tc.target, tc.secret, tc.timestamp, badSig) {
				t.Errorf("VerifyWithTimestamp() should have failed for %s", tc.name)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	ts := Timestamp()
	if ts == "" {
		t.Error("Timestamp() returned empty string")
	}
	n, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		t.Errorf("Timestamp() returned non-numeric value: %s", ts)
	}
	const year2000 = int64(946684800)
	const year2100 = int64(4102444800)
	if n < year2000 || n > year2100 {
		t.Errorf("Timestamp() out of expected range: %d", n)
	}
}

func TestSignDifferentInputs(t *testing.T) {
	const ts = "1609459200"
	sigs := []string{
		SignWithTimestamp("node-1", "task-1", "target-1", "secret", ts),
		SignWithTimestamp("node-2", "task-1", "target-1", "secret", ts),
		SignWithTimestamp("node-1", "task-2", "target-1", "secret", ts),
		SignWithTimestamp("node-1", "task-1", "target-2", "secret", ts),
	}
	for i := 1; i < len(sigs); i++ {
		if sigs[0] == sigs[i] {
			t.Errorf("different inputs produced same signature at index %d", i)
		}
	}
}

func TestEmptyInputs(t *testing.T) {
	sig := SignWithTimestamp("", "", "", "", "")
	if sig == "" {
		t.Error("SignWithTimestamp() failed with empty inputs")
	}
	if !VerifyWithTimestamp("", "", "", "", "", sig) {
		t.Error("VerifyWithTimestamp() failed with empty inputs")
	}
}

func TestSpecialCharacters(t *testing.T) {
	sig := SignWithTimestamp("node:with:colons", "task-with-dashes",
		"https://example.com:8080/path?query=value", "secret!@#$%^&*()", "1609459200")
	if !VerifyWithTimestamp("node:with:colons", "task-with-dashes",
		"https://example.com:8080/path?query=value", "secret!@#$%^&*()", "1609459200", sig) {
		t.Error("Sign/Verify failed with special characters")
	}
}
