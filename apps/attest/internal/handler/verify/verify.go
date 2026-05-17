package verify

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/digitorus/pkcs7"

	"github.com/kite365/idcd/lib/attest/sign"
)

// Disclaimer is the legal boundary statement attached to every
// VerifyResult. PRD §3.6: the verifier output must explicitly remind
// downstream readers that idcd produces observational records, not
// forensic judicial appraisals.
const Disclaimer = "This document records observations from idcd's monitoring network. It is not a forensic judicial appraisal."

// DefaultMaxPDFBytes caps the PDF size accepted by /verify. 32 MiB is
// well above the largest verdict report we generate (~2-4 MiB) and
// avoids a trivial memory DoS on the public endpoint.
const DefaultMaxPDFBytes = 32 << 20

// ReportLookup resolves a report_id to the signature metadata idcd
// recorded when the verdict was issued. The /verify/{id} route uses
// this so callers can re-confirm a report without re-uploading the
// PDF. Implementations live in the repo layer.
type ReportLookup interface {
	LookupByID(ctx context.Context, reportID string) (*KnownReport, error)
}

// KnownReport is the projection ReportLookup returns. All fields are
// optional except ID and ContentHash. SignedAt / SignatureKeyID /
// TSAProvider should reflect the verdict_report row at issuance time.
type KnownReport struct {
	ID             string
	ContentHash    string // hex sha256 of the signed payload
	Signature      []byte
	SignatureKeyID string
	TSAProvider    string
	TSATime        time.Time
	ReportType     string // e.g. "observation_only"
}

// ErrReportNotFound is the sentinel ReportLookup implementations
// return when no record matches the requested report_id. The handler
// translates it to HTTP 404.
var ErrReportNotFound = errors.New("verify: report not found")

