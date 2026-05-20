package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// VerdictReportsRepo is the idcd_attest.verdict_report data-access
// surface. One row per generated PDF report; UNIQUE(order_id) prevents
// double-issuing. The signature / TSA blobs live here directly so the
// public /verify endpoint can re-check without round-tripping to KMS.
type VerdictReportsRepo struct {
	pool Pool
}

const verdictReportColumns = `id, order_id, pdf_url, pdf_size_bytes,
	content_hash, signature, signature_key_id, signature_key_version,
	tsa_provider, tsa_response_blob, tsa_time,
	blockchain_anchor, nodes_used, node_consistency_pct,
	llm_used, llm_model, llm_prompt_version,
	self_verify_status, self_verify_at,
	confidence_label, report_type, archived_url, created_at`

const verdictReportInsertSQL = `
	INSERT INTO idcd_attest.verdict_report (
		id, order_id, pdf_url, pdf_size_bytes,
		content_hash, signature, signature_key_id, signature_key_version,
		tsa_provider, tsa_response_blob, tsa_time,
		blockchain_anchor, nodes_used, node_consistency_pct,
		llm_used, llm_model, llm_prompt_version,
		self_verify_status, self_verify_at,
		confidence_label, report_type, archived_url, created_at
	) VALUES (
		$1, $2, $3, $4,
		$5, $6, $7, $8,
		$9, $10, $11,
		$12, $13, $14,
		$15, $16, $17,
		$18, $19,
		$20, $21, $22, COALESCE($23, now())
	)
	RETURNING id
`

// Insert persists a new verdict_report. The id (vr_*) must be generated
// by the caller and is echoed back on success. Returns ErrConflict on
// UNIQUE(order_id) violation — i.e. the order has already been
// delivered.
func (r *VerdictReportsRepo) Insert(ctx context.Context, rep *Report) (string, error) {
	var (
		id        string
		createdAt any
	)
	if !rep.CreatedAt.IsZero() {
		createdAt = rep.CreatedAt
	}
	err := r.pool.QueryRow(ctx, verdictReportInsertSQL,
		rep.ID,
		rep.OrderID,
		rep.PDFURL,
		rep.PDFSizeBytes,
		rep.ContentHash,
		rep.Signature,
		rep.SignatureKeyID,
		rep.SignatureKeyVersion,
		rep.TSAProvider,
		rep.TSAResponseBlob,
		rep.TSATime,
		rep.BlockchainAnchor,
		rep.NodesUsed,
		rep.NodeConsistencyPct,
		rep.LLMUsed,
		rep.LLMModel,
		rep.LLMPromptVersion,
		rep.SelfVerifyStatus,
		rep.SelfVerifyAt,
		rep.ConfidenceLabel,
		rep.ReportType,
		rep.ArchivedURL,
		createdAt,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return "", ErrConflict
		}
		return "", fmt.Errorf("verdict_report insert: %w", err)
	}
	rep.ID = id
	return id, nil
}

const verdictReportGetByIDSQL = `
	SELECT ` + verdictReportColumns + `
	FROM idcd_attest.verdict_report
	WHERE id = $1
`

// GetByID returns the report with the given id, or ErrNotFound.
func (r *VerdictReportsRepo) GetByID(ctx context.Context, id string) (*Report, error) {
	row := r.pool.QueryRow(ctx, verdictReportGetByIDSQL, id)
	rep, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("verdict_report get: %w", err)
	}
	return rep, nil
}

const verdictReportGetByOrderIDSQL = `
	SELECT ` + verdictReportColumns + `
	FROM idcd_attest.verdict_report
	WHERE order_id = $1
`

// GetByOrderID returns the (unique) report attached to the given order,
// or ErrNotFound when generation has not yet completed.
func (r *VerdictReportsRepo) GetByOrderID(ctx context.Context, orderID string) (*Report, error) {
	row := r.pool.QueryRow(ctx, verdictReportGetByOrderIDSQL, orderID)
	rep, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("verdict_report get by order: %w", err)
	}
	return rep, nil
}

const verdictReportUpdateSelfVerifySQL = `
	UPDATE idcd_attest.verdict_report
	SET self_verify_status = $1, self_verify_at = $2
	WHERE id = $3
`

// UpdateSelfVerify records the Self-Verify worker's outcome (D6: this
// worker runs in an independent VPC subnet and only consumes the public
// /verify API). status is pass | fail | pending.
func (r *VerdictReportsRepo) UpdateSelfVerify(ctx context.Context, id, status string, at time.Time) error {
	tag, err := r.pool.Exec(ctx, verdictReportUpdateSelfVerifySQL, status, at, id)
	if err != nil {
		return fmt.Errorf("verdict_report update self_verify: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanReport(r rowScanner) (*Report, error) {
	var rep Report
	if err := r.Scan(
		&rep.ID,
		&rep.OrderID,
		&rep.PDFURL,
		&rep.PDFSizeBytes,
		&rep.ContentHash,
		&rep.Signature,
		&rep.SignatureKeyID,
		&rep.SignatureKeyVersion,
		&rep.TSAProvider,
		&rep.TSAResponseBlob,
		&rep.TSATime,
		&rep.BlockchainAnchor,
		&rep.NodesUsed,
		&rep.NodeConsistencyPct,
		&rep.LLMUsed,
		&rep.LLMModel,
		&rep.LLMPromptVersion,
		&rep.SelfVerifyStatus,
		&rep.SelfVerifyAt,
		&rep.ConfidenceLabel,
		&rep.ReportType,
		&rep.ArchivedURL,
		&rep.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &rep, nil
}
