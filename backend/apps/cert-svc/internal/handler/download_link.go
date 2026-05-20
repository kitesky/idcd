package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// mountDownloadLink registers the W5 public download endpoint. The token
// itself is the only credential — no JWT, no session — so this route
// MUST be mounted outside the auth middleware (see handler.go::New).
func mountDownloadLink(r chi.Router, deps Deps) {
	r.Get("/v1/cert/dl/{token}", downloadByToken(deps))
}

// downloadByToken consumes a one-shot token and streams the matching
// cert in the format the token carries.
//
// Failure mode: any token-side failure (bad shape, bad HMAC, already
// consumed, expired) collapses to 410 GONE — the resource the URL once
// pointed at is gone and will not return. Cert-side failures (cert
// missing / ownership drift) keep their canonical codes so admins can
// diagnose mis-issued tokens.
func downloadByToken(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Service == nil || deps.Service.Downloads == nil {
			writeErr(w, http.StatusServiceUnavailable, codeInternal,
				"download token manager not configured", nil)
			return
		}
		if deps.Repos == nil || deps.Vault == nil {
			writeErr(w, http.StatusInternalServerError, codeInternal, "deps not configured", nil)
			return
		}

		token := chi.URLParam(r, "token")
		payload, err := deps.Service.Downloads.Consume(r.Context(), token)
		if err != nil {
			writeErr(w, http.StatusGone, codeDownloadTokenInvalid,
				"token invalid or already used", nil)
			return
		}

		c, err := deps.Repos.Certs.GetByID(r.Context(), payload.CertID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				writeErr(w, http.StatusNotFound, codeNotFound, "cert not found", nil)
				return
			}
			writeErr(w, http.StatusInternalServerError, codeInternal, "cert get failed", nil)
			return
		}
		// Defence in depth: the cert's owner should still match the
		// account the token was minted for. Drift here means either a
		// transfer between Issue and Consume (impossible in S1) or a
		// forged token that somehow cleared HMAC — refuse either way.
		if c.AccountID != payload.AccountID {
			writeErr(w, http.StatusForbidden, codeForbidden,
				"cert does not belong to this account", nil)
			return
		}
		if c.Status != repo.CertStatusIssued {
			writeErr(w, http.StatusConflict, codeInvalidStatus,
				"cert is not in issued status", nil)
			return
		}

		// PFX requires a password baked into the token. PEM/nginx do
		// not — fall through and ignore any stray value.
		if payload.Format == "pfx" && payload.Password == "" {
			writeErr(w, http.StatusUnprocessableEntity, codeBadRequest,
				"pfx export requires a non-empty password", nil)
			return
		}

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
			body        []byte
			contentType string
			disposition string
			buildErr    error
		)
		switch payload.Format {
		case "pem":
			body, buildErr = buildZip(map[string][]byte{
				"fullchain.pem": []byte(fullchain),
				"privkey.pem":   plainPEM,
			})
			contentType = "application/zip"
			disposition = fmt.Sprintf(`attachment; filename="cert-%d.zip"`, c.ID)
		case "nginx":
			body, buildErr = buildZip(map[string][]byte{
				"nginx.crt": []byte(fullchain),
				"nginx.key": plainPEM,
			})
			contentType = "application/zip"
			disposition = fmt.Sprintf(`attachment; filename="cert-%d-nginx.zip"`, c.ID)
		case "pfx":
			body, buildErr = buildPFX(c.LeafPEM, c.ChainPEM, plainPEM, payload.Password)
			contentType = "application/x-pkcs12"
			disposition = fmt.Sprintf(`attachment; filename="cert-%d.pfx"`, c.ID)
		default:
			// Format made it through Issue's validation — anything else
			// means the token came from a future Issue path we don't yet
			// understand. Refuse rather than guess.
			writeErr(w, http.StatusBadRequest, codeBadRequest,
				"unsupported format: "+payload.Format, nil)
			return
		}
		if buildErr != nil {
			writeErr(w, http.StatusInternalServerError, codeInternal,
				"cert bundle build failed", nil)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", disposition)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}