// VerifyResult is the JSON body returned by both /verify routes.
// Field names follow the public API spec (snake_case).
//
// A "valid=false" result is still HTTP 200 — the verification ran but
// produced a negative outcome. HTTP 4xx / 5xx are reserved for
// request-level errors (no file, file too large, internal panic).
type VerifyResult struct {
	Valid          bool      `json:"valid"`
	ReportID       string    `json:"report_id,omitempty"`
	SignerKeyID    string    `json:"signer_key_id,omitempty"`
	SignedAt       time.Time `json:"signed_at,omitempty"`
	TSAProvider    string    `json:"tsa_provider,omitempty"`
	ContentSHA256  string    `json:"content_sha256,omitempty"`
	ReportType     string    `json:"report_type,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	DisclaimerNote string    `json:"disclaimer,omitempty"`
}

// Handler implements the public /verify HTTP routes.
//
// Verifier is the KMS-backed sign.Verifier whose PublicKey() will be
// used to validate the embedded CMS signature. ReportLookup may be
// nil, in which case GET /verify/{id} returns 404 for every request.
// Logger may be nil (falls back to slog.Default()).
type Handler struct {
	Verifier     sign.Verifier
	ReportLookup ReportLookup
	Logger       *slog.Logger
	MaxPDFBytes  int64
}

// ServeHTTP routes:
//
//	POST /verify              - multipart upload "file" = PDF bytes
//	GET  /verify/{report_id}  - looks up known signature via ReportLookup
//
// Anything else returns 405. The router intentionally does no path
// normalisation beyond a TrimPrefix — mount the handler at "/verify".
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Verifier == nil {
		http.Error(w, "verifier not configured", http.StatusInternalServerError)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/verify")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodPost && path == "":
		h.handlePOST(w, r)
	case r.Method == http.MethodGet && path != "":
		h.handleGET(w, r, path)
	default:
		w.Header().Set("Allow", "POST, GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func (h *Handler) maxBytes() int64 {
	if h.MaxPDFBytes > 0 {
		return h.MaxPDFBytes
	}
	return DefaultMaxPDFBytes
}

// handlePOST consumes a multipart upload whose "file" field is a PDF.
// It runs the full verify pipeline and writes a VerifyResult JSON.
func (h *Handler) handlePOST(w http.ResponseWriter, r *http.Request) {
	// Cap the request body up-front. http.MaxBytesReader makes
	// subsequent Read() return an error past the limit, which the
	// multipart parser surfaces as a generic error — we translate
	// that to 413 below.
	max := h.maxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, max)

	if err := r.ParseMultipartForm(max); err != nil {
		if isTooLarge(err) {
			http.Error(w, "pdf exceeds maximum size", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `missing "file" form field`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header != nil && header.Size == 0 {
		http.Error(w, "uploaded file is empty", http.StatusBadRequest)
		return
	}

	pdfBytes, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		http.Error(w, "failed to read uploaded file: "+err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(pdfBytes)) > max {
		http.Error(w, "pdf exceeds maximum size", http.StatusRequestEntityTooLarge)
		return
	}
	if len(pdfBytes) == 0 {
		http.Error(w, "uploaded file is empty", http.StatusBadRequest)
		return
	}

	result := h.verifyPDF(r.Context(), pdfBytes)
	writeJSON(w, http.StatusOK, result)
}

// handleGET answers /verify/{report_id} by looking up the stored
// signature metadata. We do NOT re-verify the PDF here — the caller
// trusts that the original verdict generator already produced a valid
// signature; this endpoint only republishes the recorded metadata so
// users can quote a stable URL. The Self-Verify worker uses POST.
func (h *Handler) handleGET(w http.ResponseWriter, r *http.Request, reportID string) {
	if h.ReportLookup == nil {
		http.Error(w, "report lookup not configured", http.StatusNotFound)
		return
	}
	rec, err := h.ReportLookup.LookupByID(r.Context(), reportID)
	if err != nil {
		if errors.Is(err, ErrReportNotFound) {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}
		h.logger().Error("verify: lookup failed", "report_id", reportID, "err", err)
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if rec == nil {
		http.Error(w, "report not found", http.StatusNotFound)
		return
	}

	res := VerifyResult{
		Valid:          true,
		ReportID:       rec.ID,
		SignerKeyID:    rec.SignatureKeyID,
		SignedAt:       rec.TSATime,
		TSAProvider:    rec.TSAProvider,
		ContentSHA256:  rec.ContentHash,
		ReportType:     rec.ReportType,
		DisclaimerNote: Disclaimer,
	}
	writeJSON(w, http.StatusOK, res)
}

// verifyPDF is the core algorithm shared by POST /verify. It:
//
//  1. extracts /ByteRange + /Contents,
//  2. parses the CMS SignedData and verifies it (digest + signature)
//     using the certificate embedded in the SignedData itself,
//  3. cross-checks that the embedded signer cert's public key matches
//     the KMS-held public key returned by sign.Verifier.PublicKey(),
//  4. extracts the RFC3161 TimeStampToken (if any) for SignedAt.
//
// Step 3 is what binds the verification to *idcd's* signing key. If
// the cert in the PDF were swapped for an attacker-controlled cert,
// pkcs7.Verify() would still pass, but the public-key match would
// fail.
func (h *Handler) verifyPDF(ctx context.Context, pdfBytes []byte) VerifyResult {
	res := VerifyResult{
		Valid:          false,
		SignerKeyID:    h.Verifier.KeyID(),
		DisclaimerNote: Disclaimer,
	}

	// Content hash is reported regardless of verify outcome — clients
	// can use it to compare against an out-of-band record.
	contentSum := sha256.Sum256(pdfBytes)
	res.ContentSHA256 = hex.EncodeToString(contentSum[:])

	ext, err := extract(pdfBytes)
	if err != nil {
		res.Reason = err.Error()
		return res
	}

	p7, err := pkcs7.Parse(ext.CMS)
	if err != nil {
		res.Reason = "parse cms: " + err.Error()
		return res
	}
	p7.Content = ext.SignedBytes

	// pkcs7.Verify() validates the message digest, the signed
	// attributes, and the signature against the cert embedded in the
	// SignedData. We deliberately skip chain validation (we don't
	// distribute the idcd root in system trust stores) and instead
	// re-bind trust to the KMS public key below.
	if err := p7.Verify(); err != nil {
		res.Reason = "signature verification failed: " + err.Error()
		return res
	}

	// Bind to KMS-held public key. The embedded signer cert must use
	// the same key we'd get from KMS GetPublicKey(); otherwise the
	// signature is technically valid CMS but not idcd-authentic.
	signerCert := p7.GetOnlySigner()
	if signerCert == nil && len(p7.Certificates) > 0 {
		signerCert = p7.Certificates[0]
	}
	if signerCert == nil {
		res.Reason = "no signer certificate in cms"
		return res
	}

	pemBytes, err := h.Verifier.PublicKey(ctx)
	if err != nil {
		// KMS lookup failure is logged but exposed only as a generic
		// "kms unavailable" reason so we don't leak provider error
		// strings on the public endpoint.
		h.logger().Error("verify: kms public key fetch failed", "err", err)
		res.Reason = "kms unavailable"
		return res
	}
	matches, err := publicKeyMatches(pemBytes, signerCert.RawSubjectPublicKeyInfo)
	if err != nil {
		res.Reason = "kms key parse failed: " + err.Error()
		return res
	}
	if !matches {
		res.Reason = "signer key does not match idcd kms key"
		return res
	}

	// Extract RFC3161 TimeStampToken (optional). Failure to parse a
	// token is treated as "no timestamp" rather than a verify failure
	// — the CMS signature itself is already validated.
	if ts, err := extractTSAToken(ext.CMS); err == nil && ts != nil {
		res.SignedAt = ts.Time
		// We don't have a structured TSA provider field from the
		// token alone; the caller can resolve it from cert subject.
		if len(ts.Certificates) > 0 {
			res.TSAProvider = ts.Certificates[0].Subject.CommonName
		}
	}

	res.Valid = true
	return res
}

// publicKeyMatches reports whether pemBytes (PEM-encoded PKIX
// SubjectPublicKeyInfo, as returned by sign.Verifier) is byte-equal
// to the SubjectPublicKeyInfo carried by an embedded x509
// certificate. We compare DER bytes rather than parsed key structs so
// the check works for ECDSA, RSA, and any future algorithm uniformly.
func publicKeyMatches(pemBytes []byte, certPKIX []byte) (bool, error) {
	if len(pemBytes) == 0 {
		return false, fmt.Errorf("empty pem")
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return false, fmt.Errorf("pem decode failed")
	}
	return len(block.Bytes) > 0 && len(certPKIX) > 0 && bytesEqual(block.Bytes, certPKIX), nil
}

// bytesEqual avoids pulling in bytes just for one Equal call.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeJSON serialises v as JSON to w with the standard headers.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// isTooLarge detects MaxBytesReader's "http: request body too large"
// indirectly — net/http does not expose a sentinel for it.
func isTooLarge(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "request body too large") ||
		strings.Contains(err.Error(), "http: request body too large")
}
