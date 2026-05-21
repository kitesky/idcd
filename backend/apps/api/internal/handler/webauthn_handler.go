package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/api/internal/middleware"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/auth/webauthn"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/idgen"
)

const (
	webauthnChallengeTTL    = 5 * time.Minute
	webauthnDefaultRPID     = "idcd.com"
	webauthnDefaultRPName   = "idcd"
)

type WebAuthnPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type WebAuthnHandler struct {
	pool    WebAuthnPool
	redis   redis.UniversalClient
	rpID    string
	origins []string
	jwtSvc  JWTSigner
	sessSvc SessionStorer
}

func NewWebAuthnHandler(pool *pgxpool.Pool, rdb redis.UniversalClient, rpID string) *WebAuthnHandler {
	if rpID == "" {
		rpID = webauthnDefaultRPID
	}
	return &WebAuthnHandler{pool: pool, redis: rdb, rpID: rpID}
}

func (h *WebAuthnHandler) WithAuth(jwtSvc JWTSigner, sessSvc SessionStorer) *WebAuthnHandler {
	h.jwtSvc = jwtSvc
	h.sessSvc = sessSvc
	return h
}

// WithOrigins sets the explicit allow-list of acceptable Web Origins for
// WebAuthn ceremonies. When unset, the verifier falls back to
// "https://" + rpID — fine for prod but breaks local dev (which uses
// http://localhost:3000). Callers that need multi-origin support (e.g.
// dev + staging + prod, or apex + www) should pass the full list here.
func (h *WebAuthnHandler) WithOrigins(origins []string) *WebAuthnHandler {
	h.origins = origins
	return h
}

// verifier builds a fresh Verifier from the handler's rpID + origins.
// If origins is empty we fall back to ["https://" + rpID] so production
// deployments work without extra wiring; dev/staging callers MUST use
// WithOrigins to allow http://localhost or alternate hosts.
func (h *WebAuthnHandler) verifier() *webauthn.Verifier {
	origins := h.origins
	if len(origins) == 0 {
		origins = []string{"https://" + h.rpID}
	}
	return webauthn.NewVerifier(h.rpID, origins)
}

type registerBeginResponse struct {
	ChallengeID string                            `json:"challenge_id"`
	Options     webauthn.CredentialCreationOptions `json:"options"`
}

type registerCompleteRequest struct {
	Challenge  string         `json:"challenge"`
	Response   map[string]any `json:"response"`
	DeviceName string         `json:"device_name"`
}

type registerCompleteResponse struct {
	CredentialID string `json:"credential_id"`
	DeviceName   string `json:"device_name"`
}

