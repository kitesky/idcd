package handler

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/vault"
)

func mountCerts(r chi.Router, deps Deps) {
	r.Get("/certs", listCerts(deps))
	r.Get("/certs/{id}", getCert(deps))
	r.Post("/certs/{id}/download", downloadCert(deps))
	r.Post("/certs/{id}/revoke", revokeCert(deps))
}

type certResponse struct {
	ID                int64    `json:"id"`
	OrderID           int64    `json:"order_id"`
	AccountID         int64    `json:"account_id"`
	SANs              []string `json:"sans"`
	Issuer            string   `json:"issuer"`
	SerialHex         string   `json:"serial_hex"`
	FingerprintSHA256 string   `json:"fingerprint_sha256"`
	NotBefore         string   `json:"not_before"`
	NotAfter          string   `json:"not_after"`
	Status            string   `json:"status"`
	RevokedAt         *string  `json:"revoked_at,omitempty"`
	RevokeReason      *string  `json:"revoke_reason,omitempty"`
	CreatedAt         string   `json:"created_at"`
}

func listCerts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "repos not configured", nil)
			return
		}
		limit := queryIntDefault(r, "limit", 20)
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		offset := queryIntDefault(r, "offset", 0)
		if offset < 0 {
			offset = 0
		}
		var statusFilter *repo.CertStatus
		if s := r.URL.Query().Get("status"); s != "" {
			cs := repo.CertStatus(s)
			statusFilter = &cs
		}
		rows, err := deps.Repos.Certs.ListByAccount(r.Context(), accountID, statusFilter, limit, offset)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "certs list failed", nil)
			return
		}
		out := make([]certResponse, 0, len(rows))
		for _, c := range rows {
			out = append(out, projectCert(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"certs":  out,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func getCert(deps Deps) http.HandlerFunc {
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
		c, err := deps.Repos.Certs.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "cert not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "cert get failed", nil)
			return
		}
		if c.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"cert does not belong to this account", nil)
			return
		}
		writeJSON(w, http.StatusOK, projectCert(c))
	}
}

type downloadRequest struct {
	Format string `json:"format"`
}

func downloadCert(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Repos == nil || deps.Vault == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}
		var req downloadRequest
		// Body is optional — default to pem.
		if r.ContentLength > 0 {
			if !readJSON(w, r, &req) {
				return
			}
		}
		if req.Format == "" {
			req.Format = "pem"
		}
		switch req.Format {
		case "pem", "nginx":
			// supported
		case "pfx":
			writeErr(w, http.StatusNotImplemented, codeFormatUnsupported,
				"pfx download lands in W4", nil)
			return
		default:
			writeErr(w, http.StatusBadRequest, codeBadRequest,
				"unsupported format: "+req.Format, nil)
			return
		}

		c, err := deps.Repos.Certs.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "cert not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "cert get failed", nil)
			return
		}
		if c.AccountID != accountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"cert does not belong to this account", nil)
			return
		}
		if c.Status != repo.CertStatusIssued {
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"cert is not in issued status", nil)
			return
		}

		// Decode and decrypt the private key. The on-disk handle is
		// base64(json(vault.EncryptedKey)) — matching how the worker
		// writes it after issuance. We tolerate raw JSON too so older
		// rows (and tests) that skipped the base64 layer still decrypt.
		ek, err := decodeKeyHandle(c.KeyKMSHandle)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"cert key handle invalid: "+err.Error(), nil)
			return
		}
		plainPEM, err := deps.Vault.DecryptKey(r.Context(), ek)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"cert key decrypt failed", nil)
			return
		}

		fullchain := c.LeafPEM + c.ChainPEM

		var (
			body         []byte
			contentType  string
			disposition  string
		)
		switch req.Format {
		case "pem":
			body, err = buildZip(map[string][]byte{
				"fullchain.pem": []byte(fullchain),
				"privkey.pem":   plainPEM,
			})
			contentType = "application/zip"
			disposition = fmt.Sprintf(`attachment; filename="cert-%d.zip"`, c.ID)
		case "nginx":
			body, err = buildZip(map[string][]byte{
				"nginx.crt": []byte(fullchain),
				"nginx.key": plainPEM,
			})
			contentType = "application/zip"
			disposition = fmt.Sprintf(`attachment; filename="cert-%d-nginx.zip"`, c.ID)
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"cert bundle build failed", nil)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", disposition)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}

// decodeKeyHandle accepts the storage shape used by the cert-issuance
// worker: base64(json(vault.EncryptedKey)). The base64 hop keeps the
// TEXT column free of literal JSON braces (and stops clients from
// accidentally pretty-printing the row). Older / test rows may carry
// raw JSON — we fall back to JSON-on-the-wire if base64 fails.
func decodeKeyHandle(handle string) (vault.EncryptedKey, error) {
	if handle == "" {
		return vault.EncryptedKey{}, errors.New("empty handle")
	}
	if raw, err := base64.StdEncoding.DecodeString(handle); err == nil {
		var ek vault.EncryptedKey
		if jerr := json.Unmarshal(raw, &ek); jerr == nil {
			return ek, nil
		}
	}
	var ek vault.EncryptedKey
	if err := json.Unmarshal([]byte(handle), &ek); err != nil {
		return vault.EncryptedKey{}, fmt.Errorf("decode key handle: %w", err)
	}
	return ek, nil
}

// buildZip returns a single zip archive containing the named files.
// Files are added in iteration order — callers should expect a stable
// order across tests by passing in pre-sorted maps when it matters.
func buildZip(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func revokeCert(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ok := requireUser(w, r)
		if !ok {
			return
		}
		_, ok = pathInt64(w, r, "id")
		if !ok {
			return
		}
		// service.RevokeCert lands in W4 — until then we surface a
		// stable 501 so clients can branch on CERT_FORMAT_UNSUPPORTED
		// (reusing the code rather than minting a one-off).
		writeErr(w, http.StatusNotImplemented, codeNotImplemented,
			"cert revoke lands in W4", nil)
	}
}

func projectCert(c *repo.Cert) certResponse {
	resp := certResponse{
		ID:                c.ID,
		OrderID:           c.OrderID,
		AccountID:         c.AccountID,
		SANs:              c.SANs,
		Issuer:            c.Issuer,
		SerialHex:         c.SerialHex,
		FingerprintSHA256: c.FingerprintSHA256,
		NotBefore:         c.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:          c.NotAfter.UTC().Format(time.RFC3339),
		Status:            string(c.Status),
		RevokeReason:      c.RevokeReason,
		CreatedAt:         c.CreatedAt.UTC().Format(time.RFC3339),
	}
	if c.RevokedAt != nil {
		s := c.RevokedAt.UTC().Format(time.RFC3339)
		resp.RevokedAt = &s
	}
	return resp
}
