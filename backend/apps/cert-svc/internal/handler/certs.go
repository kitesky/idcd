package handler

import (
	"archive/zip"
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/apps/cert-svc/internal/service"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/vault"
	"github.com/kite365/idcd/lib/shared/pagination"
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
	AccountID  string    `json:"account_id"`
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
		limit := pagination.Clamp(queryIntDefault(r, "limit", pagination.DefaultPageSize))
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
	Format   string `json:"format"`
	Password string `json:"password,omitempty"`
}

// downloadResponse is the wire shape for POST /v1/cert/certs/{id}/download
// (W5+). The actual cert bytes never travel back through this response —
// the caller follows download_url, which is a single-use signed token
// resolved by GET /v1/cert/dl/{token}. See docs/prd/20-free-cert.md §10.1.
type downloadResponse struct {
	DownloadURL string `json:"download_url"`
	ExpiresAt   string `json:"expires_at"`
}

// downloadCert issues a one-shot URL pointing at /v1/cert/dl/{token}.
//
// W5 swap: the previous in-line PEM/PFX/zip response leaked private keys
// through the authenticated API surface (proxies, browser history,
// service logs). We now mint a 5-minute HMAC-signed token, persist a
// single-use Redis marker, and return {download_url, expires_at}. The
// actual content-bearing handler lives in download_link.go.
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
		if deps.Repos == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}
		if deps.Service == nil || deps.Service.Downloads == nil {
			// Surface a hard 503 so operators see the misconfig rather
			// than letting downloads silently 500 on Issue.
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"download token manager not configured", nil)
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
			// PFX requires a non-empty password — empty-password PFX
			// files are not portable and most consumers (Windows, Java
			// keytool) flat-out reject them. We surface the constraint
			// up-front before issuing a token we'd refuse on redeem.
			if req.Password == "" {
				writeErr(w, http.StatusUnprocessableEntity, codeBadRequest,
					"pfx export requires a non-empty password", nil)
				return
			}
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

		token, expiresAt, err := deps.Service.Downloads.Issue(r.Context(), service.DownloadTokenPayload{
			CertID:    c.ID,
			AccountID: c.AccountID,
			Format:    req.Format,
			Password:  req.Password,
		})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"download token issue failed", nil)
			return
		}
		writeJSON(w, http.StatusOK, downloadResponse{
			DownloadURL: "/v1/cert/dl/" + token,
			ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		})
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

// revokeRequest is the wire body for POST /v1/cert/certs/{id}/revoke.
// Reason is optional and defaults to "unspecified" — the RFC 5280 catch-all
// the ACME server accepts when the caller does not have a stronger signal.
type revokeRequest struct {
	Reason string `json:"reason,omitempty"`
}

// parseRevokeReason maps the wire string onto a ca.RevokeReason. Unknown
// values fall back to RevokeUnspecified rather than rejecting the request
// outright — the user already wants the cert dead; we should not block on
// a typo.
func parseRevokeReason(s string) ca.RevokeReason {
	switch s {
	case "keyCompromise", "key_compromise":
		return ca.RevokeKeyCompromise
	case "cessationOfOperation", "cessation_of_operation":
		return ca.RevokeCessationOfOperation
	case "certificateHold", "certificate_hold":
		return ca.RevokeCertificateHold
	default:
		return ca.RevokeUnspecified
	}
}

func revokeCert(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accountID, ok := requireUser(w, r)
		if !ok {
			return
		}
		id, ok := pathInt64(w, r, "id")
		if !ok {
			return
		}
		if deps.Service == nil {
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"cert-svc service not configured", nil)
			return
		}

		var req revokeRequest
		// Body is optional — POST without payload defaults to
		// reason=unspecified, matching the OpenAPI spec.
		if r.ContentLength > 0 {
			if !readJSON(w, r, &req) {
				return
			}
		}
		reason := parseRevokeReason(req.Reason)

		err := deps.Service.RevokeCert(r.Context(), accountID, id, reason)
		switch {
		case err == nil:
			writeJSON(w, http.StatusAccepted, map[string]any{
				"cert_id": id,
				"status":  "revoking",
			})
			return
		case errors.Is(err, service.ErrNotFound):
			writeErr(w, http.StatusNotFound, codeNotFound, "cert not found", nil)
		case errors.Is(err, service.ErrForbidden):
			writeErr(w, http.StatusForbidden, codeForbidden,
				"cert does not belong to this account", nil)
		case errors.Is(err, service.ErrInvalidStatus):
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"cert is not in a revocable state", nil)
		case errors.Is(err, service.ErrNotConfigured):
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"cert-svc not configured for revoke", nil)
		case errors.Is(err, ca.ErrCAQuotaExceeded), errors.Is(err, ca.ErrNetwork):
			writeErr(w, http.StatusBadGateway, codeInternal,
				"upstream CA unavailable: "+err.Error(), nil)
		default:
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"revoke failed: "+err.Error(), nil)
		}
	}
}

// buildPFX assembles a PKCS#12 archive containing the leaf, the chain and
// the private key, encrypted under password. The chain PEM may carry zero
// or more intermediate certificates; we decode them all and pass to
// pkcs12.Modern.Encode which chains them as CA certificates.
//
// "Modern" encoding uses AES-256 + PBKDF2-SHA256 — accepted by current
// Windows, macOS Keychain, Java keytool and openssl >= 3.0. Older
// consumers may need pkcs12.LegacyRC2.Encode; we deliberately default to
// the modern path because S2 mobile / IoT targets all support it.
func buildPFX(leafPEM, chainPEM string, keyPEM []byte, password string) ([]byte, error) {
	leaf, err := parseFirstCert([]byte(leafPEM))
	if err != nil {
		return nil, fmt.Errorf("parse leaf: %w", err)
	}
	chain, err := parseAllCerts([]byte(chainPEM))
	if err != nil {
		return nil, fmt.Errorf("parse chain: %w", err)
	}
	priv, err := parsePEMPrivateKey(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return pkcs12.Modern.Encode(priv, leaf, chain, password)
}

// parseFirstCert decodes the first CERTIFICATE PEM block. Returns an
// error if the input has no parseable block.
func parseFirstCert(p []byte) (*x509.Certificate, error) {
	for {
		block, rest := pem.Decode(p)
		if block == nil {
			return nil, errors.New("no PEM block found")
		}
		if block.Type == "CERTIFICATE" {
			return x509.ParseCertificate(block.Bytes)
		}
		p = rest
	}
}

// parseAllCerts decodes every CERTIFICATE block in p. Returns an empty
// slice (no error) when the input is empty or contains only non-cert
// blocks — a leaf-only PEM is a valid PFX input.
func parseAllCerts(p []byte) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	for {
		block, rest := pem.Decode(p)
		if block == nil {
			return out, nil
		}
		if block.Type == "CERTIFICATE" {
			c, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		p = rest
	}
}

// parsePEMPrivateKey decodes a PEM-encoded private key. Accepts PKCS#8
// (the format Vault.GenerateKey emits), PKCS#1 RSA, and SEC1 EC keys —
// the union covers every wire shape S1 can produce.
func parsePEMPrivateKey(p []byte) (any, error) {
	block, _ := pem.Decode(p)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("unsupported private key format")
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
