package verify

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/digitorus/timestamp"

	"github.com/kite365/idcd/lib/attest/pdfsign"
	"github.com/kite365/idcd/lib/attest/sign"
)

// minimalPDF is borrowed verbatim from lib/attest/pdfsign — the
// hand-crafted xref offsets are exactly correct for this byte
// sequence.
const minimalPDF = "%PDF-1.4\n" +
	"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
	"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
	"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> >>\nendobj\n" +
	"xref\n" +
	"0 4\n" +
	"0000000000 65535 f \n" +
	"0000000009 00000 n \n" +
	"0000000058 00000 n \n" +
	"0000000115 00000 n \n" +
	"trailer\n<< /Size 4 /Root 1 0 R >>\n" +
	"startxref\n203\n" +
	"%%EOF\n"

// fakeVerifier implements sign.Verifier with an in-memory RSA public
// key. The PEM matches the cert returned by generateRSACert below.
type fakeVerifier struct {
	keyID string
	pem   []byte
	err   error
}

func (f *fakeVerifier) KeyID() string                               { return f.keyID }
func (f *fakeVerifier) Algorithm() string                           { return sign.AlgorithmRSAPKCS1SHA256 }
func (f *fakeVerifier) PublicKey(_ context.Context) ([]byte, error) { return f.pem, f.err }

func generateRSACert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idcd-test-signer"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return cert, key, pubPEM
}

// newTSAServer mirrors the test helper in lib/attest/pdfsign so we
// can issue a real RFC3161 token without a network dependency.
func newTSAServer(t *testing.T, tsaKey *rsa.PrivateKey, tsaCert *x509.Certificate) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req, err := timestamp.ParseRequest(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ts := timestamp.Timestamp{
			HashAlgorithm: req.HashAlgorithm,
			HashedMessage: req.HashedMessage,
			Time:          time.Now(),
			Nonce:         req.Nonce,
			Policy:        asn1.ObjectIdentifier{1, 2, 3, 4},
			SerialNumber:  big.NewInt(1),
		}
		token, err := ts.CreateResponseWithOpts(tsaCert, tsaKey, crypto.SHA256)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(token)
	}))
}

// signMinimalPDF produces a real signed PDF backed by a local RSA key
// for use as fixture input to the handler.
func signMinimalPDF(t *testing.T, withTSA bool) (signedPDF []byte, pubPEM []byte) {
	t.Helper()
	cert, key, pem := generateRSACert(t)

	signFn := func(_ context.Context, digest []byte) ([]byte, error) {
		return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	}
	req := pdfsign.SignRequest{
		Input:             []byte(minimalPDF),
		KMSSign:           signFn,
		SignerCertificate: cert,
		Name:              "idcd-test",
		Reason:            "unit test",
		Location:          "ci",
	}
	if withTSA {
		srv := newTSAServer(t, key, cert)
		t.Cleanup(srv.Close)
		req.TSAEndpoint = srv.URL
	}
	out, err := pdfsign.Sign(context.Background(), req)
	if err != nil {
		t.Fatalf("pdfsign.Sign: %v", err)
	}
	return out, pem
}

