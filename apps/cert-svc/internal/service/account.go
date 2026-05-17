package service

import (
	"context"
	"crypto"
	"errors"
	"fmt"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/vault"
)

// AccountManager persists the platform's ACME-account private key for
// each (CA, env) pair. The key is generated via Vault (envelope-encrypted
// for S1, KMS for S2) and the wrapped EncryptedKey is stored in
// cert.acme_accounts.key_kms_handle using the same base64(json) handle
// format we use for cert.certs.
//
// account_url is left empty on first insert — lego registers the account
// on the first RequestCertificate call and we currently do not persist
// the returned URL. S2 may extend this to round-trip the URL.
type AccountManager struct {
	repos *repo.Repos
	vault vault.Vault
}

// NewAccountManager wires an AccountManager over the cert.* repos and a
// Vault implementation. Both are required.
func NewAccountManager(repos *repo.Repos, vlt vault.Vault) *AccountManager {
	return &AccountManager{repos: repos, vault: vlt}
}

// GetOrCreate returns the persisted ACME account signer for (ca, env).
// On first call for a given pair the manager generates a fresh ECDSA P256
// key, encrypts it via Vault and inserts a cert.acme_accounts row;
// subsequent calls load and decrypt the existing row.
//
// The returned crypto.Signer is the long-lived account key the orchestrator
// hands to lego. Callers must not assume the key changes between calls.
func (a *AccountManager) GetOrCreate(ctx context.Context, ca, env string) (crypto.Signer, error) {
	if a == nil || a.repos == nil || a.vault == nil {
		return nil, fmt.Errorf("account manager: not configured")
	}
	if ca == "" || env == "" {
		return nil, fmt.Errorf("account manager: ca and env required")
	}

	existing, err := a.repos.ACMEAccounts.GetByCAEnv(ctx, ca, env)
	if err == nil {
		ek, decodeErr := DecodeKeyHandle(existing.KeyKMSHandle)
		if decodeErr != nil {
			return nil, fmt.Errorf("account manager: decode handle: %w", decodeErr)
		}
		plain, decErr := a.vault.DecryptKey(ctx, ek)
		if decErr != nil {
			return nil, fmt.Errorf("account manager: decrypt: %w", decErr)
		}
		signer, parseErr := DecodeAccountKey(plain)
		if parseErr != nil {
			return nil, fmt.Errorf("account manager: parse: %w", parseErr)
		}
		return signer, nil
	}
	if !errors.Is(err, repo.ErrNotFound) {
		return nil, fmt.Errorf("account manager: lookup: %w", err)
	}

	plain, ek, genErr := a.vault.GenerateKey(ctx, vault.KeyAlgECDSAP256)
	if genErr != nil {
		return nil, fmt.Errorf("account manager: generate: %w", genErr)
	}
	handle, encErr := encodeKeyHandle(ek)
	if encErr != nil {
		return nil, fmt.Errorf("account manager: encode handle: %w", encErr)
	}
	row := &repo.ACMEAccount{
		CA:           ca,
		Env:          env,
		AccountURL:   "",
		KeyKMSHandle: handle,
	}
	if _, insErr := a.repos.ACMEAccounts.Insert(ctx, row); insErr != nil {
		// Insert lost the race with another worker — fall back to a fresh
		// lookup so both workers converge on the same persisted key.
		if errors.Is(insErr, repo.ErrConflict) {
			return a.GetOrCreate(ctx, ca, env)
		}
		return nil, fmt.Errorf("account manager: insert: %w", insErr)
	}
	signer, parseErr := DecodeAccountKey(plain)
	if parseErr != nil {
		return nil, fmt.Errorf("account manager: parse new: %w", parseErr)
	}
	return signer, nil
}
