// Package record defines the attestation_record WAL types, sentinel
// errors and the Replayer helper used by the S2 Evidence/Attestation
// workers (see docs/prd/18-evidence-and-attestation.md §3.2 and
// DECISIONS.md D4).
//
// Design intent:
//
//   - Each verdict-generation step (KMS sign, RFC3161 TSA stamp, WORM
//     archive, blockchain anchor, self-verify, revoke) writes a single
//     row keyed by UNIQUE(report_id, action). Workers consult the WAL
//     before running a step so a crashed/retried worker never repeats
//     an externally-effecting call (D4).
//
//   - This package is the pure type layer. The Repository implementation
//     lives in apps/attest; only the interface is exposed here so the
//     worker / self-verify code is decoupled from the DB.
package record

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"time"
)

// Action is a single step identifier in the WAL. step-level
// UNIQUE(report_id, action) provides idempotency. The enum is fixed —
// adding a new step requires extending this list first.
type Action string

const (
	ActionSigned       Action = "signed"        // KMS sign completed
	ActionTSAStamped   Action = "tsa_stamped"   // RFC3161 TSA stamp completed
	ActionAnchored     Action = "anchored"      // Blockchain anchor (optional, S3+)
	ActionS3Archived   Action = "s3_archived"   // WORM archive completed
	ActionSelfVerified Action = "self_verified" // Self-Verify worker re-verified
	ActionRevoked      Action = "revoked"       // Revocation
)

// Status is the final state of a WAL row. pending exists only briefly
// between worker crash and the next replay; final rows are written as
// success or failure.
type Status string

const (
	StatusPending Status = "pending"
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

// Result mirrors Status; the D4 DDL keeps both columns (status drives
// the state machine, result is used for query filters). New code uses
// Status as the source of truth and fills Result in lockstep.
type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

// Record corresponds to one row of idcd_attest.attestation_record.
//
// D4 note on idempotency: today the Replayer.Record() helper does not
// populate IdempotencyKey or PayloadHash — they are kept on the row for
// forensic auditing and reserved for direct Repository.Insert callers
// that want richer rows. Replay safety is already guaranteed by the
// UNIQUE(report_id, action) constraint plus the in-process idempotency
// caches in lib/attest/sign/* adapters, so the columns are currently
// NULL in practice. Do not remove them: future workers will populate
// them once we wire deterministic-key derivation into the WAL helper.
type Record struct {
	ID             string    // att_*
	ReportID       string    // vr_*
	Action         Action
	Status         Status
	ExternalID     string    // TSA serial / KMS req id / S3 ETag / chain tx hash; may be empty
	IdempotencyKey string    // Reserved (see D4 note above); Replayer leaves empty.
	PayloadHash    string    // Reserved (see D4 note above); Replayer leaves empty.
	Result         Result
	ErrorDetail    string    // Filled when Status == failure
	RetryCount     int       // <= MaxRetries
	CreatedAt      time.Time
	CompletedAt    *time.Time
}

// MaxRetries is the D4-locked per-step retry ceiling. Beyond this the
// worker must route the report to DLQ and stop retrying.
const MaxRetries = 3

// Sentinel errors. Callers should compare with errors.Is.
var (
	// ErrDuplicateAction is returned by Repository.Insert when the
	// UNIQUE(report_id, action) constraint fires. Callers should then
	// Get the existing row and decide whether to skip or treat the
	// flow as already complete — never blindly retry Insert.
	ErrDuplicateAction = errors.New("attestation record: duplicate (report_id, action)")

	// ErrNotFound is returned by Repository.Get when the row is absent.
	ErrNotFound = errors.New("attestation record: not found")

	// ErrMaxRetriesExceeded is returned by Replayer.ShouldRun when a
	// failed row has already been retried MaxRetries times. The caller
	// must hand the report to the DLQ.
	ErrMaxRetriesExceeded = errors.New("attestation record: max retries exceeded; route to DLQ")

	// ErrInvalidTransition is returned when the caller attempts an
	// illegal status change (e.g. success → pending).
	ErrInvalidTransition = errors.New("attestation record: invalid status transition")
)

// Repository is the data-access abstraction for attestation_record.
// The concrete implementation lives in apps/attest; this package only
// publishes the interface so worker / self-verify code does not depend
// on a DB driver.
type Repository interface {
	// Insert writes a new WAL row. Status must be pending or a terminal
	// state. If (report_id, action) already exists the implementation
	// must return ErrDuplicateAction so the caller can fall back to
	// reading the existing row.
	Insert(ctx context.Context, r *Record) error

	// Get fetches the row for (report_id, action). Missing → ErrNotFound.
	Get(ctx context.Context, reportID string, action Action) (*Record, error)

	// Update permits only pending → success/failure, or an increment of
	// retry_count. Anything else must return ErrInvalidTransition.
	Update(ctx context.Context, r *Record) error

	// ListByReport returns every WAL row for a report ordered by
	// created_at ASC, used by workers to replay the step graph.
	ListByReport(ctx context.Context, reportID string) ([]*Record, error)
}

// Replayer is the worker-facing helper. Workers call ShouldRun before
// every step to consult the WAL, then Record the outcome.
type Replayer struct {
	Repo Repository
}

// ShouldRun consults the WAL for (reportID, action) and tells the
// caller what to do:
//
//   - cont=true, existingExternalID="", err=nil:
//     the step has not run (or last attempt failed and has retries
//     left). Caller executes the step then calls Record.
//
//   - cont=false, existingExternalID=<value>, err=nil:
//     the step already succeeded. Caller reuses externalID and skips.
//
//   - cont=false, existingExternalID="", err=ErrMaxRetriesExceeded:
//     prior failures have exhausted MaxRetries. Caller must DLQ.
//
//   - cont=false, existingExternalID="", err=<other>:
//     repository error; propagate.
func (r *Replayer) ShouldRun(ctx context.Context, reportID string, action Action) (cont bool, existingExternalID string, err error) {
	row, err := r.Repo.Get(ctx, reportID, action)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return true, "", nil
		}
		return false, "", err
	}

	switch row.Status {
	case StatusSuccess:
		return false, row.ExternalID, nil
	case StatusFailure:
		if row.RetryCount >= MaxRetries {
			return false, "", ErrMaxRetriesExceeded
		}
		return true, "", nil
	case StatusPending:
		// Pending row left behind by a crashed worker: re-run the step.
		// Treat it like failure with retries left so the caller drives
		// the row to a terminal state.
		if row.RetryCount >= MaxRetries {
			return false, "", ErrMaxRetriesExceeded
		}
		return true, "", nil
	default:
		return false, "", fmt.Errorf("attestation record: unknown status %q", row.Status)
	}
}

