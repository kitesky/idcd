package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// observation is the minimal probe data point the orchestrator's
// cross-validation and PDF rendering consume. The production
// implementation (S2 W6+) will replace fetchObservations with a real
// TimescaleDB query that returns a richer struct; everywhere
// observation is consumed in this package is internal, so we can expand
// the shape without touching callers.
type observation struct {
	NodeID    string
	Timestamp time.Time
	Latency   time.Duration
	OK        bool
}

// fetchObservations stubs the TimescaleDB read in step 1 of the
// pipeline. It returns three fixed probe points so downstream stages
// get realistic-looking input. The function takes the order so the
// future real implementation can scope the query by target / time
// window without a signature change.
func fetchObservations(_ context.Context, order *Order) ([]observation, error) {
	if order == nil {
		return nil, fmt.Errorf("fetchObservations: nil order")
	}
	now := time.Now().UTC().Truncate(time.Second)
	return []observation{
		{NodeID: "node-cn-bj", Timestamp: now.Add(-3 * time.Second), Latency: 42 * time.Millisecond, OK: true},
		{NodeID: "node-cn-sh", Timestamp: now.Add(-2 * time.Second), Latency: 51 * time.Millisecond, OK: true},
		{NodeID: "node-cn-gz", Timestamp: now.Add(-1 * time.Second), Latency: 47 * time.Millisecond, OK: true},
	}, nil
}

// crossValidate stubs step 2. It collapses observations to (1) the set
// of distinct node IDs that responded and (2) a consistency percentage.
// In the S2 MVP "consistency" is just the fraction of OK observations;
// the real implementation will check that the substantive results
// (status code, response body hash, etc.) agree across nodes.
func crossValidate(_ context.Context, obs []observation) (nodes []string, consistencyPct float64) {
	if len(obs) == 0 {
		return nil, 0
	}
	seen := map[string]struct{}{}
	okCount := 0
	for _, o := range obs {
		if _, ok := seen[o.NodeID]; !ok {
			seen[o.NodeID] = struct{}{}
			nodes = append(nodes, o.NodeID)
		}
		if o.OK {
			okCount++
		}
	}
	consistencyPct = float64(okCount) / float64(len(obs)) * 100
	return nodes, consistencyPct
}

// renderPDF stubs step 4. The real implementation will use a templated
// HTML→PDF renderer; here we hand-build the minimal PDF 1.4 layout that
// satisfies the magic-byte check in lib/attest/pdfsign and parses
// cleanly via github.com/digitorus/pdf.
//
// The byte layout is copied from the test fixture in
// lib/attest/pdfsign/pdfsign_test.go (minimalPDF) — DO NOT edit without
// recomputing xref offsets.
func renderPDF(_ *Order, _ []observation, _ []string) ([]byte, error) {
	const minimalPDF = "%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> >>\nendobj\n" +
		"xref\n" +
		"0 4\n" +
		"0000000000 65535 f \n" +
		"0000000009 00000 n \n" +
		"0000000058 00000 n \n" +
		"0000000115 00000 n \n" +
		"trailer\n<< /Size 4 /Root 1 0 R >>\n" +
		"startxref\n203\n" +
		"%%EOF\n"
	return []byte(minimalPDF), nil
}

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
