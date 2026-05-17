// Package pdfsign implements PAdES B-T style PDF signing for idcd's
// S2 Evidence / Attestation pipeline (step 4-8 in
// docs/prd/18-evidence-and-attestation.md §3.2).
//
// The caller passes an already-rendered PDF, a KMS-backed sign closure,
// the corresponding X.509 signer certificate, and a RFC3161 TSA URL.
// Sign() returns a PDF with an embedded CMS/PKCS#7 SignedData containing
// an RFC3161 TimeStampToken (PAdES-B-T).
//
// This package does not render the PDF (the verdict generator handles
// that). It also does not implement crash-safe idempotency — the outer
// attestation_record WAL (D4) covers re-runs, and callers are expected
// to pass an idempotency token to KMS inside the KMSSign closure.
package pdfsign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"io"

	pdfreader "github.com/digitorus/pdf"
	pdfsignlib "github.com/digitorus/pdfsign/sign"
)

// SignRequest describes all inputs needed for one PDF signing call.
type SignRequest struct {
	// Input is the rendered PDF bytes to sign.
	Input []byte

	// KMSSign submits a digest to KMS and returns the signature. The
	// caller is responsible for passing an idempotency key to KMS; this
	// package does not implement idempotency (the attestation_record
	// WAL in the outer layer handles crash-safe re-runs — D4).
	KMSSign func(ctx context.Context, digest []byte) (signature []byte, err error)

	// SignerCertificate is the X.509 certificate whose public key
	// matches the KMS-held private key. Embedded into the PDF /Contents
	// so verifiers can locate the public key; not used for signing.
	SignerCertificate *x509.Certificate

	// CertificateChain holds optional intermediate + root certificates.
	// May be empty (e.g. self-signed test certs).
	CertificateChain []*x509.Certificate

	// TSAEndpoint is an RFC3161 TSA URL (e.g. "http://timestamp.digicert.com").
	// pdfsign calls it to fetch a TimeStampToken which is embedded into
	// the CMS signed-data unsigned attributes (PAdES-T layer).
	//
	// Empty TSAEndpoint produces PAdES-B only (no embedded timestamp).
	// Not recommended for production verdicts.
	TSAEndpoint string

	// TSAUsername / TSAPassword enable HTTP Basic auth for the TSA call
	// (some commercial TSAs require it). Both empty = no auth.
	TSAUsername string
	TSAPassword string

	// Name / Reason / Location populate the /Sig dictionary metadata.
	Name     string
	Reason   string
	Location string
}

// Sentinel errors returned by Sign. Outer layers (verdict generator)
// branch on these via errors.Is to drive retry / failure semantics
// (D4 / D5 / D12).
var (
	// ErrInvalidPDF indicates the Input bytes are not a valid PDF (no
	// %PDF- header or parser rejected the file).
	ErrInvalidPDF = errors.New("pdfsign: invalid input pdf")

	// ErrKMSSignFailed wraps a failure from the caller-supplied
	// KMSSign closure.
	ErrKMSSignFailed = errors.New("pdfsign: kms sign failed")

	// ErrTSAFailed indicates the RFC3161 TSA request failed (HTTP
	// non-2xx, parse error, or network failure).
	ErrTSAFailed = errors.New("pdfsign: tsa request failed")

	// ErrPDFAssembly indicates the incremental update layout / xref
	// rewrite / signature embedding step failed.
	ErrPDFAssembly = errors.New("pdfsign: incremental update assembly failed")
)

// kmsSignerAdapter wraps an idcd KMS sign closure as a stdlib
// crypto.Signer so digitorus/pdfsign can call it as if it were a local
// in-process private key. The KMS round-trip happens inside signFn.
type kmsSignerAdapter struct {
	pub    crypto.PublicKey
	signFn func(digest []byte) ([]byte, error)
}

func (a *kmsSignerAdapter) Public() crypto.PublicKey { return a.pub }

func (a *kmsSignerAdapter) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	return a.signFn(digest)
}

// isPDF returns true when the buffer starts with the %PDF- magic.
func isPDF(buf []byte) bool {
	return len(buf) >= 5 && bytes.Equal(buf[:5], []byte("%PDF-"))
}

