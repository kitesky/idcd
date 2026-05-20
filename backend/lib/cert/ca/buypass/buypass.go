// Package buypass implements the ca.AcmeCA interface against the
// Buypass Go SSL ACME endpoints using github.com/go-acme/lego/v4.
//
// Buypass Go SSL is a free DV CA out of Norway. Compared with Let's
// Encrypt the headline differences are:
//
//   - 180-day certificate validity (vs LE's 90).
//   - Wildcard issuance supported since 2023 via dns-01.
//   - No External Account Binding required; account registration is
//     a plain RFC 8555 newAccount with TermsOfServiceAgreed=true.
//
// The adapter is stateless: every RequestCertificate call constructs a
// fresh lego client from the AccountKey supplied by the caller. This
// matches the worker's model where account material lives in vault and
// is fetched per request, and avoids any in-process key cache that
// could outlive a vault rotation.
package buypass

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	legoacme "github.com/go-acme/lego/v4/acme"
	"github.com/go-acme/lego/v4/certificate"
	legochallenge "github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/kite365/idcd/lib/cert/ca"
)

// Env selects the directory endpoint.
type Env string

const (
	EnvProduction Env = "production"
	EnvStaging    Env = "staging"
)

// Buypass directory URLs. Lego does not ship constants for these, so we
// hold them here and expose them through directoryURL() for tests.
const (
	directoryProduction = "https://api.buypass.com/acme/directory"
	directoryStaging    = "https://api.test4.buypass.no/acme/directory"
)

// Config configures a Buypass adapter instance.
type Config struct {
	// Env selects production or test endpoint. Defaults to production
	// when zero-valued.
	Env Env
}

// New returns an adapter implementing ca.AcmeCA backed by Buypass Go SSL.
func New(cfg Config) ca.AcmeCA {
	env := cfg.Env
	if env == "" {
		env = EnvProduction
	}
	return &bpCA{env: env}
}

type bpCA struct {
	env Env
}

func (c *bpCA) Name() string  { return "buypass" }
func (c *bpCA) Tier() ca.Tier { return ca.TierFreeDV }

// SupportsWildcard: Buypass enabled wildcard issuance via dns-01 in 2023.
func (c *bpCA) SupportsWildcard() bool { return true }

// ValidityDays: Buypass Go SSL issues 180-day certificates.
func (c *bpCA) ValidityDays() int { return 180 }

// SupportedChallenges: this adapter only wires dns-01 because it's the
// one challenge that works for wildcards and for non-public origins.
func (c *bpCA) SupportedChallenges() []ca.ChallengeType {
	return []ca.ChallengeType{ca.ChallengeDNS01}
}

func (c *bpCA) directoryURL() string {
	if c.env == EnvStaging {
		return directoryStaging
	}
	return directoryProduction
}

// RequestCertificate runs the full ACME flow and returns the issued
// certificate. Errors are mapped to ca sentinel errors.
func (c *bpCA) RequestCertificate(ctx context.Context, req ca.CertificateRequest) (ca.CertificateResult, error) {
	if err := validateRequest(req); err != nil {
		return ca.CertificateResult{}, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	user := &bpUser{email: req.AccountEmail, key: req.AccountKey}
	client, err := newClient(c.directoryURL(), user)
	if err != nil {
		return ca.CertificateResult{}, mapErr(err)
	}

	if err := registerAccount(client, user); err != nil {
		return ca.CertificateResult{}, mapErr(err)
	}

	provider := &legoProvider{ctx: ctx, solver: req.DNS}
	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return ca.CertificateResult{}, mapErr(err)
	}

	resource, err := obtain(ctx, client, req)
	if err != nil {
		return ca.CertificateResult{}, mapErr(err)
	}

	return buildResult(resource)
}

