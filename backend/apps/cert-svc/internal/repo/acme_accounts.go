package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ACMEAccountsRepo handles cert.acme_accounts — the platform's own ACME
// registrations, one row per (CA, env). The worker resolves which row to
// use at task start; new rows are inserted by the bootstrap script.
type ACMEAccountsRepo struct {
	pool Pool
}

const acmeAccountsColumns = `id, ca, env, account_url, key_kms_handle,
	eab_kid, eab_hmac_kms_handle, created_at`

const acmeAccountsGetByCAEnvSQL = `
	SELECT ` + acmeAccountsColumns + `
	FROM cert.acme_accounts
	WHERE ca = $1 AND env = $2
`

// GetByCAEnv resolves the ACME account row for (ca, env), e.g.
// ("lets-encrypt", "prod"). Returns ErrNotFound when no row matches.
func (r *ACMEAccountsRepo) GetByCAEnv(ctx context.Context, ca, env string) (*ACMEAccount, error) {
	var a ACMEAccount
	err := r.pool.QueryRow(ctx, acmeAccountsGetByCAEnvSQL, ca, env).Scan(
		&a.ID,
		&a.CA,
		&a.Env,
		&a.AccountURL,
		&a.KeyKMSHandle,
		&a.EABKID,
		&a.EABHMACKMSHandle,
		&a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("acme_accounts get: %w", err)
	}
	return &a, nil
}

const acmeAccountsInsertSQL = `
	INSERT INTO cert.acme_accounts (
		ca, env, account_url, key_kms_handle, eab_kid, eab_hmac_kms_handle
	) VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, created_at
`

// Insert registers a new ACME account row. UNIQUE (ca, env) — caller
// should branch on ErrConflict.
func (r *ACMEAccountsRepo) Insert(ctx context.Context, a *ACMEAccount) (int64, error) {
	var (
		id        int64
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, acmeAccountsInsertSQL,
		a.CA,
		a.Env,
		a.AccountURL,
		a.KeyKMSHandle,
		a.EABKID,
		a.EABHMACKMSHandle,
	).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrConflict
		}
		return 0, fmt.Errorf("acme_accounts insert: %w", err)
	}
	a.ID = id
	a.CreatedAt = createdAt
	return id, nil
}
