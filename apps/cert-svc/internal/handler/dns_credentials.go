package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/vault"
)

func mountDNSCredentials(r chi.Router, deps Deps) {
	r.Post("/dns-credentials", createDNSCredential(deps))
	r.Get("/dns-credentials", listDNSCredentials(deps))
	r.Delete("/dns-credentials/{id}", deleteDNSCredential(deps))
	r.Post("/dns-credentials/{id}/health-check", healthCheckDNSCredential(deps))
}

type createDNSCredentialRequest struct {
	Provider    string            `json:"provider"`
	DisplayName string            `json:"display_name"`
	Secrets     map[string]string `json:"secrets"`
}

type dnsCredentialResponse struct {
	ID              int64   `json:"id"`
	Provider        string  `json:"provider"`
	DisplayName     string  `json:"display_name"`
	HealthStatus    string  `json:"health_status"`
	HealthCheckedAt *string `json:"health_checked_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	RevokedAt       *string `json:"revoked_at,omitempty"`
}

func createDNSCredential(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Vault == nil || deps.DNSReg == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"dns deps not configured", nil)
			return
		}
		var req createDNSCredentialRequest
		if !readJSON(w, r, &req) {
			return
		}
		if req.Provider == "" {
			writeErr(w, http.StatusBadRequest, codeBadRequest, "provider is required", nil)
			return
		}
		if req.DisplayName == "" {
			writeErr(w, http.StatusBadRequest, codeBadRequest, "display_name is required", nil)
			return
		}
		if req.Secrets == nil {
			req.Secrets = map[string]string{}
		}

		provider, err := deps.DNSReg.Get(dns.ProviderKind(req.Provider))
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, codeCredentialInvalid,
				"provider not registered: "+req.Provider, nil)
			return
		}
		if err := provider.ValidateCredential(req.Secrets); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, codeCredentialInvalid,
				err.Error(), nil)
			return
		}

		// Serialise + encrypt the secret map.
		raw, err := json.Marshal(req.Secrets)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"secrets marshal failed", nil)
			return
		}
		eb, err := deps.Vault.EncryptBlob(r.Context(), raw)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"secrets encrypt failed", nil)
			return
		}
		ebBytes, err := json.Marshal(eb)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"encrypted blob marshal failed", nil)
			return
		}

		// Best-effort live health check. A failure does NOT block
		// insertion — the credential is still useful manually, and the
		// row is marked invalid so the UI can prompt for rotation.
		health := "valid"
		if err := provider.HealthCheck(r.Context(), req.Secrets); err != nil {
			health = "invalid"
		}

		cred := &repo.DNSCredential{
			AccountID:     accountID,
			Provider:      req.Provider,
			DisplayName:   req.DisplayName,
			EncryptedBlob: ebBytes,
			DEKWrapped:    nil,
			KEKKeyID:      deps.Vault.KeyID(),
			HealthStatus:  health,
		}
		id, err := deps.Repos.DNSCredentials.Insert(r.Context(), cred)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"credential insert failed", nil)
			return
		}
		if health != "valid" {
			_ = deps.Repos.DNSCredentials.UpdateHealthStatus(r.Context(), id, health)
		}

		writeJSON(w, http.StatusCreated, dnsCredentialResponse{
			ID:           id,
			Provider:     req.Provider,
			DisplayName:  req.DisplayName,
			HealthStatus: health,
			CreatedAt:    cred.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
}

func listDNSCredentials(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "repos not configured", nil)
			return
		}
		rows, err := deps.Repos.DNSCredentials.ListByAccount(r.Context(), accountID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "list failed", nil)
			return
		}
		out := make([]dnsCredentialResponse, 0, len(rows))
		for _, c := range rows {
			out = append(out, projectDNSCredential(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_credentials": out})
	}
}

func deleteDNSCredential(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "repos not configured", nil)
			return
		}
		cred, err := deps.Repos.DNSCredentials.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "dns credential not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "lookup failed", nil)
			return
		}
		if cred.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"dns credential does not belong to this account", nil)
			return
		}
		if err := deps.Repos.DNSCredentials.Revoke(r.Context(), id); err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				// Already revoked — idempotent success.
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "revoke failed", nil)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func healthCheckDNSCredential(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Vault == nil || deps.DNSReg == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}
		cred, err := deps.Repos.DNSCredentials.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "dns credential not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "lookup failed", nil)
			return
		}
		if cred.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"dns credential does not belong to this account", nil)
			return
		}
		if cred.RevokedAt != nil {
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"credential already revoked", nil)
			return
		}
		secrets, err := decryptCredentialSecrets(r.Context(), deps.Vault, cred.EncryptedBlob)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"credential decrypt failed", nil)
			return
		}
		provider, err := deps.DNSReg.Get(dns.ProviderKind(cred.Provider))
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, codeCredentialInvalid,
				"provider not registered", nil)
			return
		}
		status := "valid"
		if err := provider.HealthCheck(r.Context(), secrets); err != nil {
			status = "invalid"
		}
		if uerr := deps.Repos.DNSCredentials.UpdateHealthStatus(r.Context(), id, status); uerr != nil {
			// Surface the write failure but include the probe outcome
			// the user actually cares about.
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"health status persist failed", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"health_status": status})
	}
}

// decryptCredentialSecrets unwraps the encrypted_blob column into the
// provider-shaped secrets map. The on-disk shape is
// json(vault.EncryptedBlob) — matching how createDNSCredential writes it
// — so symmetrical marshal/unmarshal works without a wrapper.
func decryptCredentialSecrets(ctx context.Context, v vault.Vault, blob []byte) (map[string]string, error) {
	var eb vault.EncryptedBlob
	if err := json.Unmarshal(blob, &eb); err != nil {
		return nil, err
	}
	plain, err := v.DecryptBlob(ctx, eb)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if len(plain) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func projectDNSCredential(c *repo.DNSCredential) dnsCredentialResponse {
	resp := dnsCredentialResponse{
		ID:           c.ID,
		Provider:     c.Provider,
		DisplayName:  c.DisplayName,
		HealthStatus: c.HealthStatus,
		CreatedAt:    c.CreatedAt.UTC().Format(time.RFC3339),
	}
	if c.HealthCheckedAt != nil {
		s := c.HealthCheckedAt.UTC().Format(time.RFC3339)
		resp.HealthCheckedAt = &s
	}
	if c.RevokedAt != nil {
		s := c.RevokedAt.UTC().Format(time.RFC3339)
		resp.RevokedAt = &s
	}
	return resp
}
