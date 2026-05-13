package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/packages/db/gen/idcdmain"
)

// AuditLogRepository wraps audit_log sqlc queries.
type AuditLogRepository struct {
	q *idcdmain.Queries
}

// NewAuditLogRepository returns an AuditLogRepository backed by the given pool.
func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{q: idcdmain.New(pool)}
}

func (r *AuditLogRepository) Create(ctx context.Context, p idcdmain.CreateAuditLogParams) error {
	if err := r.q.CreateAuditLog(ctx, p); err != nil {
		return fmt.Errorf("auditlog.Create: %w", err)
	}
	return nil
}

func (r *AuditLogRepository) ListByOwner(ctx context.Context, p idcdmain.ListAuditLogsByOwnerParams) ([]idcdmain.AuditLog, error) {
	rows, err := r.q.ListAuditLogsByOwner(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("auditlog.ListByOwner: %w", err)
	}
	return rows, nil
}

func (r *AuditLogRepository) ListByActor(ctx context.Context, p idcdmain.ListAuditLogsByActorParams) ([]idcdmain.AuditLog, error) {
	rows, err := r.q.ListAuditLogsByActor(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("auditlog.ListByActor: %w", err)
	}
	return rows, nil
}

func (r *AuditLogRepository) ListByResource(ctx context.Context, p idcdmain.ListAuditLogsByResourceParams) ([]idcdmain.AuditLog, error) {
	rows, err := r.q.ListAuditLogsByResource(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("auditlog.ListByResource: %w", err)
	}
	return rows, nil
}
