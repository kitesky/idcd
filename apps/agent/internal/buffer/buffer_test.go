package buffer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/task"
)

func TestNewBuffer(t *testing.T) {
	tempDir := t.TempDir()

	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Check that database file was created
	dbPath := filepath.Join(tempDir, DefaultDBPath)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestStoreAndRetrieveResults(t *testing.T) {
	tempDir := t.TempDir()
	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Create test result
	result := task.Result{
		TaskID:     "test-task-123",
		NodeID:     "test-node-456",
		Type:       "http",
		Target:     "example.com",
		Success:    true,
		Data:       map[string]any{"status_code": 200, "duration_ms": 150},
		Watermark:  "test-watermark",
		Timestamp:  time.Now(),
		DurationMs: 150,
	}

	// Store result
	err = buffer.Store(result)
	if err != nil {
		t.Fatalf("Failed to store result: %v", err)
	}

	// Retrieve pending results
	pending, err := buffer.Pending()
	if err != nil {
		t.Fatalf("Failed to get pending results: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending result, got %d", len(pending))
	}

	retrieved := pending[0]
	if retrieved.Result.TaskID != result.TaskID {
		t.Errorf("Expected TaskID %s, got %s", result.TaskID, retrieved.Result.TaskID)
	}

	if retrieved.Result.NodeID != result.NodeID {
		t.Errorf("Expected NodeID %s, got %s", result.NodeID, retrieved.Result.NodeID)
	}

	if retrieved.Result.Success != result.Success {
		t.Errorf("Expected Success %t, got %t", result.Success, retrieved.Result.Success)
	}
}

func TestMarkSent(t *testing.T) {
	tempDir := t.TempDir()
	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Store a result
	result := task.Result{
		TaskID:     "test-task-123",
		NodeID:     "test-node-456",
		Type:       "http",
		Target:     "example.com",
		Success:    true,
		Data:       map[string]any{},
		Timestamp:  time.Now(),
		DurationMs: 150,
	}

	err = buffer.Store(result)
	if err != nil {
		t.Fatalf("Failed to store result: %v", err)
	}

	// Get pending results
	pending, err := buffer.Pending()
	if err != nil {
		t.Fatalf("Failed to get pending results: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending result, got %d", len(pending))
	}

	// Mark as sent
	err = buffer.MarkSent(pending[0].ID)
	if err != nil {
		t.Fatalf("Failed to mark result as sent: %v", err)
	}

	// Check that no results are pending
	pending, err = buffer.Pending()
	if err != nil {
		t.Fatalf("Failed to get pending results: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("Expected 0 pending results after marking sent, got %d", len(pending))
	}
}

func TestBufferStats(t *testing.T) {
	tempDir := t.TempDir()
	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Initial stats should be empty
	stats, err := buffer.Stats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.PendingCount != 0 {
		t.Errorf("Expected 0 pending results, got %d", stats.PendingCount)
	}

	if stats.SentCount != 0 {
		t.Errorf("Expected 0 sent results, got %d", stats.SentCount)
	}

	// Store some results
	for i := 0; i < 3; i++ {
		result := task.Result{
			TaskID:     fmt.Sprintf("task-%d", i),
			NodeID:     "test-node",
			Type:       "http",
			Target:     "example.com",
			Success:    true,
			Data:       map[string]any{},
			Timestamp:  time.Now(),
			DurationMs: 100,
		}

		err = buffer.Store(result)
		if err != nil {
			t.Fatalf("Failed to store result %d: %v", i, err)
		}
	}

	// Check stats after storing
	stats, err = buffer.Stats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.PendingCount != 3 {
		t.Errorf("Expected 3 pending results, got %d", stats.PendingCount)
	}

	if stats.SentCount != 0 {
		t.Errorf("Expected 0 sent results, got %d", stats.SentCount)
	}

	// Mark one as sent
	pending, _ := buffer.Pending()
	buffer.MarkSent(pending[0].ID)

	// Check stats after marking one sent
	stats, err = buffer.Stats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.PendingCount != 2 {
		t.Errorf("Expected 2 pending results, got %d", stats.PendingCount)
	}

	if stats.SentCount != 1 {
		t.Errorf("Expected 1 sent result, got %d", stats.SentCount)
	}
}

func TestCleanup(t *testing.T) {
	tempDir := t.TempDir()
	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Store and mark some results as sent
	for i := 0; i < 5; i++ {
		result := task.Result{
			TaskID:     fmt.Sprintf("task-%d", i),
			NodeID:     "test-node",
			Type:       "http",
			Target:     "example.com",
			Success:    true,
			Data:       map[string]any{},
			Timestamp:  time.Now().Add(-time.Duration(i) * time.Hour), // Varying ages
			DurationMs: 100,
		}

		buffer.Store(result)
	}

	// Mark all as sent
	pending, _ := buffer.Pending()
	for _, p := range pending {
		buffer.MarkSent(p.ID)
	}

	// Cleanup old results (older than 2 hours)
	err = buffer.Cleanup(2 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Check that only recent results remain
	stats, err := buffer.Stats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Should have cleaned up results older than 2 hours
	if stats.SentCount > 3 {
		t.Errorf("Expected <= 3 sent results after cleanup, got %d", stats.SentCount)
	}
}

func TestBufferSizeLimit(t *testing.T) {
	tempDir := t.TempDir()

	// Create a large file to simulate buffer size exceeded
	dbPath := filepath.Join(tempDir, DefaultDBPath)
	largeFile, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	// Write more than MaxBufferSize
	largeData := make([]byte, MaxBufferSize+1)
	largeFile.Write(largeData)
	largeFile.Close()

	// Try to create buffer - should fail due to size limit
	_, err = New(tempDir)
	if err == nil {
		t.Error("Expected error due to buffer size limit, got none")
	}

	if err != ErrBufferFull {
		t.Errorf("Expected ErrBufferFull, got %v", err)
	}
}

func TestMultipleResults(t *testing.T) {
	tempDir := t.TempDir()
	buffer, err := New(tempDir)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	defer buffer.Close()

	// Store multiple results with different types
	results := []task.Result{
		{
			TaskID: "http-task", NodeID: "node-1", Type: "http", Target: "example.com",
			Success: true, Data: map[string]any{"status_code": 200}, Timestamp: time.Now(), DurationMs: 100,
		},
		{
			TaskID: "ping-task", NodeID: "node-1", Type: "ping", Target: "example.com",
			Success: true, Data: map[string]any{"avg_ms": 50}, Timestamp: time.Now(), DurationMs: 200,
		},
		{
			TaskID: "tcp-task", NodeID: "node-1", Type: "tcp", Target: "example.com:80",
			Success: false, Error: "connection refused", Data: map[string]any{}, Timestamp: time.Now(), DurationMs: 5000,
		},
	}

	for _, result := range results {
		err = buffer.Store(result)
		if err != nil {
			t.Fatalf("Failed to store result %s: %v", result.TaskID, err)
		}
	}

	// Retrieve all pending results
	pending, err := buffer.Pending()
	if err != nil {
		t.Fatalf("Failed to get pending results: %v", err)
	}

	if len(pending) != 3 {
		t.Fatalf("Expected 3 pending results, got %d", len(pending))
	}

	// Verify results are ordered by creation time (ASC)
	for i := 0; i < len(pending)-1; i++ {
		if pending[i].CreatedAt.After(pending[i+1].CreatedAt) {
			t.Error("Results are not ordered by creation time")
		}
	}

	// Verify different task types are preserved
	taskTypes := make(map[string]bool)
	for _, p := range pending {
		taskTypes[string(p.Result.Type)] = true
	}

	expectedTypes := []string{"http", "ping", "tcp"}
	for _, expected := range expectedTypes {
		if !taskTypes[expected] {
			t.Errorf("Expected task type %s not found in results", expected)
		}
	}
}