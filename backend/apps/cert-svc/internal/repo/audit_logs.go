package repo

import (
	"context"
	"fmt"
)

// AuditLogsRepo is the append-only writer for cert.audit_logs. The
// service layer calls Append on every mutating action; reads happen via
// out-of-band admin tooling, not this repo.
type AuditLogsRepo struct {
	pool Pool
}

const auditLogsInsertSQL = `
	INSERT INTO cert.audit_logs (
		account_id, actor, action, target_kind, target_id, payload_jsonb
	) VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, occurred_at
`

// Append writes one audit row. AccountID can be nil for system actions
// without a specific account context (e.g. KMS rotation). Payload is
// stored as JSONB; pass nil for empty.
func (r *AuditLogsRepo) Append(ctx context.Context, l *AuditLog) error {
	var payloadArg any
	if len(l.Payload) > 0 {
		payloadArg = l.Payload
	}
	err := r.pool.QueryRow(ctx, auditLogsInsertSQL,
		l.AccountID,
		l.Actor,
		l.Action,
		l.TargetKind,
		l.TargetID,
		payloadArg,
	).Scan(&l.ID, &l.OccurredAt)
	if err != nil {
		return fmt.Errorf("audit_logs append: %w", err)
	}
	return nil
}
