package watermark

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	ts := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC
	secretKey := []byte("test-secret-key")

	// Test signing
	signature := Sign(nodeID, taskID, target, ts, secretKey)

	if signature == "" {
		t.Error("Sign() returned empty signature")
	}

	// Test verification with correct parameters
	if !Verify(signature, nodeID, taskID, target, ts, secretKey) {
		t.Error("Verify() failed with correct parameters")
	}

	// Test verification with wrong node ID
	if Verify(signature, "wrong-node", taskID, target, ts, secretKey) {
		t.Error("Verify() succeeded with wrong node ID")
	}

	// Test verification with wrong task ID
	if Verify(signature, nodeID, "wrong-task", target, ts, secretKey) {
		t.Error("Verify() succeeded with wrong task ID")
	}

	// Test verification with wrong target
	if Verify(signature, nodeID, taskID, "wrong-target", ts, secretKey) {
		t.Error("Verify() succeeded with wrong target")
	}

	// Test verification with wrong timestamp
	wrongTS := ts.Add(time.Hour)
	if Verify(signature, nodeID, taskID, target, wrongTS, secretKey) {
		t.Error("Verify() succeeded with wrong timestamp")
	}

	// Test verification with wrong secret key
	wrongKey := []byte("wrong-secret-key")
	if Verify(signature, nodeID, taskID, target, ts, wrongKey) {
		t.Error("Verify() succeeded with wrong secret key")
	}
}

func TestVerifyWithSkew(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	ts := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC
	secretKey := []byte("test-secret-key")

	signature := Sign(nodeID, taskID, target, ts, secretKey)

	// Test verification with exact timestamp
	if !VerifyWithSkew(signature, nodeID, taskID, target, ts, secretKey, 60) {
		t.Error("VerifyWithSkew() failed with exact timestamp")
	}

	// Test verification with timestamp within skew tolerance
	skewedTS := ts.Add(30 * time.Second)
	if !VerifyWithSkew(signature, nodeID, taskID, target, skewedTS, secretKey, 60) {
		t.Error("VerifyWithSkew() failed with timestamp within tolerance")
	}

	// Test verification with timestamp outside skew tolerance
	outsideSkewTS := ts.Add(120 * time.Second)
	if VerifyWithSkew(signature, nodeID, taskID, target, outsideSkewTS, secretKey, 60) {
		t.Error("VerifyWithSkew() succeeded with timestamp outside tolerance")
	}

	// Test verification with negative skew
	negativeSkewTS := ts.Add(-30 * time.Second)
	if !VerifyWithSkew(signature, nodeID, taskID, target, negativeSkewTS, secretKey, 60) {
		t.Error("VerifyWithSkew() failed with negative skew within tolerance")
	}
}

func TestParseWatermarkPayload(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		expectErr bool
		nodeID    string
		taskID    string
		target    string
		timestamp int64
	}{
		{
			name:      "valid payload",
			payload:   "node-123:task-456:example.com:1609459200",
			expectErr: false,
			nodeID:    "node-123",
			taskID:    "task-456",
			target:    "example.com",
			timestamp: 1609459200,
		},
		{
			name:      "invalid format - too few parts",
			payload:   "node-123:task-456",
			expectErr: true,
		},
		{
			name:      "invalid format - too many parts",
			payload:   "node-123:task-456:example.com:1609459200:extra",
			expectErr: true,
		},
		{
			name:      "invalid timestamp",
			payload:   "node-123:task-456:example.com:invalid",
			expectErr: true,
		},
		{
			name:      "empty payload",
			payload:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeID, taskID, target, ts, err := ParseWatermarkPayload(tt.payload)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if nodeID != tt.nodeID {
				t.Errorf("Expected nodeID %s, got %s", tt.nodeID, nodeID)
			}

			if taskID != tt.taskID {
				t.Errorf("Expected taskID %s, got %s", tt.taskID, taskID)
			}

			if target != tt.target {
				t.Errorf("Expected target %s, got %s", tt.target, target)
			}

			if ts.Unix() != tt.timestamp {
				t.Errorf("Expected timestamp %d, got %d", tt.timestamp, ts.Unix())
			}
		})
	}
}

func TestSignConsistency(t *testing.T) {
	nodeID := "node-123"
	taskID := "task-456"
	target := "example.com"
	ts := time.Unix(1609459200, 0)
	secretKey := []byte("test-secret-key")

	// Sign the same payload multiple times
	sig1 := Sign(nodeID, taskID, target, ts, secretKey)
	sig2 := Sign(nodeID, taskID, target, ts, secretKey)
	sig3 := Sign(nodeID, taskID, target, ts, secretKey)

	// All signatures should be identical
	if sig1 != sig2 || sig2 != sig3 {
		t.Error("Sign() produced different signatures for identical inputs")
	}

	// All signatures should verify
	if !Verify(sig1, nodeID, taskID, target, ts, secretKey) {
		t.Error("First signature failed verification")
	}
	if !Verify(sig2, nodeID, taskID, target, ts, secretKey) {
		t.Error("Second signature failed verification")
	}
	if !Verify(sig3, nodeID, taskID, target, ts, secretKey) {
		t.Error("Third signature failed verification")
	}
}