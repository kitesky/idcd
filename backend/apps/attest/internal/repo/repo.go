// Package repo holds the Postgres data-access layer for the
// idcd_attest.* tables that back the S2 Evidence / Attestation service
// (see docs/prd/18-evidence-and-attestation.md and DECISIONS.md D4/D5).
//
// Per CLAUDE.md D1, this layer never writes cross-schema FKs; the service
// layer joins idcd_attest.* rows to idcd_main.* rows in application code.
// Methods are intentionally narrow — CRUD plus the atomic state-machine
// helpers consumed by the verdict worker — and contain no business logic.
//
// Mirrors apps/cert-svc/internal/repo in shape: every repo takes a small
// Pool interface that both *pgxpool.Pool and pgxmock satisfy, so unit
// tests and production wiring exercise the same concrete types.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the minimal pgx surface every repo needs. Both *pgxpool.Pool
// and pgxmock.PgxPoolIface satisfy it, so production wiring and unit
// tests share the same concrete repos.
type Pool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Compile-time guarantee that the production pool satisfies our Pool
// interface — catches drift in the pgx API at build time.
var _ Pool = (*pgxpool.Pool)(nil)

// Repos aggregates one repository per idcd_attest.* table. Construct once
// at startup with New and pass into the service / worker layers.
type Repos struct {
	Orders             *VerdictOrdersRepo
	Reports            *VerdictReportsRepo
	AttestationRecords *AttestationRecordsRepo
	TSAResponses       *TsaResponsesRepo
	KeyCeremonyLog     *KeyCeremonyLogRepo
}

// New wires every per-table repo over the given Pool. Accepts the
// minimal Pool interface so tests can pass a pgxmock pool instead of a
// real *pgxpool.Pool.
func New(pool Pool) *Repos {
	return &Repos{
		Orders:             &VerdictOrdersRepo{pool: pool},
		Reports:            &VerdictReportsRepo{pool: pool},
		AttestationRecords: &AttestationRecordsRepo{pool: pool},
		TSAResponses:       &TsaResponsesRepo{pool: pool},
		KeyCeremonyLog:     &KeyCeremonyLogRepo{pool: pool},
	}
}

// Shared errors. Callers branch on these via errors.Is.
//
// Note: AttestationRecordsRepo deliberately uses the sentinel errors
// declared in lib/attest/record (ErrDuplicateAction / ErrNotFound /
// ErrInvalidTransition) so it satisfies the record.Repository interface.
// These local sentinels apply to the other four repos.
var (
	// ErrNotFound is returned when a single-row lookup matches zero rows.
	ErrNotFound = errors.New("repo: not found")
	// ErrConflict is returned when an INSERT trips a UNIQUE constraint.
	ErrConflict = errors.New("repo: unique constraint violated")
	// ErrInvalidStatus is returned by optimistic-locked status updates
	// when zero rows match, meaning the caller's "from" status no longer
	// reflects the row in the DB.
	ErrInvalidStatus = errors.New("repo: invalid status transition")
)

// pgUniqueViolation is the SQLSTATE for "unique_violation".
const pgUniqueViolation = "23505"

// isUniqueViolation returns true when err wraps a pgx unique-violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolation
	}
	return false
}

// rowScanner is the slice of pgx.Row and pgx.Rows we use for scanning —
// pgx.Row.Scan and pgx.Rows.Scan share this signature.
type rowScanner interface {
	Scan(dest ...any) error
}

// ---- VerdictOrder status enum ---------------------------------------------

// Order status values mirror idcd_attest.verdict_order.status (see
// migration 00002 and STATE-MACHINES.md). D5: refund_failed escalates to
// admin dashboard + P0 alert.
const (
	OrderStatusPending      = "pending"
	OrderStatusPaid         = "paid"
	OrderStatusGenerating   = "generating"
	OrderStatusDelivered    = "delivered"
	OrderStatusFailed       = "failed"
	OrderStatusRefunded     = "refunded"
	OrderStatusRefundFailed = "refund_failed"
)

// ---- Row projections ------------------------------------------------------

// Order is the in-memory projection of one idcd_attest.verdict_order row.
type Order struct {
	ID                  string
	OwnerID             string
	Template            string // sla|incident|compliance|legal
	Target              string // domain|url|ip
	TimeWindowStart     time.Time
	TimeWindowEnd       time.Time
	Status              string
	PriceCNY            float64
	PricePaidCNY        *float64
	ExtOrderID       *string
	RefundReason        *string
	RefundAttemptCount  int
	RefundLastError     *string
	RefundApologySentAt *time.Time
	CreatedAt           time.Time
	PaidAt              *time.Time
	DeliveredAt         *time.Time
	FailedAt            *time.Time
	RefundedAt          *time.Time
}

// Report is the in-memory projection of one idcd_attest.verdict_report
// row. nodes_used and blockchain_anchor are raw JSON bytes; callers
// unmarshal at the service layer.
type Report struct {
	ID                  string
	OrderID             string
	PDFURL              string
	PDFSizeBytes        *int64
	ContentHash         string
	Signature           []byte
	SignatureKeyID      string
	SignatureKeyVersion int
	TSAProvider         string
	TSAResponseBlob     []byte
	TSATime             time.Time
	BlockchainAnchor    []byte // jsonb raw
	NodesUsed           []byte // jsonb raw
	NodeConsistencyPct  *float64
	LLMUsed             bool
	LLMModel            *string
	LLMPromptVersion    *string
	SelfVerifyStatus    *string
	SelfVerifyAt        *time.Time
	ConfidenceLabel     *string
	ReportType          string
	ArchivedURL         *string
	CreatedAt           time.Time
}

// TSAResponse is the in-memory projection of one idcd_attest.tsa_response
// row. used_by_report_id may be nil for free-standing TSA probes that
// were not bound to a specific report.
type TSAResponse struct {
	ID             string
	Provider       string
	RequestHash    string
	ResponseBlob   []byte
	SerialNumber   *string
	IssuedAt       *time.Time
	ValidUntil     *time.Time
	Status         string // success|failure|timeout
	LatencyMS      *int
	UsedByReportID *string
	CreatedAt      time.Time
}

// KeyCeremony is the in-memory projection of one
// idcd_attest.key_ceremony_log row. Actors is raw jsonb (an array of
// {user_id|external_id, role} objects); the service layer unmarshals.
type KeyCeremony struct {
	ID          string
	Action      string // root_gen|root_split|sign_key_rotate|emergency_revoke
	KeyID       *string
	KeyVersion  *int
	Actors      []byte // jsonb raw
	EvidenceURL *string
	Notes       *string
	CreatedAt   time.Time
}
