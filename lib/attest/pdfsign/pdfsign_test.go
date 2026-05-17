package pdfsign

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digitorus/timestamp"
)

// minimalPDF is a hand-crafted PDF 1.4 with one empty page. Enough for
// github.com/digitorus/pdf to parse and for the signing pipeline to
// append an incremental update.
//
// The xref offsets in this string are correct for the literal byte
// layout below — do not edit without recomputing offsets.
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

// generateTestCert produces a self-signed RSA cert + private key for
// use as the signer identity in tests. RSA keeps PKCS#7 / pdfsign on
// its well-trodden path; the KMS path in production uses the same
// crypto.Signer interface so this exercises the adapter realistically.
func generateTestCert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
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
	return cert, key
}

// newTSAServer fakes an RFC3161 TSA. It parses the incoming request,
// builds a real TimeStampToken using the supplied signing key, and
// returns it. status=mode lets each test choose 200 / 503 etc.
func newTSAServer(t *testing.T, statusCode int, tsaKey *rsa.PrivateKey, tsaCert *x509.Certificate) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode != 200 {
			w.WriteHeader(statusCode)
			_, _ = w.Write([]byte("simulated tsa failure"))
			return
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
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
			Qualified:     false,
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

// rsaSignClosure builds a KMSSign closure backed by a local RSA key,
// matching how production wires AWS / Aliyun KMS clients into Sign().
func rsaSignClosure(key *rsa.PrivateKey) func(context.Context, []byte) ([]byte, error) {
	return func(_ context.Context, digest []byte) ([]byte, error) {
		return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	}
}

func TestSign_InvalidPDF(t *testing.T) {
	cert, key := generateTestCert(t)
	_, err := Sign(context.Background(), SignRequest{
		Input:             []byte("not a pdf"),
		KMSSign:           rsaSignClosure(key),
		SignerCertificate: cert,
	})
	if !errors.Is(err, ErrInvalidPDF) {
		t.Fatalf("expected ErrInvalidPDF, got %v", err)
	}
}

func TestSign_MissingSigner(t *testing.T) {
	cert, _ := generateTestCert(t)
	_, err := Sign(context.Background(), SignRequest{
		Input:             []byte(minimalPDF),
		SignerCertificate: cert,
	})
	if !errors.Is(err, ErrPDFAssembly) {
		t.Fatalf("expected ErrPDFAssembly for missing KMSSign, got %v", err)
	}
}

func TestSign_MissingCertificate(t *testing.T) {
	_, key := generateTestCert(t)
	_, err := Sign(context.Background(), SignRequest{
		Input:   []byte(minimalPDF),
		KMSSign: rsaSignClosure(key),
	})
	if !errors.Is(err, ErrPDFAssembly) {
		t.Fatalf("expected ErrPDFAssembly for missing cert, got %v", err)
	}
}

func TestSign_KMSFailure(t *testing.T) {
	cert, _ := generateTestCert(t)
	want := errors.New("kms boom")
	_, err := Sign(context.Background(), SignRequest{
		Input:             []byte(minimalPDF),
		SignerCertificate: cert,
		KMSSign: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, want
		},
	})
	if !errors.Is(err, ErrKMSSignFailed) {
		t.Fatalf("expected ErrKMSSignFailed, got %v", err)
	}
}

func TestSign_HappyPathWithTSA(t *testing.T) {
	cert, key := generateTestCert(t)
	srv := newTSAServer(t, 200, key, cert)
	defer srv.Close()

	out, err := Sign(context.Background(), SignRequest{
		Input:             []byte(minimalPDF),
		KMSSign:           rsaSignClosure(key),
		SignerCertificate: cert,
		TSAEndpoint:       srv.URL,
		Name:              "idcd-test",
		Reason:            "unit test",
		Location:          "ci",
	})
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if len(out) <= len(minimalPDF) {
		t.Fatalf("expected signed pdf to grow; in=%d out=%d", len(minimalPDF), len(out))
	}
	if string(out[:5]) != "%PDF-" {
		t.Fatalf("output missing %%PDF- header")
	}
	// The incremental update layer must write a /ByteRange entry.
	if !contains(out, []byte("/ByteRange")) {
		t.Fatalf("output PDF lacks /ByteRange dict")
	}
	if !contains(out, []byte("/Sig")) {
		t.Fatalf("output PDF lacks /Sig dict")
	}
}

func TestSign_TSAFailure(t *testing.T) {
	cert, key := generateTestCert(t)
	srv := newTSAServer(t, http.StatusServiceUnavailable, key, cert)
	defer srv.Close()

	_, err := Sign(context.Background(), SignRequest{
		Input:             []byte(minimalPDF),
		KMSSign:           rsaSignClosure(key),
		SignerCertificate: cert,
		TSAEndpoint:       srv.URL,
	})
	if !errors.Is(err, ErrTSAFailed) {
		t.Fatalf("expected ErrTSAFailed, got %v", err)
	}
}

func TestKMSSignerAdapter(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa key: %v", err)
	}
	digest := sha256.Sum256([]byte("payload"))
	adapter := &kmsSignerAdapter{
		pub: key.Public(),
		signFn: func(d []byte) ([]byte, error) {
			return ecdsa.SignASN1(rand.Reader, key, d)
		},
	}
	if adapter.Public() == nil {
		t.Fatal("Public() returned nil")
	}
	sig, err := adapter.Sign(nil, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("adapter.Sign: %v", err)
	}
	if !ecdsa.VerifyASN1(key.Public().(*ecdsa.PublicKey), digest[:], sig) {
		t.Fatal("signature did not verify")
	}
}

func TestIsPDF(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want bool
	}{
		{"valid", []byte("%PDF-1.4\n"), true},
		{"too short", []byte("%PD"), false},
		{"wrong magic", []byte("PK\x03\x04"), false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPDF(c.in); got != c.want {
				t.Fatalf("isPDF(%q)=%v want %v", c.in, got, c.want)
			}
		})
	}
}

func TestIsTSAError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("xref broken"), false},
		{"get timestamp", errors.New("get timestamp: blah"), true},
		{"non success", errors.New("non success response (503)"), true},
		{"TSA token parse", errors.New("failed to parse timestamp"), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTSAError(c.err); got != c.want {
				t.Fatalf("isTSAError(%v)=%v want %v", c.err, got, c.want)
			}
		})
	}
}

// contains is a tiny helper so we avoid importing bytes here just for
// a single call.
func contains(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
