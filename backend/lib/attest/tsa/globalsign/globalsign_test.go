package globalsign_test

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/digitorus/timestamp"

	"github.com/kite365/idcd/lib/attest/tsa"
	"github.com/kite365/idcd/lib/attest/tsa/globalsign"
)

// See digicert tests for the rationale behind the fixed RSA test
// material. Reusing the same fixture here keeps the two adapter test
// suites symmetric.
const testTSACertPEM = `
-----BEGIN CERTIFICATE-----
MIIDmzCCAoOgAwIBAgIUTrgB1p7WpwYXjwGs/uwfKJt4cFcwDQYJKoZIhvcNAQEL
BQAwXTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDEWMBQGA1UEAwwNVGVzdCBSU0EgQ2Vy
dDAeFw0yMDAzMDQyMjA4MDVaFw00MDAyMjgyMjA4MDVaMF0xCzAJBgNVBAYTAkFV
MRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRz
IFB0eSBMdGQxFjAUBgNVBAMMDVRlc3QgUlNBIENlcnQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQCl56dwf7hajdHLrm3b8V1mQcvJByLO/xe77g1MQYXn
TZ89XbIxrLj9lT4Zd5VM+HB8m4WPUPwh3qySBnUOUDP5ykipBChpS5Uzozkwwnph
x/bsoCySdCIQwjsFzIkGeLVz9qyksx3cDA+f/hdXB5f4ovwW1s2i5qQo68pP0wfb
eFSom5horHLFEAG25Fhqrc+sC9HnmBv9//Mse8+Hnu8AuLgndZIh49c2k3Ok6lki
P4n5QxA1BVZU/NFbtZ/Tnuj1y9KX8/94KEpYh4wCxgTn7tHJuPsydGLleoMUMJ08
uYdpPr9lwTqKMdKOgROL9S30Ew7IhSqlACzS5kff0UXDAgMBAAGjUzBRMB0GA1Ud
DgQWBBSI1Fk3y/DpAQwRXhoqRhjeQRsoCjAfBgNVHSMEGDAWgBSI1Fk3y/DpAQwR
XhoqRhjeQRsoCjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAP
+jK6M/zPFrO/hrXOXlfEntbKwxFWoil/BRVMkgMp6JO44wn9QS+oRIVKcMToTPe5
XaU4D8YgHPFiyhaTOQ95RDVZuy5VPf1li1oujPHXP6Y9Ps5RF9AKtLYdJa8ZBmRx
Cg3mHV4f6VJWziWz3s5n6DVQ5DDrSkQ0dIRs5Tu9W4+aHJUMwdkSP0klvBnlzPhq
kl++ygWDU5bJMbwD53eGieJyo5wL0SR08ijiGxCTmYOUuPl/C62MTPJU+oR8qRd3
I/rCr/gywfHmAbgupBo9ikC9rrYD5maaC59xr4NjjI1vSeS3nrO9qmd9KnGD98P8
wA4N9tN/F776b2RG2RZD
-----END CERTIFICATE-----
`

