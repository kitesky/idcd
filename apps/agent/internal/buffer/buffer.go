// Package buffer provides SQLite-based local buffering for probe results.
package buffer

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/probe"
	_ "modernc.org/sqlite"
)

const (
	// MaxBufferSize is the maximum size of the buffer database file (500MB).
	MaxBufferSize = 500 * 1024 * 1024
	// DefaultDBPath is the default database file name.
	DefaultDBPath = "buffer.db"
)

// ErrBufferFull is returned when the buffer exceeds the size limit.
var ErrBufferFull = fmt.Errorf("buffer size limit exceeded")

// Buffer provides SQLite-based buffering for probe results.
type Buffer struct {
	db   *sql.DB
	path string
}

// New creates a new buffer instance with the given data directory.
func New(dataDir string) (*Buffer, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, DefaultDBPath)

	// Check existing file size before opening
	if err := checkBufferSize(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	buffer := &Buffer{
		db:   db,
		path: dbPath,
	}

	// Initialize schema
	if err := buffer.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return buffer, nil
}

// Close closes the database connection.
func (b *Buffer) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// Store stores a result in the buffer.
func (b *Buffer) Store(result probe.Result) error {
	// Check buffer size before storing
	if err := checkBufferSize(b.path); err != nil {
		return err
	}

	// Generate unique ID for this result
	id := fmt.Sprintf("%s-%d", result.TaskID, time.Now().UnixNano())

	// Serialize result as JSON
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	// Insert into database
	_, err = b.db.Exec(`
		INSERT INTO results (id, task_id, payload, created_at)
		VALUES (?, ?, ?, ?)
	`, id, result.TaskID, string(payload), result.Timestamp.Unix())

	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}

	return nil
}

// MarkSent marks a result as successfully sent to the gateway.
func (b *Buffer) MarkSent(id string) error {
	_, err := b.db.Exec(`
		UPDATE results SET sent_at = ? WHERE id = ?
	`, time.Now().Unix(), id)

	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}

	return nil
}

// Pending returns all results that haven't been sent yet.
func (b *Buffer) Pending() ([]PendingResult, error) {
	rows, err := b.db.Query(`
		SELECT id, task_id, payload, created_at
		FROM results
		WHERE sent_at IS NULL
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query pending: %w", err)
	}
	defer rows.Close()

	var results []PendingResult
	for rows.Next() {
		var pr PendingResult
		var payload string
		var createdAt int64

		err := rows.Scan(&pr.ID, &pr.TaskID, &payload, &createdAt)
		if err != nil {
			continue // Skip malformed rows
		}

		// Deserialize result
		if err := json.Unmarshal([]byte(payload), &pr.Result); err != nil {
			continue // Skip malformed results
		}

		pr.CreatedAt = time.Unix(createdAt, 0)
		results = append(results, pr)
	}

	return results, nil
}

// Cleanup removes old results from the buffer.
func (b *Buffer) Cleanup(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Unix()

	_, err := b.db.Exec(`
		DELETE FROM results
		WHERE sent_at IS NOT NULL AND created_at < ?
	`, cutoff)

	if err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}

	// Vacuum to reclaim space
	_, err = b.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// Stats returns buffer statistics.
func (b *Buffer) Stats() (BufferStats, error) {
	var stats BufferStats

	// Count pending results
	err := b.db.QueryRow("SELECT COUNT(*) FROM results WHERE sent_at IS NULL").Scan(&stats.PendingCount)
	if err != nil {
		return stats, fmt.Errorf("count pending: %w", err)
	}

	// Count sent results
	err = b.db.QueryRow("SELECT COUNT(*) FROM results WHERE sent_at IS NOT NULL").Scan(&stats.SentCount)
	if err != nil {
		return stats, fmt.Errorf("count sent: %w", err)
	}

	// Get file size
	if info, err := os.Stat(b.path); err == nil {
		stats.SizeBytes = info.Size()
	}

	return stats, nil
}

// initSchema creates the database schema if it doesn't exist.
func (b *Buffer) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS results (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			payload TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			sent_at INTEGER
		);

		CREATE INDEX IF NOT EXISTS idx_results_sent_at ON results(sent_at);
		CREATE INDEX IF NOT EXISTS idx_results_created_at ON results(created_at);
	`

	_, err := b.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}

// checkBufferSize verifies that the buffer file hasn't exceeded the size limit.
func checkBufferSize(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil // File doesn't exist yet, OK to create
	}
	if err != nil {
		return fmt.Errorf("stat buffer file: %w", err)
	}

	if info.Size() > MaxBufferSize {
		return ErrBufferFull
	}

	return nil
}

// PendingResult represents a result stored in the buffer.
type PendingResult struct {
	ID        string       `json:"id"`
	TaskID    string       `json:"task_id"`
	Result    probe.Result `json:"result"`
	CreatedAt time.Time    `json:"created_at"`
}

// BufferStats contains buffer usage statistics.
type BufferStats struct {
	PendingCount int   `json:"pending_count"`
	SentCount    int   `json:"sent_count"`
	SizeBytes    int64 `json:"size_bytes"`
}