package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
)

// SessionRepository wraps user_session sqlc queries.
type SessionRepository struct {
	q *idcdmain.Queries
}

// NewSessionRepository returns a SessionRepository backed by the given pool.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{q: idcdmain.New(pool)}
}

func (r *SessionRepository) Create(ctx context.Context, p idcdmain.CreateSessionParams) (idcdmain.UserSession, error) {
	s, err := r.q.CreateSession(ctx, p)
	if err != nil {
		return idcdmain.UserSession{}, fmt.Errorf("session.Create: %w", err)
	}
	return s, nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (idcdmain.UserSession, error) {
	s, err := r.q.GetSessionByID(ctx, id)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.UserSession{}, ErrNotFound
		}
		return idcdmain.UserSession{}, fmt.Errorf("session.GetByID: %w", err)
	}
	return s, nil
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash string) (idcdmain.UserSession, error) {
	s, err := r.q.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if isNoRows(err) {
			return idcdmain.UserSession{}, ErrNotFound
		}
		return idcdmain.UserSession{}, fmt.Errorf("session.GetByTokenHash: %w", err)
	}
	return s, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, id string) error {
	if err := r.q.RevokeSession(ctx, id); err != nil {
		return fmt.Errorf("session.Revoke: %w", err)
	}
	return nil
}

func (r *SessionRepository) RevokeAll(ctx context.Context, userID string) error {
	if err := r.q.RevokeAllUserSessions(ctx, userID); err != nil {
		return fmt.Errorf("session.RevokeAll: %w", err)
	}
	return nil
}

func (r *SessionRepository) ListActive(ctx context.Context, userID string) ([]idcdmain.UserSession, error) {
	ss, err := r.q.ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("session.ListActive: %w", err)
	}
	return ss, nil
}

func (r *SessionRepository) PurgeExpired(ctx context.Context) error {
	if err := r.q.PurgeExpiredSessions(ctx); err != nil {
		return fmt.Errorf("session.PurgeExpired: %w", err)
	}
	return nil
}