// Record persists the outcome of a step. status must be a terminal
// state (success or failure); pending is rejected with
// ErrInvalidTransition.
//
// Strategy: try Insert first (the common case is a fresh terminal row).
// On ErrDuplicateAction (a pending or prior-failure row already exists)
// fall back to Update so retry_count is bumped and status flipped.
// Any other Insert error is propagated.
func (r *Replayer) Record(ctx context.Context, reportID string, action Action, status Status, externalID string, errorDetail string) error {
	if status != StatusSuccess && status != StatusFailure {
		return ErrInvalidTransition
	}

	now := time.Now().UTC()
	completed := now

	row := &Record{
		ID:          NewRecordID(),
		ReportID:    reportID,
		Action:      action,
		Status:      status,
		ExternalID:  externalID,
		Result:      statusToResult(status),
		ErrorDetail: errorDetail,
		CreatedAt:   now,
		CompletedAt: &completed,
	}

	err := r.Repo.Insert(ctx, row)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrDuplicateAction) {
		return err
	}

	// Duplicate: read existing row, bump retry, flip to terminal state.
	existing, getErr := r.Repo.Get(ctx, reportID, action)
	if getErr != nil {
		return getErr
	}

	existing.Status = status
	existing.Result = statusToResult(status)
	existing.ExternalID = externalID
	existing.ErrorDetail = errorDetail
	existing.RetryCount++
	existing.CompletedAt = &completed

	return r.Repo.Update(ctx, existing)
}

func statusToResult(s Status) Result {
	if s == StatusSuccess {
		return ResultSuccess
	}
	return ResultFailure
}

// recordIDEncoding is unpadded lowercase base32 (RFC 4648 alphabet
// lowercased). 15 random bytes → 24 base32 chars.
var recordIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewRecordID returns "att_" + 24 base32 chars sourced from crypto/rand.
// Any rand failure is unrecoverable (ID generation is a system
// invariant) and panics.
func NewRecordID() string {
	var b [15]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("attestation record: crypto/rand failed: %w", err))
	}
	return "att_" + toLower(recordIDEncoding.EncodeToString(b[:]))
}

// toLower lowercases ASCII A-Z; avoids strings import to keep zero
// stdlib-only deps obvious.
func toLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
