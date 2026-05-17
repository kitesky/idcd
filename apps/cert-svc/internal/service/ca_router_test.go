package service

import (
	"context"
	"crypto"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/ca/letsencrypt"
	"github.com/kite365/idcd/lib/cert/dns"
)

func TestRouter_PickDefault(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	r := NewRouter(le)

	// nil order → default (renewal probe / health path).
	got, err := r.Pick(nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "lets-encrypt", got.Name())

	// order.CA empty → default.
	got, err = r.Pick(&repo.Order{CA: ""})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "lets-encrypt", got.Name())
}

func TestRouter_PickByName(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	fake := &fakeCA{name: "fake-le"}
	r := NewRouter(le, fake)

	got, err := r.Pick(&repo.Order{CA: "fake-le"})
	require.NoError(t, err)
	assert.Equal(t, "fake-le", got.Name())

	got, err = r.Pick(&repo.Order{CA: "lets-encrypt"})
	require.NoError(t, err)
	assert.Equal(t, "lets-encrypt", got.Name())
}

func TestRouter_PickUnknownCA(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	r := NewRouter(le)

	_, err := r.Pick(&repo.Order{CA: "zerossl"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownCA)
}

func TestRouter_PickNilDefault(t *testing.T) {
	r := NewRouter(nil)
	require.Nil(t, r, "NewRouter(nil) must return nil so Pick short-circuits")

	_, err := r.Pick(nil)
	assert.ErrorIs(t, err, ErrNoCA)

	_, err = r.Pick(&repo.Order{CA: "lets-encrypt"})
	assert.ErrorIs(t, err, ErrNoCA)
}

func TestRouter_NilReceiverReturnsErrNoCA(t *testing.T) {
	var r *Router
	_, err := r.Pick(nil)
	assert.ErrorIs(t, err, ErrNoCA)
}

func TestRouter_Names(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	a := &fakeCA{name: "zerossl"}
	b := &fakeCA{name: "buypass"}
	r := NewRouter(le, a, b)

	// Alphabetical order regardless of insertion order.
	assert.Equal(t, []string{"buypass", "lets-encrypt", "zerossl"}, r.Names())
}

func TestRouter_Names_NilReceiver(t *testing.T) {
	var r *Router
	assert.Nil(t, r.Names())
}

func TestRouter_NewRouterDropsNilExtras(t *testing.T) {
	le := letsencrypt.New(letsencrypt.Config{Env: letsencrypt.EnvStaging})
	// Mix nil extras in — should be dropped silently so cmd/server can
	// pass optional adapters without pre-filtering.
	r := NewRouter(le, nil, &fakeCA{name: "fake-le"}, nil)
	assert.Equal(t, []string{"fake-le", "lets-encrypt"}, r.Names())
}

func TestServiceCAPick_RoutesViaRouter(t *testing.T) {
	le := &fakeCA{name: "lets-encrypt"}
	fake := &fakeCA{name: "fake-le"}
	svc := New(Config{Router: NewRouter(le, fake)})

	// nil order → default.
	got, err := svc.caPick(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "lets-encrypt", got.Name())

	// order.CA="fake-le" → fake.
	got, err = svc.caPick(context.Background(), &repo.Order{CA: "fake-le"})
	require.NoError(t, err)
	assert.Equal(t, "fake-le", got.Name())

	// order.CA="lets-encrypt" → le.
	got, err = svc.caPick(context.Background(), &repo.Order{CA: "lets-encrypt"})
	require.NoError(t, err)
	assert.Equal(t, "lets-encrypt", got.Name())

	// Unknown → ErrUnknownCA.
	_, err = svc.caPick(context.Background(), &repo.Order{CA: "no-such-ca"})
	assert.ErrorIs(t, err, ErrUnknownCA)
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