type passkeyListItem struct {
	ID          string     `json:"id"`
	DeviceName  string     `json:"device_name"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}

type authBeginRequest struct {
	UserIDOrEmail string `json:"user_id_or_email"`
}

type authBeginResponse struct {
	ChallengeID string                           `json:"challenge_id"`
	Options     webauthn.CredentialRequestOptions `json:"options"`
}

type authCompleteRequest struct {
	Challenge string         `json:"challenge"`
	Response  map[string]any `json:"response"`
}

func (h *WebAuthnHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var email string
	err := h.pool.QueryRow(r.Context(),
		`SELECT email FROM "user" WHERE id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&email)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to fetch user", err))
		return
	}

	challenge, err := webauthn.GenerateChallenge()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate challenge", err))
		return
	}

	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO webauthn_challenges (challenge, user_id, purpose, expires_at)
		VALUES ($1, $2, 'registration', $3)
	`, challenge, userID, time.Now().Add(webauthnChallengeTTL))
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to store challenge", err))
		return
	}

	opts := webauthn.NewCredentialCreationOptions(challenge, h.rpID, webauthnDefaultRPName, userID, email)
	response.JSON(w, r, http.StatusOK, registerBeginResponse{
		ChallengeID: challenge,
		Options:     opts,
	})
}

func (h *WebAuthnHandler) RegisterComplete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	var req registerCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Challenge == "" {
		response.Error(w, r, apperr.Validation("challenge is required", ""))
		return
	}
	if req.Response == nil {
		response.Error(w, r, apperr.Validation("response is required", ""))
		return
	}

	// Consume challenge atomically — prevents concurrent replay of the same challenge.
	var storedUserID string
	var expiresAt time.Time
	err := h.pool.QueryRow(r.Context(), `
		DELETE FROM webauthn_challenges
		WHERE challenge = $1 AND purpose = 'registration'
		RETURNING user_id, expires_at
	`, req.Challenge).Scan(&storedUserID, &expiresAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("invalid or unknown challenge", ""))
		return
	}
	if storedUserID != userID {
		response.Error(w, r, apperr.Validation("challenge user mismatch", ""))
		return
	}
	if time.Now().After(expiresAt) {
		response.Error(w, r, apperr.Validation("challenge expired", ""))
		return
	}

	// Real verification: challenge equality, origin, RPID, attestation
	// signature, COSE algorithm whitelist. Replaces the legacy stub that
	// only parsed the CBOR and trusted whatever the client sent.
	credentialID, publicKey, err := h.verifier().VerifyAttestation(req.Response, req.Challenge)
	if err != nil {
		response.Error(w, r, apperr.Validation("invalid attestation response: "+err.Error(), ""))
		return
	}

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "My Passkey"
	}

	id := idgen.New("wc_")
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO webauthn_credentials
		  (id, user_id, credential_id, public_key, sign_count, device_name)
		VALUES ($1, $2, $3, $4, 0, $5)
	`, id, userID, credentialID, publicKey, deviceName)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to store credential", err))
		return
	}

	response.JSON(w, r, http.StatusOK, registerCompleteResponse{
		CredentialID: credentialID,
		DeviceName:   deviceName,
	})
}

func (h *WebAuthnHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, device_name, created_at, last_used_at
		FROM webauthn_credentials
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to list passkeys", err))
		return
	}
	defer rows.Close()

	items := make([]passkeyListItem, 0)
	for rows.Next() {
		var item passkeyListItem
		var lastUsed *time.Time
		if err := rows.Scan(&item.ID, &item.DeviceName, &item.CreatedAt, &lastUsed); err != nil {
			response.Error(w, r, apperr.Internal("failed to scan passkey", err))
			return
		}
		item.LastUsedAt = lastUsed
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		response.Error(w, r, apperr.Internal("failed to iterate passkeys", err))
		return
	}

	response.JSON(w, r, http.StatusOK, items)
}

func (h *WebAuthnHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		response.Error(w, r, apperr.Unauthorized("not authenticated"))
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		response.Error(w, r, apperr.Validation("id is required", ""))
		return
	}

	tag, err := h.pool.Exec(r.Context(), `
		DELETE FROM webauthn_credentials WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to delete passkey", err))
		return
	}
	if tag.RowsAffected() == 0 {
		response.Error(w, r, apperr.NotFound("passkey not found"))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *WebAuthnHandler) AuthBegin(w http.ResponseWriter, r *http.Request) {
	var req authBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req = authBeginRequest{}
	}

	challenge, err := webauthn.GenerateChallenge()
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to generate challenge", err))
		return
	}

	var userID *string
	var credentialIDs []string

	if req.UserIDOrEmail != "" {
		var uid string
		err := h.pool.QueryRow(r.Context(),
			`SELECT id FROM "user" WHERE (id = $1 OR email = $1) AND deleted_at IS NULL`,
			req.UserIDOrEmail,
		).Scan(&uid)
		if err == nil {
			userID = &uid

			rows, err := h.pool.Query(r.Context(),
				`SELECT credential_id FROM webauthn_credentials WHERE user_id = $1`, uid)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var credID string
					if err := rows.Scan(&credID); err == nil {
						credentialIDs = append(credentialIDs, credID)
					}
				}
			}
		}
	}

	var uidStr string
	if userID != nil {
		uidStr = *userID
	}
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO webauthn_challenges (challenge, user_id, purpose, expires_at)
		VALUES ($1, NULLIF($2, ''), 'authentication', $3)
	`, challenge, uidStr, time.Now().Add(webauthnChallengeTTL))
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to store challenge", err))
		return
	}

	opts := webauthn.NewCredentialRequestOptions(challenge, h.rpID, credentialIDs)
	response.JSON(w, r, http.StatusOK, authBeginResponse{
		ChallengeID: challenge,
		Options:     opts,
	})
}