const testTSAKeyPEM = `
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEApeencH+4Wo3Ry65t2/FdZkHLyQcizv8Xu+4NTEGF502fPV2y
May4/ZU+GXeVTPhwfJuFj1D8Id6skgZ1DlAz+cpIqQQoaUuVM6M5MMJ6Ycf27KAs
knQiEMI7BcyJBni1c/aspLMd3AwPn/4XVweX+KL8FtbNouakKOvKT9MH23hUqJuY
aKxyxRABtuRYaq3PrAvR55gb/f/zLHvPh57vALi4J3WSIePXNpNzpOpZIj+J+UMQ
NQVWVPzRW7Wf057o9cvSl/P/eChKWIeMAsYE5+7Rybj7MnRi5XqDFDCdPLmHaT6/
ZcE6ijHSjoETi/Ut9BMOyIUqpQAs0uZH39FFwwIDAQABAoIBADuncUh9VD+TUQWJ
Ac2dGzVioTD2lOiTRuh3L2blBI3oFkMNhr5f2eCsojisDA4yIthbX4np188h7zFO
ixaLdjTyLHBBo3pBCDQaE71ZoIG6UipBaeV7Rqh5/pkWM4sVKkG5R9is4ya1W4Tu
61uKynVHvZdEw4o4nnxsVEGhouih5q/fmETi7XTCYSCe4gljVDtRpvFQBOrrhye/
BT38SvrXQR2WmgLLpfo+1VR5zcm9bXJXrkOKYNXWDxl9kpY+hwXD0IhTXl4GkqEe
8CP4WFHtX5WA4s9qLATp/zT7fme2Ojh+NkIdU0FMI9lf4pNX+URxii+hn15vrtCi
UxaSVtECgYEA0FobH8XOw7SWjJRs9wfLoF/Wl3s4ET9neJwx047Xlop8QAwHYzo7
CiEH+aodgr/UC8KM62+3y4pZgn3Bmt3/p/WyKOsfG3TZXqvuSGqTXO9sn3T1Z552
jVT/1/3qapHODL4ct52FHxrr243Jp2vfeMciU0tLdsx5FIgRCScqm0sCgYEAy9h/
qnDAC1fI4eEDYgj+kIUDyQegeKbi79U3aF5QjYSgvYm1pev/Zac8+x9X/zQupObB
FmgbtPYrXTY5J38qG/ELjDu7aHfXqgHcVTda0MsGsaoSCmaJ3y19ewxsmK9pFaEl
BUTmFd2hywK34RG00dyYcrvmP6M4OP/Do1+WPGkCgYEAv9lYhIcl/rr4rXW2aDk7
XO8ir9V8KRWS91IL51vuU+YsxuTMoKfr2UXVDCWCivSMElAQZnI2cStxhGC7txiX
4lawuFDYEfYkebIi9Xd9PeQQxztxBPq6+yS7eG2MPpkHfGBKHSDkhWHKsB39Azan
TZU/nCcG09sv2qH33c+8wcUCgYEAli3TqKNWqUSsZ9WZ43ES8zA8ILAwxpLVILKq
Foddu1VaAyngnPQofiDe6XgnIYq1TqH+4V4kA4dVXV/kbbffMyS8SD19jbK1PbgP
Nu0ISEk7jkro7aarrrPZ/XyiyT56IghNuPsQtE1LtMA07mlYGUD3Q5gxQvMiKcQs
w0FZ8vkCgYA7wuwLs7d9LJ4KqMNmOe0eRvIxp+Y8psxykMd1wz3PjdPz30U03xe2
o40r2ZNTK/OGYPmAOcwma7SjenBQve19eVUaECUVREmbvaJqVzz0uSrfqXrUVIiJ
YyOfhPUI5XhkyUlunO5pSAd0CtRv7NVW1wKDjMbJvgV0MlbVvGraAg==
-----END RSA PRIVATE KEY-----
`

type testTSA struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

