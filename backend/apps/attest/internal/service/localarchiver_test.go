package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalArchiver_WritesFileAndEtag(t *testing.T) {
	dir := t.TempDir()
	a := NewLocalArchiver(dir)

	pdf := []byte("%PDF-1.4 fake")
	url, etag, err := a.Archive(context.Background(), "vr_test.pdf", pdf)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if !strings.HasPrefix(url, "file://") {
		t.Fatalf("url = %q, expected file:// prefix", url)
	}
	if len(etag) != 16 {
		t.Fatalf("etag length = %d, want 16", len(etag))
	}
	data, err := os.ReadFile(filepath.Join(dir, "vr_test.pdf"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(pdf) {
		t.Fatalf("written bytes mismatch")
	}
}

func TestLocalArchiver_EmptyDirRejected(t *testing.T) {
	a := NewLocalArchiver("")
	if _, _, err := a.Archive(context.Background(), "k", []byte("x")); err == nil {
		t.Fatalf("expected error for empty dir")
	}
}
