package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/idna"

	certmw "github.com/kite365/idcd/apps/cert-svc/internal/middleware"
)

// Centralised error codes — keep in sync with PRD §10.3.
const (
	codeUnauthorized        = "CERT_UNAUTHORIZED"
	codeForbidden           = "CERT_FORBIDDEN"
	codeNotFound            = "CERT_NOT_FOUND"
	codeInvalidStatus       = "CERT_INVALID_STATUS"
	codeQuotaExceeded       = "CERT_QUOTA_EXCEEDED"
	codeDomainInvalid       = "CERT_DOMAIN_INVALID"
	codeCredentialInvalid   = "CERT_DNS_CREDENTIAL_INVALID"
	codeFormatUnsupported   = "CERT_FORMAT_UNSUPPORTED"
	codeBadRequest          = "CERT_BAD_REQUEST"
	codeInternal            = "CERT_INTERNAL"
	codeNotImplemented      = "CERT_NOT_IMPL"
	codeDownloadTokenInvalid = "CERT_DOWNLOAD_TOKEN_INVALID"
	codeCAAForbid            = "CERT_CAA_FORBID"
	codeAbuseBlocked         = "CERT_ABUSE_BLOCKED"
)

// errResp is the canonical wire shape for non-2xx responses.
type errResp struct {
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// writeErr emits a structured error using the canonical envelope.
func writeErr(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, errResp{
		Error:   strings.ToLower(strings.TrimPrefix(code, "CERT_")),
		Code:    code,
		Message: message,
		Fields:  fields,
	})
}

// readJSON decodes the request body into v. Returns false (and writes a
// 400 response) when the payload is malformed.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Body == nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "request body required", nil)
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid JSON: "+err.Error(), nil)
		return false
	}
	return true
}

// requireUser pulls the authenticated user id out of context and parses
// it as the int64 account id used by the cert.* tables. Returns false
// (and writes 401) on any failure so handlers can early-return.
func requireUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid, err := certmw.UserIDFromContext(r.Context())
	if err != nil {
		writeErr(w, http.StatusUnauthorized, codeUnauthorized, "authentication required", nil)
		return 0, false
	}
	id, perr := strconv.ParseInt(uid, 10, 64)
	if perr != nil || id <= 0 {
		// cert.* tables key on int64 account ids; non-numeric user ids
		// mean the caller authed via a path we cannot map. Surface as
		// 401 rather than 500 to keep the error surface uniform.
		writeErr(w, http.StatusUnauthorized, codeUnauthorized, "invalid user id", nil)
		return 0, false
	}
	return id, true
}

// pathInt64 extracts a numeric path parameter, writing 404 on parse fail.
func pathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := chi.URLParam(r, name)
	if raw == "" {
		writeErr(w, http.StatusNotFound, codeNotFound, fmt.Sprintf("missing %s", name), nil)
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		writeErr(w, http.StatusNotFound, codeNotFound, fmt.Sprintf("invalid %s", name), nil)
		return 0, false
	}
	return v, true
}

// queryIntDefault reads an int query param, falling back to def if missing
// or unparseable.
func queryIntDefault(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

// reserved TLDs / labels we never issue certs for. See PRD §20 and
// https://www.iana.org/assignments/special-use-domain-names.
var reservedSANSuffixes = []string{
	".local",
	".internal",
	".test",
	".localhost",
	".example",
	".invalid",
	".onion",
}

var reservedExactSANs = map[string]struct{}{
	"localhost": {},
}

// normaliseSAN lowercases, strips a trailing dot, splits off any wildcard
// label, and IDN→Punycode the remainder. Returns the canonical ASCII
// form plus an error describing why a value was rejected.
func normaliseSAN(raw string) (string, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return "", errors.New("empty SAN")
	}
	s = strings.TrimSuffix(s, ".")
	if ip := net.ParseIP(s); ip != nil {
		return "", errors.New("IP addresses are not allowed")
	}
	if _, isReserved := reservedExactSANs[s]; isReserved {
		return "", errors.New("reserved name not allowed: " + s)
	}
	for _, suf := range reservedSANSuffixes {
		if strings.HasSuffix(s, suf) {
			return "", errors.New("reserved suffix not allowed: " + suf)
		}
	}
	wildcard := false
	body := s
	if strings.HasPrefix(s, "*.") {
		wildcard = true
		body = strings.TrimPrefix(s, "*.")
		if body == "" {
			return "", errors.New("wildcard requires a base domain")
		}
	}
	if strings.Contains(body, "*") {
		return "", errors.New("wildcard only allowed as leftmost label")
	}
	// idna.Lookup enforces the strict subset suitable for TLS SAN values.
	punycode, err := idna.Lookup.ToASCII(body)
	if err != nil {
		return "", fmt.Errorf("invalid domain %q: %v", raw, err)
	}
	if !strings.Contains(punycode, ".") {
		return "", errors.New("SAN must include at least one dot")
	}
	if wildcard {
		return "*." + punycode, nil
	}
	return punycode, nil
}

// dedupePreserveOrder collapses duplicates while keeping first-seen order.
func dedupePreserveOrder(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