func (h *WebAuthnHandler) AuthComplete(w http.ResponseWriter, r *http.Request) {
	var req authCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, apperr.Validation("invalid request body", err.Error()))
		return
	}
	if req.Challenge == "" {
		response.Error(w, r, apperr.Validation("challenge is required", ""))
		return
	}
	if req.Response == nil {
		response.Error(w, r, apperr.Validation("response is required", ""))
		return
	}

	// Consume challenge atomically — prevents concurrent replay of the same challenge.
	var expiresAt time.Time
	err := h.pool.QueryRow(r.Context(), `
		DELETE FROM webauthn_challenges
		WHERE challenge = $1 AND purpose = 'authentication'
		RETURNING expires_at
	`, req.Challenge).Scan(&expiresAt)
	if err != nil {
		response.Error(w, r, apperr.Validation("invalid or unknown challenge", ""))
		return
	}
	if time.Now().After(expiresAt) {
		response.Error(w, r, apperr.Validation("challenge expired", ""))
		return
	}

	// First parse — *no* signature check yet — just to extract the
	// credential ID so we can look up the matching public key in the DB.
	// The real cryptographic verification happens via VerifyAssertion below.
	credentialID, _, err := webauthn.ParseAssertionResponse(req.Response)
	if err != nil {
		response.Error(w, r, apperr.Validation("invalid assertion response: "+err.Error(), ""))
		return
	}

	var storedUserID, storedPubKey string
	var storedSignCount int64
	err = h.pool.QueryRow(r.Context(), `
		SELECT user_id, public_key, sign_count FROM webauthn_credentials WHERE credential_id = $1
	`, credentialID).Scan(&storedUserID, &storedPubKey, &storedSignCount)
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("unknown credential"))
		return
	}

	// Real WebAuthn assertion verification: challenge equality, origin,
	// RPID, signature against storedPubKey, and replay protection
	// (newSignCount > storedSignCount when non-zero).
	_, newSignCount, err := h.verifier().VerifyAssertion(req.Response, storedPubKey, req.Challenge, storedSignCount)
	if err != nil {
		response.Error(w, r, apperr.Unauthorized("assertion verification failed: "+err.Error()))
		return
	}

	_, _ = h.pool.Exec(r.Context(), `
		UPDATE webauthn_credentials SET sign_count = $1, last_used_at = now()
		WHERE credential_id = $2
	`, newSignCount, credentialID)

	if h.jwtSvc == nil || h.sessSvc == nil {
		response.Error(w, r, apperr.Internal("auth service not configured", nil))
		return
	}

	sessionID := idgen.New("sess")
	if err := h.sessSvc.Store(r.Context(), sessionID, storedUserID, sessionTTL); err != nil {
		response.Error(w, r, apperr.Internal("failed to create session", err))
		return
	}
	// Pull the user's stored locale so the JWT carries the right claim
	// (the i18n middleware's resolution chain trusts this). Empty locale
	// downgrades gracefully — middleware will fall through to Accept-Language.
	var userLocale string
	_ = h.pool.QueryRow(r.Context(),
		`SELECT locale FROM "user" WHERE id = $1`, storedUserID,
	).Scan(&userLocale)
	token, err := h.jwtSvc.SignWithLocale(storedUserID, sessionID, userLocale, accessTokenTTL)
	if err != nil {
		response.Error(w, r, apperr.Internal("failed to sign token", err))
		return
	}

	response.JSON(w, r, http.StatusOK, authResponse{
		AccessToken: token,
		ExpiresIn:   int(accessTokenTTL.Seconds()),
		UserID:      storedUserID,
	})
}
