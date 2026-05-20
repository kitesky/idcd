package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// APIKeyRepository wraps api_key sqlc queries.
type APIKeyRepository struct {
	q *idcdmain.Queries
}

// NewAPIKeyRepository returns an APIKeyRepository backed by the given pool.
func NewAPIKeyRepository(pool *pgxpool.Pool) *APIKeyRepository {
	return &APIKeyRepository{q: idcdmain.New(pool)}
}

func (r *APIKeyRepository) Create(ctx context.Context, p idcdmain.CreateAPIKeyParams) (idcdmain.ApiKey, error) {
	k, err := r.q.CreateAPIKey(ctx, p)
	if err != nil {
		if isDuplicate(err) {
			return idcdmain.ApiKey{}, ErrDuplicate
		}
		return idcdmain.ApiKey{}, fmt.Errorf("apikey.Create: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepository) GetByID(ctx context.Context, id string) (idcdmain.ApiKey, error) {
	k, err := r.q.GetAPIKeyByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.ApiKey{}, ErrNotFound
		}
		return idcdmain.ApiKey{}, fmt.Errorf("apikey.GetByID: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (idcdmain.ApiKey, error) {
	k, err := r.q.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.ApiKey{}, ErrNotFound
		}
		return idcdmain.ApiKey{}, fmt.Errorf("apikey.GetByPrefix: %w", err)
	}
	return k, nil
}

func (r *APIKeyRepository) ListByOwner(ctx context.Context, ownerType, ownerID string) ([]idcdmain.ApiKey, error) {
	ks, err := r.q.ListAPIKeysByOwner(ctx, idcdmain.ListAPIKeysByOwnerParams{
		OwnerType: ownerType,
		OwnerID:   ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("apikey.ListByOwner: %w", err)
	}
	return ks, nil
}

func (r *APIKeyRepository) Revoke(ctx context.Context, id string) error {
	if err := r.q.RevokeAPIKey(ctx, id); err != nil {
		return fmt.Errorf("apikey.Revoke: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, p idcdmain.UpdateAPIKeyLastUsedParams) error {
	if err := r.q.UpdateAPIKeyLastUsed(ctx, p); err != nil {
		return fmt.Errorf("apikey.UpdateLastUsed: %w", err)
	}
	return nil
}

func (r *APIKeyRepository) ExpireStale(ctx context.Context) error {
	if err := r.q.ExpireAPIKey(ctx); err != nil {
		return fmt.Errorf("apikey.ExpireStale: %w", err)
	}
	return nil
}