// Sign produces a PAdES-B-T signed PDF from req.Input. The output is
// returned as bytes; callers persist to S3 / attestation_record.
//
// Workflow (per docs/prd/18 §3.2 step 4-8):
//   - Validate input PDF magic.
//   - Wrap req.KMSSign in a crypto.Signer adapter.
//   - Hand to digitorus/pdfsign which:
//   - appends an incremental update section with a /Sig dict +
//     /Contents placeholder,
//   - computes the ByteRange digest,
//   - calls our Signer (= KMS),
//   - fetches an RFC3161 TimeStampToken from req.TSAEndpoint,
//   - embeds CMS + token into /Contents,
//   - rewrites the xref / trailer.
//
// All non-nil errors wrap a sentinel from this package.
func Sign(ctx context.Context, req SignRequest) ([]byte, error) {
	if !isPDF(req.Input) {
		return nil, ErrInvalidPDF
	}
	if req.SignerCertificate == nil {
		return nil, fmt.Errorf("%w: signer certificate is required", ErrPDFAssembly)
	}
	if req.KMSSign == nil {
		return nil, fmt.Errorf("%w: kms sign closure is required", ErrPDFAssembly)
	}

	// Track whether the failure originated in the KMS closure so we can
	// classify the error correctly after digitorus/pdfsign returns
	// (it wraps Signer errors generically).
	var kmsErr error

	adapter := &kmsSignerAdapter{
		pub: req.SignerCertificate.PublicKey,
		signFn: func(d []byte) ([]byte, error) {
			sig, err := req.KMSSign(ctx, d)
			if err != nil {
				kmsErr = err
				return nil, err
			}
			return sig, nil
		},
	}

	reader := bytes.NewReader(req.Input)
	pdfRdr, err := pdfreader.NewReader(reader, int64(len(req.Input)))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPDF, err)
	}

	var chains [][]*x509.Certificate
	if len(req.CertificateChain) > 0 {
		chains = [][]*x509.Certificate{req.CertificateChain}
	}

	out := &bytes.Buffer{}
	signData := pdfsignlib.SignData{
		Signature: pdfsignlib.SignDataSignature{
			Info: pdfsignlib.SignDataSignatureInfo{
				Name:     req.Name,
				Reason:   req.Reason,
				Location: req.Location,
			},
			CertType:   pdfsignlib.CertificationSignature,
			DocMDPPerm: pdfsignlib.AllowFillingExistingFormFieldsAndSignaturesPerms,
		},
		Signer:            adapter,
		DigestAlgorithm:   crypto.SHA256,
		Certificate:       req.SignerCertificate,
		CertificateChains: chains,
		TSA: pdfsignlib.TSA{
			URL:      req.TSAEndpoint,
			Username: req.TSAUsername,
			Password: req.TSAPassword,
		},
		Appearance: pdfsignlib.Appearance{Visible: false},
	}

	if err := pdfsignlib.Sign(reader, out, pdfRdr, int64(len(req.Input)), signData); err != nil {
		// Classify into our sentinel set. Order matters: KMS first
		// (we set kmsErr inside the adapter), then TSA, otherwise
		// treat as a PDF-assembly failure.
		if kmsErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrKMSSignFailed, kmsErr)
		}
		if isTSAError(err) {
			return nil, fmt.Errorf("%w: %v", ErrTSAFailed, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrPDFAssembly, err)
	}

	return out.Bytes(), nil
}

// isTSAError heuristically classifies an error returned from
// digitorus/pdfsign as originating in the RFC3161 TSA call. The
// upstream library wraps TSA failures with phrases like
// "get timestamp:" / "failed to create request" / "non success response"
// / "failed to parse timestamp" — we match those patterns so the outer
// retry layer (D4 attestation_record WAL) can distinguish transient
// TSA errors from KMS / PDF-layout errors.
func isTSAError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	needles := []string{
		"get timestamp",
		"timestamp:",
		"non success response",
		"failed to parse timestamp",
		"tsa",
		"TSA",
	}
	for _, n := range needles {
		if bytes.Contains([]byte(msg), []byte(n)) {
			return true
		}
	}
	return false
}
