package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// localArchiver is the S2-MVP stub for the WORM archive step. It writes
// each signed PDF to dir/key on local disk and reports a file:// URL
// plus the first 16 hex chars of sha256(pdf) as a synthetic ETag.
//
// Production uses an S3 bucket with Object Lock (compliance mode); see
// docs/prd/18 §3.3.
type localArchiver struct {
	dir string
}

// NewLocalArchiver creates a localArchiver rooted at dir. dir is
// created (mkdir -p) on first Archive call; tests can pass a
// t.TempDir() for hermetic cleanup.
func NewLocalArchiver(dir string) Archiver { return &localArchiver{dir: dir} }

func (a *localArchiver) Archive(_ context.Context, key string, pdf []byte) (string, string, error) {
	if a.dir == "" {
		return "", "", fmt.Errorf("localArchiver: dir is empty")
	}
	if err := os.MkdirAll(a.dir, 0o755); err != nil {
		return "", "", fmt.Errorf("localArchiver: mkdir %s: %w", a.dir, err)
	}
	path := filepath.Join(a.dir, key)
	if err := os.WriteFile(path, pdf, 0o644); err != nil {
		return "", "", fmt.Errorf("localArchiver: write %s: %w", path, err)
	}
	sum := sha256.Sum256(pdf)
	etag := hex.EncodeToString(sum[:])[:16]
	return "file://" + path, etag, nil
}