// Revoke revokes a previously issued certificate. The caller's accountKey
// is used so this adapter remains stateless.
func (c *bpCA) Revoke(ctx context.Context, cert []byte, reason ca.RevokeReason, accountKey crypto.Signer) error {
	if len(cert) == 0 {
		return fmt.Errorf("%w: empty certificate", ca.ErrInvalidInput)
	}
	if accountKey == nil {
		return fmt.Errorf("%w: missing account key", ca.ErrInvalidInput)
	}

	user := &bpUser{key: accountKey}
	client, err := newClient(c.directoryURL(), user)
	if err != nil {
		return mapErr(err)
	}
	// Surface the user's account to the server. Revoke against an
	// unknown account would otherwise return accountDoesNotExist.
	if _, regErr := client.Registration.ResolveAccountByKey(); regErr != nil {
		return mapErr(regErr)
	}

	done := make(chan error, 1)
	go func() {
		reasonCode := uint(reason)
		done <- client.Certificate.RevokeWithReason(cert, &reasonCode)
	}()

	select {
	case err := <-done:
		if err != nil {
			return mapErr(err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ca.ErrNetwork, ctx.Err())
	}
}

// validateRequest rejects malformed CertificateRequest values before any
// network I/O.
func validateRequest(req ca.CertificateRequest) error {
	if req.AccountKey == nil {
		return fmt.Errorf("%w: missing AccountKey", ca.ErrInvalidInput)
	}
	if len(req.Domains) == 0 && len(req.CSR) == 0 {
		return fmt.Errorf("%w: Domains or CSR required", ca.ErrInvalidInput)
	}
	if len(req.CSR) == 0 && req.PrivateKey == nil {
		return fmt.Errorf("%w: PrivateKey required when CSR not provided", ca.ErrInvalidInput)
	}
	if req.DNS == nil {
		return fmt.Errorf("%w: DnsSolver required", ca.ErrInvalidInput)
	}
	return nil
}

// newClient wraps lego.NewClient with our directory URL.
func newClient(dirURL string, user *bpUser) (*lego.Client, error) {
	cfg := lego.NewConfig(user)
	cfg.CADirURL = dirURL
	cfg.UserAgent = "idcd-cert/1.0"
	return lego.NewClient(cfg)
}

// registerAccount registers the account if it has no registration yet.
// Buypass does not require External Account Binding, so this is the
// straight RFC 8555 newAccount path.
func registerAccount(client *lego.Client, user *bpUser) error {
	if user.registration != nil {
		return nil
	}
	// Try lookup first; succeeds for already-registered keys without
	// re-agreeing to ToS.
	if reg, err := client.Registration.ResolveAccountByKey(); err == nil {
		user.registration = reg
		return nil
	}
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return err
	}
	user.registration = reg
	return nil
}

// obtain runs the Certificate.Obtain or ObtainForCSR path.
func obtain(ctx context.Context, client *lego.Client, req ca.CertificateRequest) (*certificate.Resource, error) {
	// lego's Obtain is synchronous; bridge ctx cancellation via goroutine.
	type result struct {
		res *certificate.Resource
		err error
	}
	out := make(chan result, 1)

	go func() {
		if len(req.CSR) > 0 {
			parsed, err := parseCSR(req.CSR)
			if err != nil {
				out <- result{nil, err}
				return
			}
			res, err := client.Certificate.ObtainForCSR(certificate.ObtainForCSRRequest{
				CSR:    parsed,
				Bundle: false,
			})
			out <- result{res, err}
			return
		}
		res, err := client.Certificate.Obtain(certificate.ObtainRequest{
			Domains:    req.Domains,
			PrivateKey: req.PrivateKey,
			Bundle:     false,
		})
		out <- result{res, err}
	}()

	select {
	case r := <-out:
		return r.res, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ca.ErrNetwork, ctx.Err())
	}
}

func parseCSR(pemBytes []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%w: CSR is not valid PEM", ca.ErrInvalidInput)
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ca.ErrInvalidInput, err)
	}
	return csr, nil
}

// buildResult turns a lego Resource into our CertificateResult.
func buildResult(res *certificate.Resource) (ca.CertificateResult, error) {
	if res == nil || len(res.Certificate) == 0 {
		return ca.CertificateResult{}, fmt.Errorf("%w: empty certificate from CA", ca.ErrNetwork)
	}
	leafBlock, _ := pem.Decode(res.Certificate)
	if leafBlock == nil {
		return ca.CertificateResult{}, fmt.Errorf("%w: leaf is not PEM", ca.ErrNetwork)
	}
	leafCert, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		return ca.CertificateResult{}, fmt.Errorf("%w: parse leaf: %v", ca.ErrNetwork, err)
	}
	return ca.CertificateResult{
		LeafPEM:   res.Certificate,
		ChainPEM:  res.IssuerCertificate,
		IssuerURL: res.CertStableURL,
		Serial:    hex.EncodeToString(leafCert.SerialNumber.Bytes()),
		NotBefore: leafCert.NotBefore,
		NotAfter:  leafCert.NotAfter,
	}, nil
}

// bpUser adapts crypto.Signer + email into lego's registration.User.
type bpUser struct {
	email        string
	key          crypto.Signer
	registration *registration.Resource
}

