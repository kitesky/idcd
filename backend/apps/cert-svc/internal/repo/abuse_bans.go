package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AbuseBan represents a single cert.abuse_bans row. Active = LiftedAt
// is nil; lifted = both LiftedAt / LiftedBy populated.
type AbuseBan struct {
	ID           int64
	AccountID    string
	Reason       string
	BannedBy     string
	BannedAt     time.Time
	LiftedAt     *time.Time
	LiftedBy     *string
	LiftedReason *string
}

// AbuseBansRepo writes / reads cert.abuse_bans. Used by the admin
// handler (Ban / Unban) and by the AbuseDetector (IsBanned) — both go
// through this repo so the partial-unique index is the only source of
// truth for "is this account active-banned?".
type AbuseBansRepo struct {
	pool Pool
}

// ErrAlreadyBanned is returned by Ban when the account already has an
// active (lifted_at IS NULL) row. Callers should treat this as a
// success-equivalent (idempotent admin action).
var ErrAlreadyBanned = errors.New("repo: account already banned")

// ErrNotBanned is returned by Lift when no active ban exists.
var ErrNotBanned = errors.New("repo: account is not currently banned")

const banInsertSQL = `
	INSERT INTO cert.abuse_bans (account_id, reason, banned_by)
	VALUES ($1, $2, $3)
	RETURNING id, banned_at
`

// Ban inserts a new active ban row. Trips ErrAlreadyBanned when the
// partial-unique index (account_id) WHERE lifted_at IS NULL collides.
func (r *AbuseBansRepo) Ban(ctx context.Context, accountID string, reason, by string) (*AbuseBan, error) {
	if by == "" {
		by = "admin"
	}
	ban := &AbuseBan{
		AccountID: accountID,
		Reason:    reason,
		BannedBy:  by,
	}
	err := r.pool.QueryRow(ctx, banInsertSQL, accountID, reason, by).
		Scan(&ban.ID, &ban.BannedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrAlreadyBanned
		}
		return nil, fmt.Errorf("abuse_bans insert: %w", err)
	}
	return ban, nil
}

const banActiveLookupSQL = `
	SELECT id, account_id, reason, banned_by, banned_at,
	       lifted_at, lifted_by, lifted_reason
	FROM cert.abuse_bans
	WHERE account_id = $1 AND lifted_at IS NULL
	LIMIT 1
`

// IsBanned returns true when an active ban exists for accountID. The
// returned ban is nil when no active row is found.
func (r *AbuseBansRepo) IsBanned(ctx context.Context, accountID string) (bool, *AbuseBan, error) {
	var b AbuseBan
	err := r.pool.QueryRow(ctx, banActiveLookupSQL, accountID).Scan(
		&b.ID, &b.AccountID, &b.Reason, &b.BannedBy, &b.BannedAt,
		&b.LiftedAt, &b.LiftedBy, &b.LiftedReason,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("abuse_bans lookup: %w", err)
	}
	return true, &b, nil
}

const banLiftSQL = `
	UPDATE cert.abuse_bans
	SET lifted_at = now(), lifted_by = $2, lifted_reason = $3
	WHERE account_id = $1 AND lifted_at IS NULL
`

// Lift marks the account's active ban as lifted. Returns ErrNotBanned
// when no active row exists.
func (r *AbuseBansRepo) Lift(ctx context.Context, accountID string, by, reason string) error {
	if by == "" {
		by = "admin"
	}
	tag, err := r.pool.Exec(ctx, banLiftSQL, accountID, by, reason)
	if err != nil {
		return fmt.Errorf("abuse_bans lift: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotBanned
	}
	return nil
}