func newTestTSA(t *testing.T) *testTSA {
	t.Helper()
	certBlock, _ := pem.Decode([]byte(testTSACertPEM))
	if certBlock == nil {
		t.Fatal("decode cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	keyBlock, _ := pem.Decode([]byte(testTSAKeyPEM))
	if keyBlock == nil {
		t.Fatal("decode key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return &testTSA{cert: cert, key: key}
}

func (tt *testTSA) stampResponse(t *testing.T, tsqBytes []byte) []byte {
	t.Helper()
	req, err := timestamp.ParseRequest(tsqBytes)
	if err != nil {
		t.Fatalf("parse TSQ: %v", err)
	}
	ts := &timestamp.Timestamp{
		HashAlgorithm:     req.HashAlgorithm,
		HashedMessage:     req.HashedMessage,
		Time:              time.Date(2026, 5, 17, 11, 45, 0, 0, time.UTC),
		Nonce:             req.Nonce,
		Policy:            asn1.ObjectIdentifier{1, 2, 3, 4, 1},
		Certificates:      []*x509.Certificate{tt.cert},
		AddTSACertificate: true,
	}
	resp, err := ts.CreateResponseWithOpts(tt.cert, tt.key, crypto.SHA256)
	if err != nil {
		t.Fatalf("create response: %v", err)
	}
	return resp
}

func TestNew_NonNil(t *testing.T) {
	if globalsign.New(globalsign.Config{}) == nil {
		t.Fatal("New returned nil")
	}
}

func TestProvider_Name(t *testing.T) {
	if got := globalsign.New(globalsign.Config{}).Name(); got != "globalsign" {
		t.Fatalf("want globalsign, got %s", got)
	}
}

func TestStamp_HappyPath(t *testing.T) {
	tt := newTestTSA(t)
	digest := sha256.Sum256([]byte("evidence-pdf-bytes"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		resp := tt.stampResponse(t, body)
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(resp)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	tok, issued, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) == 0 {
		t.Fatal("empty token")
	}
	if issued.IsZero() {
		t.Fatal("zero issuedAt")
	}
}

func TestStamp_InvalidDigestLength(t *testing.T) {
	p := globalsign.New(globalsign.Config{Endpoint: "http://example.invalid"})
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, []byte("short"))
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestStamp_UnavailableHash(t *testing.T) {
	p := globalsign.New(globalsign.Config{Endpoint: "http://example.invalid"})
	_, _, err := p.Stamp(context.Background(), crypto.Hash(0), nil)
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestStamp_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestStamp_AuthFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrAuthFailed) {
		t.Fatalf("want ErrAuthFailed, got %v", err)
	}
}

func TestStamp_400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestStamp_4xxOther(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse, got %v", err)
	}
}

func TestStamp_NotTSPBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write([]byte("garbage"))
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse, got %v", err)
	}
}

func TestStamp_StatusRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, _ := timestamp.CreateErrorResponse(timestamp.Rejection, timestamp.BadAlgorithm)
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse, got %v", err)
	}
}

func TestStamp_NetworkDown(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	p := globalsign.New(globalsign.Config{Endpoint: "http://" + addr})
	digest := sha256.Sum256([]byte("x"))
	_, _, err = p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
}

func TestStamp_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	digest := sha256.Sum256([]byte("x"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := p.Stamp(ctx, crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable on ctx cancel, got %v", err)
	}
}

func TestStamp_DigestMismatch(t *testing.T) {
	tt := newTestTSA(t)
	other := sha256.Sum256([]byte("not the requested digest"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forged, err := (&timestamp.Request{
			HashAlgorithm: crypto.SHA256,
			HashedMessage: other[:],
			Certificates:  true,
		}).Marshal()
		if err != nil {
			t.Fatalf("marshal forged: %v", err)
		}
		resp := tt.stampResponse(t, forged)
		w.Header().Set("Content-Type", "application/timestamp-reply")
		_, _ = w.Write(resp)
	}))
	defer srv.Close()

	p := globalsign.New(globalsign.Config{Endpoint: srv.URL})
	asked := sha256.Sum256([]byte("real evidence"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, asked[:])
	if !errors.Is(err, tsa.ErrInvalidResponse) {
		t.Fatalf("want ErrInvalidResponse on digest mismatch, got %v", err)
	}
}

func TestNew_DefaultsAppliedWhenZeroConfig(t *testing.T) {
	p := globalsign.New(globalsign.Config{})
	if p == nil {
		t.Fatal("nil provider")
	}
	if p.Name() != "globalsign" {
		t.Fatalf("want globalsign, got %s", p.Name())
	}
}

func TestNew_CustomHTTPClient(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripperFn(func(_ *http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("injected transport error")
	})}
	p := globalsign.New(globalsign.Config{Endpoint: "http://example.invalid", HTTPClient: client})
	digest := sha256.Sum256([]byte("x"))
	_, _, err := p.Stamp(context.Background(), crypto.SHA256, digest[:])
	if !errors.Is(err, tsa.ErrUpstreamUnavailable) {
		t.Fatalf("want ErrUpstreamUnavailable, got %v", err)
	}
	if !called {
		t.Fatal("injected HTTPClient was not used")
	}
}

type roundTripperFn func(*http.Request) (*http.Response, error)

func (f roundTripperFn) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