func (u *bpUser) GetEmail() string                        { return u.email }
func (u *bpUser) GetRegistration() *registration.Resource { return u.registration }
func (u *bpUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// legoProvider adapts ca.DnsSolver into lego's challenge.Provider so we
// can plug a worker-supplied DNS provider into the lego solver chain.
//
// We capture the request context here because lego's Provider interface
// is context-less. The ctx is per-RequestCertificate so cancellation
// still works end to end.
type legoProvider struct {
	ctx    context.Context
	solver ca.DnsSolver
}

func (p *legoProvider) Present(domain, _ /* token */, keyAuth string) error {
	fqdn, value := dns01Record(domain, keyAuth)
	return p.solver.Present(p.ctx, fqdn, value)
}

func (p *legoProvider) CleanUp(domain, _ /* token */, keyAuth string) error {
	fqdn, value := dns01Record(domain, keyAuth)
	return p.solver.CleanUp(p.ctx, fqdn, value)
}

func (p *legoProvider) Timeout() (timeout, interval time.Duration) {
	t := p.solver.Timeout()
	if t <= 0 {
		t = 2 * time.Minute
	}
	return t, 5 * time.Second
}

// dns01Record computes the TXT record name + value per RFC 8555 §8.4.
// We do this ourselves (rather than calling lego's dns01.GetChallengeInfo)
// so the value is independent of LEGO_DISABLE_CNAME_SUPPORT env behaviour.
func dns01Record(domain, keyAuth string) (fqdn, value string) {
	sum := sha256.Sum256([]byte(keyAuth))
	value = base64.RawURLEncoding.EncodeToString(sum[:])
	fqdn = "_acme-challenge." + domain + "."
	return fqdn, value
}

// mapErr translates lego / acme errors into our sentinels.
//
// Lego wraps protocol errors in *acme.ProblemDetails (sometimes inside
// the typed *acme.RateLimitedError / *acme.NonceError envelopes). We
// match on the urn:ietf:params:acme:error:* type substring so this
// keeps working if lego adds new envelope types.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	// Already one of ours (validateRequest path).
	if isSentinel(err) {
		return err
	}

	// Typed rate-limit envelope.
	var rl *legoacme.RateLimitedError
	if errors.As(err, &rl) {
		return fmt.Errorf("%w: %s", ca.ErrCAQuotaExceeded, rl.Error())
	}
	var nonce *legoacme.NonceError
	if errors.As(err, &nonce) {
		return fmt.Errorf("%w: %s", ca.ErrNetwork, nonce.Error())
	}

	var pd *legoacme.ProblemDetails
	if errors.As(err, &pd) {
		return mapProblem(pd)
	}

	// Fallback: treat unknown as network so worker retries with
	// backoff; pure invariant violations should already have been
	// caught by validateRequest.
	return fmt.Errorf("%w: %v", ca.ErrNetwork, err)
}

// mapProblem maps an ACME ProblemDetails to a sentinel by inspecting
// the urn:ietf:params:acme:error:<kind> type tail.
func mapProblem(pd *legoacme.ProblemDetails) error {
	kind := problemKind(pd.Type)
	switch kind {
	case "rateLimited":
		return fmt.Errorf("%w: %s", ca.ErrCAQuotaExceeded, pd.Error())
	case "caa":
		return fmt.Errorf("%w: %s", ca.ErrCAATooStrict, pd.Error())
	case "unauthorized", "accountDoesNotExist":
		return fmt.Errorf("%w: %s", ca.ErrAccountInvalid, pd.Error())
	case "badNonce", "serverInternal":
		return fmt.Errorf("%w: %s", ca.ErrNetwork, pd.Error())
	case "malformed", "badCSR", "rejectedIdentifier":
		return fmt.Errorf("%w: %s", ca.ErrInvalidInput, pd.Error())
	}
	// HTTP status fallback: 5xx and timeouts are network, the rest
	// are validation failures we surface to the user.
	if pd.HTTPStatus >= 500 {
		return fmt.Errorf("%w: %s", ca.ErrNetwork, pd.Error())
	}
	return fmt.Errorf("%w: %s", ca.ErrAuthzInvalid, pd.Error())
}

func problemKind(t string) string {
	const prefix = "urn:ietf:params:acme:error:"
	if strings.HasPrefix(t, prefix) {
		return strings.TrimPrefix(t, prefix)
	}
	return t
}

func isSentinel(err error) bool {
	for _, s := range []error{
		ca.ErrCAQuotaExceeded,
		ca.ErrAuthzInvalid,
		ca.ErrCAATooStrict,
		ca.ErrAccountInvalid,
		ca.ErrNetwork,
		ca.ErrInvalidInput,
	} {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}

// Compile-time interface check.
var _ legochallenge.Provider = (*legoProvider)(nil)
var _ ca.AcmeCA = (*bpCA)(nil)
