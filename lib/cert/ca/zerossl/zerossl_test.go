package zerossl

import (
	"context"
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"time"

	legoacme "github.com/go-acme/lego/v4/acme"

	"github.com/kite365/idcd/lib/cert/ca"
)

// validEABConfig returns a Config with placeholder EAB credentials that
// pass validateRequest's non-empty check. The values are not real and
// only used for input-validation tests; no network I/O happens.
func validEABConfig() Config {
	return Config{
		EABKID:     "kid-test",
		EABHMACKey: "aGVsbG8td29ybGQtaG1hYy1rZXktdGVzdA",
	}
}

func TestNewReturnsImplementation(t *testing.T) {
	c := New(validEABConfig())
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewWithEmptyConfigStillReturnsImplementation(t *testing.T) {
	// New must not panic / nil-return even when EAB is missing; the
	// emptiness is reported as ca.ErrInvalidInput at request time so
	// callers can construct the adapter early in boot and load EAB
	// material lazily from vault.
	c := New(Config{})
	if c == nil {
		t.Fatal("New(Config{}) returned nil; expected lazy validation")
	}
}

func TestMetadata(t *testing.T) {
	c := New(validEABConfig())
	if got := c.Name(); got != "zerossl" {
		t.Errorf("Name = %q, want %q", got, "zerossl")
	}
	if got := c.Tier(); got != ca.TierFreeDV {
		t.Errorf("Tier = %q, want %q", got, ca.TierFreeDV)
	}
	if !c.SupportsWildcard() {
		t.Error("SupportsWildcard = false, want true (ZeroSSL supports wildcards via dns-01)")
	}
	if got := c.ValidityDays(); got != 90 {
		t.Errorf("ValidityDays = %d, want 90", got)
	}
	chals := c.SupportedChallenges()
	if len(chals) != 1 || chals[0] != ca.ChallengeDNS01 {
		t.Errorf("SupportedChallenges = %v, want [dns-01]", chals)
	}
}

func TestDirectoryURL(t *testing.T) {
	c := New(validEABConfig()).(*zerosslCA)
	if got := c.directoryURL(); got != "https://acme.zerossl.com/v2/DV90" {
		t.Errorf("directoryURL = %q, want https://acme.zerossl.com/v2/DV90", got)
	}
	// Empty Config should not change the directory URL — ZeroSSL has
	// no staging directory; local integration tests run against Pebble.
	c2 := New(Config{}).(*zerosslCA)
	if got := c2.directoryURL(); got != "https://acme.zerossl.com/v2/DV90" {
		t.Errorf("directoryURL (empty cfg) = %q, want https://acme.zerossl.com/v2/DV90", got)
	}
}

func TestDns01Record(t *testing.T) {
	domain := "example.com"
	keyAuth := "token.thumbprint"

	fqdn, value := dns01Record(domain, keyAuth)

	wantFQDN := "_acme-challenge.example.com."
	if fqdn != wantFQDN {
		t.Errorf("fqdn = %q, want %q", fqdn, wantFQDN)
	}

	sum := sha256.Sum256([]byte(keyAuth))
	wantValue := base64.RawURLEncoding.EncodeToString(sum[:])
	if value != wantValue {
		t.Errorf("value = %q, want %q (base64url(sha256(keyAuth)))", value, wantValue)
	}
}

// fakeSolver records what the adapter asked it to write so we can assert
// on the fqdn / value the adapter computed from lego's (domain, token, keyAuth).
type fakeSolver struct {
	presentFQDN  string
	presentValue string
	cleanupFQDN  string
	cleanupValue string
	timeout      time.Duration
}

func (f *fakeSolver) Present(_ context.Context, fqdn, value string) error {
	f.presentFQDN = fqdn
	f.presentValue = value
	return nil
}

func (f *fakeSolver) CleanUp(_ context.Context, fqdn, value string) error {
	f.cleanupFQDN = fqdn
	f.cleanupValue = value
	return nil
}

func (f *fakeSolver) Timeout() time.Duration { return f.timeout }

func TestLegoProviderAdaptsSolver(t *testing.T) {
	solver := &fakeSolver{timeout: 90 * time.Second}
	p := &legoProvider{ctx: context.Background(), solver: solver}

	const domain = "sub.example.com"
	const keyAuth = "tok.thumbprint"

	if err := p.Present(domain, "tok", keyAuth); err != nil {
		t.Fatalf("Present: %v", err)
	}
	wantFQDN := "_acme-challenge.sub.example.com."
	if solver.presentFQDN != wantFQDN {
		t.Errorf("present fqdn = %q, want %q", solver.presentFQDN, wantFQDN)
	}
	sum := sha256.Sum256([]byte(keyAuth))
	wantValue := base64.RawURLEncoding.EncodeToString(sum[:])
	if solver.presentValue != wantValue {
		t.Errorf("present value = %q, want base64url(sha256(keyAuth))=%q", solver.presentValue, wantValue)
	}

	if err := p.CleanUp(domain, "tok", keyAuth); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if solver.cleanupFQDN != wantFQDN || solver.cleanupValue != wantValue {
		t.Errorf("cleanup not symmetrical: fqdn=%q value=%q", solver.cleanupFQDN, solver.cleanupValue)
	}

	to, ivl := p.Timeout()
	if to != 90*time.Second {
		t.Errorf("timeout passthrough = %v, want 90s", to)
	}
	if ivl != 5*time.Second {
		t.Errorf("interval = %v, want 5s", ivl)
	}
}

func TestLegoProviderTimeoutDefault(t *testing.T) {
	p := &legoProvider{ctx: context.Background(), solver: &fakeSolver{}}
	to, _ := p.Timeout()
	if to != 2*time.Minute {
		t.Errorf("default timeout = %v, want 2m", to)
	}
}

func TestValidateRequest(t *testing.T) {
	dummyKey := &stubSigner{}
	dummyDNS := &fakeSolver{}
	good := validEABConfig()

	cases := []struct {
		name string
		ca   *zerosslCA
		req  ca.CertificateRequest
		ok   bool
	}{
		{
			name: "missing account key",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{Domains: []string{"x"}, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "no domains and no csr",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{AccountKey: dummyKey, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "missing key & csr",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "missing dns solver",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, PrivateKey: dummyKey},
			ok:   false,
		},
		{
			name: "missing EABKID",
			ca:   &zerosslCA{cfg: Config{EABHMACKey: "k"}},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "missing EABHMACKey",
			ca:   &zerosslCA{cfg: Config{EABKID: "k"}},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "missing both EAB fields",
			ca:   &zerosslCA{cfg: Config{}},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   false,
		},
		{
			name: "ok with privkey + domain",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{AccountKey: dummyKey, Domains: []string{"x"}, PrivateKey: dummyKey, DNS: dummyDNS},
			ok:   true,
		},
		{
			name: "ok with csr",
			ca:   &zerosslCA{cfg: good},
			req:  ca.CertificateRequest{AccountKey: dummyKey, CSR: []byte("---"), DNS: dummyDNS},
			ok:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.ca.validateRequest(c.req)
			if c.ok && err != nil {
				t.Fatalf("expected ok, got %v", err)
			}
			if !c.ok {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, ca.ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
			}
		})
	}
}

// TestMapErr exercises the lego problem-type → sentinel mapping with a
// hand-built ProblemDetails so we don't depend on a live ACME server.
func TestMapErr(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{
			name: "nil passes through",
			in:   nil,
			want: nil,
		},
		{
			name: "rate limited typed envelope",
			in:   &legoacme.RateLimitedError{ProblemDetails: &legoacme.ProblemDetails{Type: legoacme.RateLimitedErr, HTTPStatus: 429}},
			want: ca.ErrCAQuotaExceeded,
		},
		{
			name: "nonce error typed envelope",
			in:   &legoacme.NonceError{ProblemDetails: &legoacme.ProblemDetails{Type: legoacme.BadNonceErr, HTTPStatus: 400}},
			want: ca.ErrNetwork,
		},
		{
			name: "caa problem",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:caa", HTTPStatus: 403, Detail: "CAA forbids issuance"},
			want: ca.ErrCAATooStrict,
		},
		{
			name: "rateLimited problem",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:rateLimited", HTTPStatus: 429},
			want: ca.ErrCAQuotaExceeded,
		},
		{
			name: "unauthorized",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:unauthorized", HTTPStatus: 403},
			want: ca.ErrAccountInvalid,
		},
		{
			name: "accountDoesNotExist",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:accountDoesNotExist", HTTPStatus: 400},
			want: ca.ErrAccountInvalid,
		},
		{
			name: "externalAccountRequired",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:externalAccountRequired", HTTPStatus: 403, Detail: "EAB required"},
			want: ca.ErrAccountInvalid,
		},
		{
			name: "badNonce",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:badNonce", HTTPStatus: 400},
			want: ca.ErrNetwork,
		},
		{
			name: "serverInternal",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:serverInternal", HTTPStatus: 500},
			want: ca.ErrNetwork,
		},
		{
			name: "malformed -> invalid input",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:malformed", HTTPStatus: 400},
			want: ca.ErrInvalidInput,
		},
		{
			name: "badCSR -> invalid input",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:badCSR", HTTPStatus: 400},
			want: ca.ErrInvalidInput,
		},
		{
			name: "rejectedIdentifier -> invalid input",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:rejectedIdentifier", HTTPStatus: 400},
			want: ca.ErrInvalidInput,
		},
		{
			name: "unknown 5xx -> network",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:weird", HTTPStatus: 503},
			want: ca.ErrNetwork,
		},
		{
			name: "unknown 4xx authz failure",
			in:   &legoacme.ProblemDetails{Type: "urn:ietf:params:acme:error:dns", HTTPStatus: 400},
			want: ca.ErrAuthzInvalid,
		},
		{
			name: "plain error -> network",
			in:   errors.New("dial timeout"),
			want: ca.ErrNetwork,
		},
		{
			name: "sentinel passes through",
			in:   ca.ErrInvalidInput,
			want: ca.ErrInvalidInput,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapErr(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("want nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want %v, got nil", tc.want)
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want errors.Is(_, %v)", got, tc.want)
			}
		})
	}
}