// postMultipart builds a multipart/form-data request with one "file"
// field containing the supplied bytes and returns the recorded
// response.
func postMultipart(t *testing.T, h *Handler, fieldName string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	if fieldName != "" {
		part, err := w.CreateFormFile(fieldName, "report.pdf")
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(body); err != nil {
			t.Fatalf("part.Write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/verify", buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeResult(t *testing.T, rec *httptest.ResponseRecorder) VerifyResult {
	t.Helper()
	var r VerifyResult
	if err := json.NewDecoder(rec.Body).Decode(&r); err != nil {
		t.Fatalf("decode result: %v body=%q", err, rec.Body.String())
	}
	return r
}

func TestServeHTTP_MissingFile(t *testing.T) {
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	rec := postMultipart(t, h, "", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_EmptyFile(t *testing.T) {
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	rec := postMultipart(t, h, "file", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_NotPDF(t *testing.T) {
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	rec := postMultipart(t, h, "file", []byte("this is plain text, not a pdf"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (business failure), got %d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeResult(t, rec)
	if res.Valid {
		t.Fatal("expected valid=false")
	}
	if !strings.Contains(res.Reason, "pdf") {
		t.Fatalf("reason should mention pdf, got %q", res.Reason)
	}
	if res.DisclaimerNote == "" {
		t.Fatal("disclaimer should always be present")
	}
}

func TestServeHTTP_HappyPath(t *testing.T) {
	signed, pubPEM := signMinimalPDF(t, false)
	h := &Handler{Verifier: &fakeVerifier{keyID: "kms-test-key", pem: pubPEM}}
	rec := postMultipart(t, h, "file", signed)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeResult(t, rec)
	if !res.Valid {
		t.Fatalf("expected valid=true, reason=%q", res.Reason)
	}
	if res.SignerKeyID != "kms-test-key" {
		t.Fatalf("SignerKeyID=%q want kms-test-key", res.SignerKeyID)
	}
	if res.ContentSHA256 == "" {
		t.Fatal("ContentSHA256 should be populated")
	}
	// Confirm the reported hash actually matches the bytes we sent.
	want := sha256.Sum256(signed)
	if res.ContentSHA256 != hexEncode(want[:]) {
		t.Fatalf("ContentSHA256 mismatch")
	}
	if res.DisclaimerNote != Disclaimer {
		t.Fatalf("disclaimer mismatch")
	}
}

func TestServeHTTP_HappyPathWithTSA(t *testing.T) {
	signed, pubPEM := signMinimalPDF(t, true)
	h := &Handler{Verifier: &fakeVerifier{keyID: "kms-test-key", pem: pubPEM}}
	rec := postMultipart(t, h, "file", signed)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeResult(t, rec)
	if !res.Valid {
		t.Fatalf("expected valid=true, reason=%q", res.Reason)
	}
	if res.SignedAt.IsZero() {
		t.Fatal("SignedAt should be populated from TSA token")
	}
}

func TestServeHTTP_TamperedSignature(t *testing.T) {
	signed, pubPEM := signMinimalPDF(t, false)
	// Flip a byte deep inside the PDF so the signed bytes change but
	// the byte range / cms blob layout stay valid. Hit a byte well
	// past the header and before the appended /Sig dict.
	tampered := make([]byte, len(signed))
	copy(tampered, signed)
	// flip a byte at offset 50 which sits inside the original PDF
	// catalog content — within the first /ByteRange slice.
	tampered[50] ^= 0xFF

	h := &Handler{Verifier: &fakeVerifier{keyID: "kms-test-key", pem: pubPEM}}
	rec := postMultipart(t, h, "file", tampered)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	res := decodeResult(t, rec)
	if res.Valid {
		t.Fatal("expected valid=false for tampered pdf")
	}
	if res.Reason == "" {
		t.Fatal("expected reason on failure")
	}
}

func TestServeHTTP_KeyMismatch(t *testing.T) {
	signed, _ := signMinimalPDF(t, false)
	// Use a DIFFERENT key in the verifier than the one that signed
	// the PDF — pkcs7 verification will still pass (cert embedded in
	// SignedData matches its own signature) but the KMS public-key
	// cross-check must fail.
	_, _, otherPEM := generateRSACert(t)

	h := &Handler{Verifier: &fakeVerifier{keyID: "wrong-key", pem: otherPEM}}
	rec := postMultipart(t, h, "file", signed)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	res := decodeResult(t, rec)
	if res.Valid {
		t.Fatal("expected valid=false on key mismatch")
	}
	if !strings.Contains(res.Reason, "key") {
		t.Fatalf("reason should mention key mismatch, got %q", res.Reason)
	}
}

func TestServeHTTP_KMSUnavailable(t *testing.T) {
	signed, _ := signMinimalPDF(t, false)
	h := &Handler{Verifier: &fakeVerifier{keyID: "k", err: errors.New("kms boom")}}
	rec := postMultipart(t, h, "file", signed)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	res := decodeResult(t, rec)
	if res.Valid {
		t.Fatal("expected valid=false when kms is unreachable")
	}
	if !strings.Contains(res.Reason, "kms") {
		t.Fatalf("reason should mention kms, got %q", res.Reason)
	}
}

func TestServeHTTP_TooLarge(t *testing.T) {
	h := &Handler{
		Verifier:    &fakeVerifier{keyID: "k"},
		MaxPDFBytes: 1024,
	}
	// Exceed the cap.
	big := make([]byte, 2048)
	rec := postMultipart(t, h, "file", big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	req := httptest.NewRequest(http.MethodPut, "/verify", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestServeHTTP_NilHandler(t *testing.T) {
	// Verifier nil is a misconfiguration — guard rail returns 500.
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/verify", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
}

// ---------- GET /verify/{id} ----------

type fakeLookup struct {
	records map[string]*KnownReport
	err     error
}

func (f *fakeLookup) LookupByID(_ context.Context, id string) (*KnownReport, error) {
	if f.err != nil {
		return nil, f.err
	}
	r, ok := f.records[id]
	if !ok {
		return nil, ErrReportNotFound
	}
	return r, nil
}

func TestServeHTTP_GetUnknown(t *testing.T) {
	h := &Handler{
		Verifier:     &fakeVerifier{keyID: "k"},
		ReportLookup: &fakeLookup{records: map[string]*KnownReport{}},
	}
	req := httptest.NewRequest(http.MethodGet, "/verify/missing-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestServeHTTP_GetHappy(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := &Handler{
		Verifier: &fakeVerifier{keyID: "k"},
		ReportLookup: &fakeLookup{
			records: map[string]*KnownReport{
				"rep-123": {
					ID:             "rep-123",
					ContentHash:    "deadbeef",
					SignatureKeyID: "kms-key-1",
					TSAProvider:    "digicert",
					TSATime:        now,
					ReportType:     "observation_only",
				},
			},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/verify/rep-123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	res := decodeResult(t, rec)
	if !res.Valid || res.ReportID != "rep-123" || res.SignerKeyID != "kms-key-1" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.ReportType != "observation_only" {
		t.Fatalf("ReportType=%q", res.ReportType)
	}
	if res.DisclaimerNote != Disclaimer {
		t.Fatal("disclaimer missing on GET response")
	}
}

func TestServeHTTP_GetWithoutLookup(t *testing.T) {
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	req := httptest.NewRequest(http.MethodGet, "/verify/some-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 when no lookup configured, got %d", rec.Code)
	}
}

func TestServeHTTP_GetLookupError(t *testing.T) {
	h := &Handler{
		Verifier:     &fakeVerifier{keyID: "k"},
		ReportLookup: &fakeLookup{err: errors.New("db down")},
	}
	req := httptest.NewRequest(http.MethodGet, "/verify/any", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
}

func TestPublicKeyMatches(t *testing.T) {
	cert, _, pemBytes := generateRSACert(t)
	ok, err := publicKeyMatches(pemBytes, cert.RawSubjectPublicKeyInfo)
	if err != nil {
		t.Fatalf("publicKeyMatches: %v", err)
	}
	if !ok {
		t.Fatal("expected match for self-derived key")
	}

	// Cross test: different key, different cert => no match.
	other, _, _ := generateRSACert(t)
	ok, err = publicKeyMatches(pemBytes, other.RawSubjectPublicKeyInfo)
	if err != nil {
		t.Fatalf("publicKeyMatches: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch for different key")
	}
}

func TestPublicKeyMatches_BadPEM(t *testing.T) {
	_, err := publicKeyMatches([]byte("not a pem"), []byte{0x30, 0x82})
	if err == nil {
		t.Fatal("expected error on bad pem")
	}
	_, err = publicKeyMatches(nil, []byte{0x30, 0x82})
	if err == nil {
		t.Fatal("expected error on empty pem")
	}
}

func TestIsTooLarge(t *testing.T) {
	if !isTooLarge(errors.New("http: request body too large")) {
		t.Fatal("expected isTooLarge true for canonical message")
	}
	if isTooLarge(nil) {
		t.Fatal("nil err must not be too large")
	}
	if isTooLarge(errors.New("unrelated")) {
		t.Fatal("unrelated err must not be too large")
	}
}

// TestHandler_LoggerOverride covers the `h.Logger != nil` branch in
// the logger() helper. We use a custom slog logger and trigger a path
// that calls logger() — handleGET on an error lookup logs via h.logger().
func TestHandler_LoggerOverride(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{}))
	h := &Handler{
		Verifier:     &fakeVerifier{keyID: "k"},
		ReportLookup: &fakeLookup{err: errors.New("db down")},
		Logger:       logger,
	}
	req := httptest.NewRequest(http.MethodGet, "/verify/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "lookup failed") {
		t.Fatalf("custom logger did not receive the lookup-failed line: %q", buf.String())
	}
}

// nilReturningLookup returns (nil, nil) — a degenerate but possible
// implementation that the handler must treat as 404.
type nilReturningLookup struct{}

func (nilReturningLookup) LookupByID(context.Context, string) (*KnownReport, error) {
	return nil, nil
}

func TestServeHTTP_GetReturnsNilRecord(t *testing.T) {
	h := &Handler{
		Verifier:     &fakeVerifier{keyID: "k"},
		ReportLookup: nilReturningLookup{},
	}
	req := httptest.NewRequest(http.MethodGet, "/verify/some-id", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for nil record, got %d", rec.Code)
	}
}

// TestServeHTTP_FileExceedsCapAfterRead exercises the post-ReadAll size
// guard. We craft a multipart upload whose total body fits ParseMultipartForm's
// memory budget but whose extracted file bytes exceed max. The simplest
// trigger: set MaxPDFBytes to 0 — which falls back to DefaultMaxPDFBytes
// (32 MiB) — so this branch is impractical to hit without writing 32 MiB
// to the test buffer. Instead, we verify the empty-file branch on the
// POST handler when the form file has zero bytes but a Content-Disposition
// header (rare clients send empty multipart files with a header).
func TestServeHTTP_MalformedMultipart(t *testing.T) {
	// Send a body declared as multipart/form-data but with garbage
	// content — ParseMultipartForm will error out with a non-too-large
	// error, hitting the "failed to parse multipart form" branch.
	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte("not a multipart body")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=----nonexistent")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "multipart") {
		t.Fatalf("expected multipart error in body, got %q", rec.Body.String())
	}
}

// TestVerifyPDF_ParseCMSFailure exercises the pkcs7.Parse error branch.
// We build a PDF whose /Contents contains a valid hex blob but that
// blob is NOT well-formed CMS, so pkcs7.Parse rejects it.
func TestVerifyPDF_ParseCMSFailure(t *testing.T) {
	// /ByteRange [0 8 X Y] covers the "%PDF-1.4" header bytes; the
	// /Contents hex decodes to "not cms" which pkcs7.Parse will reject.
	garbageHex := "6E6F7420636D73" // "not cms"
	pdf := "%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Sig /ByteRange [0 8 100 50] /Contents <" + garbageHex + "> >>\nendobj\n%%EOF\n"
	// Pad so the byte range maths is valid against the file size.
	for len(pdf) < 200 {
		pdf += " "
	}

	h := &Handler{Verifier: &fakeVerifier{keyID: "k"}}
	rec := postMultipart(t, h, "file", []byte(pdf))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	res := decodeResult(t, rec)
	if res.Valid {
		t.Fatal("expected valid=false")
	}
	if !strings.Contains(res.Reason, "parse cms") {
		t.Fatalf("reason=%q want 'parse cms'", res.Reason)
	}
}

// hexEncode mirrors hex.EncodeToString without importing the package
// just for one call in tests.
func hexEncode(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0F]
	}
	return string(out)
}
