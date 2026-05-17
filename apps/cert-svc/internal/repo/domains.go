package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DomainsRepo manages cert.domains — the per-account FQDN registry and
// CAA cache. Upsert is used by the validator after a fresh CAA probe;
// Get is used by the issuer to read a cached result before issuing.
type DomainsRepo struct {
	pool Pool
}

const domainsColumns = `id, account_id, fqdn, caa_status, caa_checked_at, created_at`

const domainsUpsertSQL = `
	INSERT INTO cert.domains (account_id, fqdn, caa_status, caa_checked_at)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (account_id, fqdn)
	DO UPDATE SET
		caa_status     = COALESCE(EXCLUDED.caa_status, cert.domains.caa_status),
		caa_checked_at = COALESCE(EXCLUDED.caa_checked_at, cert.domains.caa_checked_at)
	RETURNING ` + domainsColumns + `
`

// Upsert inserts a (account_id, fqdn) row, or refreshes its CAA cache
// fields when one already exists. Pass caaStatus == nil to merely
// register the domain without touching the cache.
func (r *DomainsRepo) Upsert(ctx context.Context, accountID int64, fqdn string, caaStatus *string) (*Domain, error) {
	var checkedAt *time.Time
	if caaStatus != nil {
		now := time.Now().UTC()
		checkedAt = &now
	}
	row := r.pool.QueryRow(ctx, domainsUpsertSQL, accountID, fqdn, caaStatus, checkedAt)
	d, err := scanDomain(row)
	if err != nil {
		return nil, fmt.Errorf("domains upsert: %w", err)
	}
	return d, nil
}

const domainsGetSQL = `
	SELECT ` + domainsColumns + `
	FROM cert.domains
	WHERE account_id = $1 AND fqdn = $2
`

// Get returns the cert.domains row for (accountID, fqdn), or ErrNotFound.
func (r *DomainsRepo) Get(ctx context.Context, accountID int64, fqdn string) (*Domain, error) {
	row := r.pool.QueryRow(ctx, domainsGetSQL, accountID, fqdn)
	d, err := scanDomain(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("domains get: %w", err)
	}
	return d, nil
}

func scanDomain(r rowScanner) (*Domain, error) {
	var d Domain
	if err := r.Scan(
		&d.ID,
		&d.AccountID,
		&d.FQDN,
		&d.CAAStatus,
		&d.CAACheckedAt,
		&d.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &d, nil
}