func TestProblemKind(t *testing.T) {
	if k := problemKind("urn:ietf:params:acme:error:externalAccountRequired"); k != "externalAccountRequired" {
		t.Errorf("kind = %q, want externalAccountRequired", k)
	}
	if k := problemKind("urn:ietf:params:acme:error:caa"); k != "caa" {
		t.Errorf("kind = %q, want caa", k)
	}
	if k := problemKind("something:else"); k != "something:else" {
		t.Errorf("non-acme passes through unchanged, got %q", k)
	}
}

// TestZsUser confirms the registration.User adapter wiring; this is
// trivial but covers the GetEmail / GetPrivateKey / GetRegistration
// surface lego depends on.
func TestZsUser(t *testing.T) {
	key := stubSigner{}
	u := &zsUser{email: "ops@example.com", key: key}
	if u.GetEmail() != "ops@example.com" {
		t.Errorf("GetEmail = %q, want ops@example.com", u.GetEmail())
	}
	if u.GetPrivateKey() == nil {
		t.Error("GetPrivateKey = nil; should reflect supplied key")
	}
	if u.GetRegistration() != nil {
		t.Error("GetRegistration = non-nil for fresh user; should be nil until register")
	}
}

// TestZeroSSL_Integration_Pebble is deliberately skipped.
//
// Pebble emulates Let's Encrypt's ACME flavour, including LE's specific
// problem-document URN namespace and its NO-EAB registration path.
// ZeroSSL requires External Account Binding for every newAccount, and
// the only way to obtain real EAB credentials is the ZeroSSL portal —
// there is no public ZeroSSL staging directory and Pebble cannot
// accept ZeroSSL-issued EAB material. End-to-end coverage of ZeroSSL
// therefore needs a manual smoke test against the production directory
// with throwaway EAB credentials (run out-of-band, not on CI).
//
// The local LE-against-Pebble test (see ../letsencrypt/le_test.go)
// already exercises the shared ACME plumbing: newOrder + DNS-01 +
// finalize + revoke. The ZeroSSL adapter's only material differences
// are (a) the EAB envelope, covered by validateRequest unit tests, and
// (b) the directory URL, covered by TestDirectoryURL above.
func TestZeroSSL_Integration_Pebble(t *testing.T) {
	t.Skip("ZeroSSL integration needs real EAB credentials and has no public staging; local LE-on-Pebble validates the shared ACME path")
}

// stubSigner is a non-functional crypto.Signer used purely for input-validation
// tests. Real flows require a real signer; this is enough to walk the
// validateRequest path without touching network.
type stubSigner struct{}

func (stubSigner) Public() crypto.PublicKey { return nil }
func (stubSigner) Sign(_ io.Reader, _ []byte, _ crypto.SignerOpts) ([]byte, error) {
	return nil, nil
}

var _ crypto.Signer = stubSigner{}
