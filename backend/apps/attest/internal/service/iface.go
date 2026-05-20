// Package service implements the Verdict generation orchestrator for the
// attest-svc (see docs/prd/18-evidence-and-attestation.md §3.2).
//
// This file declares the consumer-defined interfaces the orchestrator
// depends on. The actual implementations live in apps/attest/internal/repo
// (DB-backed Order / Report repos) and in apps/attest/cmd/* (S3
// Archiver). Keeping the interfaces here decouples the orchestrator from
// any concrete driver so unit tests can wire in fakes and so the repo
// package can evolve its richer struct shapes independently.
package service

import (
	"context"
	"time"
)

// Order is the in-memory shape the orchestrator needs from verdict_order.
// The repo package returns its own richer struct; we only require the
// fields used by the 10-step pipeline.
type Order struct {
	ID              string
	OwnerID         string
	Template        string
	Target          string
	TimeWindowStart time.Time
	TimeWindowEnd   time.Time
	Status          string
	PriceCNY        float64
}

// Report is the in-memory shape the orchestrator needs from
// verdict_report. Mirrors the DDL in docs/prd/18 §3.1 but trimmed to the
// columns the orchestrator actually populates.
type Report struct {
	ID                  string
	OrderID             string
	PDFURL              string
	ContentHash         string
	Signature           []byte
	SignatureKeyID      string
	SignatureKeyVersion int
	TSAProvider         string
	TSATime             time.Time
	NodesUsed           []byte // JSON-encoded list of node IDs
	NodeConsistencyPct  float64
	SelfVerifyStatus    string
	ReportType          string
	ArchivedURL         string
	CreatedAt           time.Time
}

// OrderRepo is the subset of verdict_order operations the orchestrator
// drives. The real implementation lives in apps/attest/internal/repo.
type OrderRepo interface {
	GetByID(ctx context.Context, id string) (*Order, error)
	// UpdateStatus performs an optimistic transition from->to. errReason
	// is recorded only on the failed path; pass nil otherwise.
	UpdateStatus(ctx context.Context, id, from, to string, errReason *string) error
	SetDelivered(ctx context.Context, id string, t time.Time) error
	SetFailed(ctx context.Context, id string, t time.Time, reason string) error
}

// ReportRepo is the subset of verdict_report operations the orchestrator
// drives.
type ReportRepo interface {
	// Insert persists a new verdict_report row. Returns the persisted ID
	// (typically r.ID). Implementations may use ON CONFLICT (order_id) DO
	// NOTHING so an orchestrator replay does not double-write.
	Insert(ctx context.Context, r *Report) (string, error)
	// GetByOrderID returns the in-flight report for orderID, or nil with
	// a nil error if none exists yet. Implementations MUST NOT return a
	// sentinel "not found" error — nil/nil is the success-but-empty case.
	GetByOrderID(ctx context.Context, orderID string) (*Report, error)
}

// Archiver writes the signed PDF to WORM storage (S3 with Object Lock in
// production). Returns the stable URL the verifier should fetch from and
// the provider-supplied ETag (stored in attestation_record.external_id).
type Archiver interface {
	Archive(ctx context.Context, key string, pdf []byte) (url string, etag string, err error)
}
