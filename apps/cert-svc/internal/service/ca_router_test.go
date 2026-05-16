package service

import (
	"context"
	"crypto"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
	"github.com/kite365/idcd/lib/cert/dns"
)

func TestNewRouter_PickReturnsLE(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	r := NewRouter(le)

	got, err := r.Pick()
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "lets-encrypt", got.Name())
}

func TestNewRouter_NilReturnsError(t *testing.T) {
	var r *Router
	_, err := r.Pick()
	assert.ErrorIs(t, err, ErrNoCA)

	r2 := NewRouter(nil)
	_, err = r2.Pick()
	assert.ErrorIs(t, err, ErrNoCA)
}

func TestServiceCAPick_RoutesViaRouter(t *testing.T) {
	fake := &fakeCA{name: "fake-le"}
	svc := New(Config{Router: NewRouter(fake)})

	got, err := svc.caPick(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "fake-le", got.Name())
}

// fakeCA is a minimal ca.AcmeCA used by router / orchestrator tests.
type fakeCA struct {
	name           string
	requestErr     error
	requestResult  ca.CertificateResult
	requestCalls   int
	revokeErr      error
	revokeCalls    int
	lastRequest    ca.CertificateRequest
	skipUnused     bool
}

func (f *fakeCA) Name() string                            { return f.name }
func (f *fakeCA) Tier() ca.Tier                           { return ca.TierFreeDV }
func (f *fakeCA) SupportsWildcard() bool                  { return true }
func (f *fakeCA) ValidityDays() int                       { return 90 }
func (f *fakeCA) SupportedChallenges() []ca.ChallengeType { return []ca.ChallengeType{ca.ChallengeDNS01} }

func (f *fakeCA) RequestCertificate(_ context.Context, req ca.CertificateRequest) (ca.CertificateResult, error) {
	f.requestCalls++
	f.lastRequest = req
	if f.requestErr != nil {
		return ca.CertificateResult{}, f.requestErr
	}
	return f.requestResult, nil
}

func (f *fakeCA) Revoke(_ context.Context, _ []byte, _ ca.RevokeReason, _ crypto.Signer) error {
	f.revokeCalls++
	return f.revokeErr
}

// Compile-time interface assertion.
var _ ca.AcmeCA = (*fakeCA)(nil)

var errFakeNetwork = errors.New("fake network")

// stubProvider is a minimal dns.Provider used by orchestrator tests that
// need the registry to return a working solver without hitting a real DNS
// API.
type stubProvider struct {
	kind dns.ProviderKind
}

func (s *stubProvider) Kind() dns.ProviderKind                                    { return s.kind }
func (s *stubProvider) ValidateCredential(_ map[string]string) error              { return nil }
func (s *stubProvider) HealthCheck(_ context.Context, _ map[string]string) error  { return nil }
func (s *stubProvider) BuildSolver(_ context.Context, _ map[string]string, _ []string) (ca.DnsSolver, error) {
	return &stubSolver{}, nil
}

type stubSolver struct{}

func (s *stubSolver) Present(_ context.Context, _, _ string) error { return nil }
func (s *stubSolver) CleanUp(_ context.Context, _, _ string) error { return nil }
func (s *stubSolver) Timeout() time.Duration                       { return time.Second }
