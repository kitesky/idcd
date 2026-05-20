package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	attestrec "github.com/kite365/idcd/lib/attest/record"
)

// AttestationRecordsRepo is the idcd_attest.attestation_record (WAL)
// data-access surface. D4: every verdict-generation step writes a single
// row keyed by UNIQUE(report_id, action); workers consult the WAL
// before running a step so a crashed/retried worker never repeats an
// externally-effecting call (KMS sign / TSA stamp / WORM archive).
//
// This repo must satisfy lib/attest/record.Repository — the compile-time
// assertion below guards against drift.
type AttestationRecordsRepo struct {
	pool Pool
}

var _ attestrec.Repository = (*AttestationRecordsRepo)(nil)

const attestationRecordColumns = `id, report_id, action, status,
	external_id, idempotency_key, payload_hash,
	result, error_detail, retry_count,
	created_at, completed_at`

const attestationRecordInsertSQL = `
	INSERT INTO idcd_attest.attestation_record (
		id, report_id, action, status,
		external_id, idempotency_key, payload_hash,
		result, error_detail, retry_count,
		created_at, completed_at
	) VALUES (
		$1, $2, $3, $4,
		$5, $6, $7,
		$8, $9, $10,
		COALESCE($11, now()), $12
	)
`

// Insert writes a new WAL row. UNIQUE(report_id, action) is enforced by
// the database; on conflict we return attestrec.ErrDuplicateAction so
// the caller can fall back to Get + Update via Replayer.Record.
//
// Note: this implementation returns the lib/attest/record sentinel
// errors (not the local repo sentinels) so it can satisfy the
// record.Repository contract that worker code consumes.
func (r *AttestationRecordsRepo) Insert(ctx context.Context, rec *attestrec.Record) error {
	if rec == nil {
		return fmt.Errorf("attestation_record insert: nil record")
	}
	var createdAt any
	if !rec.CreatedAt.IsZero() {
		createdAt = rec.CreatedAt
	}
	_, err := r.pool.Exec(ctx, attestationRecordInsertSQL,
		rec.ID,
		rec.ReportID,
		string(rec.Action),
		string(rec.Status),
		nullableString(rec.ExternalID),
		nullableString(rec.IdempotencyKey),
		nullableString(rec.PayloadHash),
		string(rec.Result),
		nullableString(rec.ErrorDetail),
		rec.RetryCount,
		createdAt,
		rec.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return attestrec.ErrDuplicateAction
		}
		return fmt.Errorf("attestation_record insert: %w", err)
	}
	return nil
}

const attestationRecordGetSQL = `
	SELECT ` + attestationRecordColumns + `
	FROM idcd_attest.attestation_record
	WHERE report_id = $1 AND action = $2
`

// Get fetches the WAL row for (reportID, action). Missing rows return
// attestrec.ErrNotFound so Replayer.ShouldRun can distinguish them from
// transport-level errors.
func (r *AttestationRecordsRepo) Get(ctx context.Context, reportID string, action attestrec.Action) (*attestrec.Record, error) {
	row := r.pool.QueryRow(ctx, attestationRecordGetSQL, reportID, string(action))
	rec, err := scanAttestationRecord(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, attestrec.ErrNotFound
		}
		return nil, fmt.Errorf("attestation_record get: %w", err)
	}
	return rec, nil
}

// attestationRecordUpdateSQL implements the D4 transition rules in pure
// SQL: only allow pending → success/failure, and require strictly
// monotonic retry_count. Anything else affects zero rows and triggers
// ErrInvalidTransition.
const attestationRecordUpdateSQL = `
	UPDATE idcd_attest.attestation_record
	SET status       = $1,
	    result       = $2,
	    external_id  = $3,
	    error_detail = $4,
	    retry_count  = $5,
	    completed_at = $6
	WHERE id = $7
	  AND status = 'pending'
	  AND $1 IN ('success', 'failure')
	  AND $5 > retry_count
`

// Update applies a terminal state to a WAL row. D4 transition rules
// (enforced in SQL):
//
//   - current status must be 'pending'
//   - new status must be 'success' or 'failure'
//   - retry_count must be strictly greater than the row's current value
//
// Any violation returns attestrec.ErrInvalidTransition.
func (r *AttestationRecordsRepo) Update(ctx context.Context, rec *attestrec.Record) error {
	if rec == nil {
		return fmt.Errorf("attestation_record update: nil record")
	}
	tag, err := r.pool.Exec(ctx, attestationRecordUpdateSQL,
		string(rec.Status),
		string(rec.Result),
		nullableString(rec.ExternalID),
		nullableString(rec.ErrorDetail),
		rec.RetryCount,
		rec.CompletedAt,
		rec.ID,
	)
	if err != nil {
		return fmt.Errorf("attestation_record update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return attestrec.ErrInvalidTransition
	}
	return nil
}

const attestationRecordListByReportSQL = `
	SELECT ` + attestationRecordColumns + `
	FROM idcd_attest.attestation_record
	WHERE report_id = $1
	ORDER BY created_at ASC
`

// ListByReport returns every WAL row for a report ordered by created_at
// ASC, used by workers to replay the step graph.
func (r *AttestationRecordsRepo) ListByReport(ctx context.Context, reportID string) ([]*attestrec.Record, error) {
	rows, err := r.pool.Query(ctx, attestationRecordListByReportSQL, reportID)
	if err != nil {
		return nil, fmt.Errorf("attestation_record list by report: %w", err)
	}
	defer rows.Close()

	out := make([]*attestrec.Record, 0)
	for rows.Next() {
		rec, scanErr := scanAttestationRecord(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("attestation_record list scan: %w", scanErr)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attestation_record list rows: %w", err)
	}
	return out, nil
}

func scanAttestationRecord(r rowScanner) (*attestrec.Record, error) {
	var (
		rec         attestrec.Record
		actionText  string
		statusText  string
		resultText  string
		extID       *string
		idemKey     *string
		payloadHash *string
		errDetail   *string
	)
	if err := r.Scan(
		&rec.ID,
		&rec.ReportID,
		&actionText,
		&statusText,
		&extID,
		&idemKey,
		&payloadHash,
		&resultText,
		&errDetail,
		&rec.RetryCount,
		&rec.CreatedAt,
		&rec.CompletedAt,
	); err != nil {
		return nil, err
	}
	rec.Action = attestrec.Action(actionText)
	rec.Status = attestrec.Status(statusText)
	rec.Result = attestrec.Result(resultText)
	rec.ExternalID = deref(extID)
	rec.IdempotencyKey = deref(idemKey)
	rec.PayloadHash = deref(payloadHash)
	rec.ErrorDetail = deref(errDetail)
	return &rec, nil
}

// nullableString returns nil when s is empty so empty strings round-trip
// as SQL NULL, matching the nullable columns in the DDL.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
