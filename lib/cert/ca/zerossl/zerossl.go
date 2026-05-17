// Package zerossl implements the ca.AcmeCA interface against ZeroSSL's
// ACME endpoints using github.com/go-acme/lego/v4.
//
// ZeroSSL only accepts ACME accounts that bind to an existing portal
// account via External Account Binding (RFC 8555 §7.3.4). Operators
// register at https://app.zerossl.com → Developer → "Generate EAB
// Credentials"; the resulting Kid / HMAC key pair is provided via
// Config.EABKID / Config.EABHMACKey and forwarded on every fresh
// account registration.
//
// The adapter is stateless: every RequestCertificate call constructs a
// fresh lego client from the AccountKey supplied by the caller, mirroring
// the Let's Encrypt adapter. Account material lives in vault and is
// fetched per request, avoiding any in-process key cache that could
// outlive a vault rotation.
//
// There is no public ZeroSSL staging directory; local integration tests
// run against Pebble via TestZeroSSL_Integration_Pebble.
package zerossl

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

// zerosslDirectoryURL is the only production ACME directory ZeroSSL
// publishes. Their DV90 endpoint issues 90-day certificates and is the
// one CAB Forum-compliant tier this adapter targets.
const zerosslDirectoryURL = "https://acme.zerossl.com/v2/DV90"

// Config configures a ZeroSSL adapter instance.
type Config struct {
	// EABKID is the External Account Binding Key ID issued by ZeroSSL
	// when the operator registers at https://app.zerossl.com (Developer →
	// Generate). Required; ZeroSSL refuses anonymous ACME registration.
	EABKID string

	// EABHMACKey is the base64url-encoded HMAC key paired with EABKID.
	// Required.
	EABHMACKey string
}

// New returns an adapter implementing ca.AcmeCA backed by ZeroSSL.
//
// New does not validate that EABKID / EABHMACKey are non-empty: an
// operator may construct the adapter eagerly at boot and only learn the
// EAB credentials lazily via vault. Missing credentials surface as
// ca.ErrInvalidInput at RequestCertificate / Revoke time.
func New(cfg Config) ca.AcmeCA {
	return &zerosslCA{cfg: cfg}
}

type zerosslCA struct {
	cfg Config
}

func (c *zerosslCA) Name() string  { return "zerossl" }
func (c *zerosslCA) Tier() ca.Tier { return ca.TierFreeDV }

// SupportsWildcard: ZeroSSL issues wildcards via dns-01.
func (c *zerosslCA) SupportsWildcard() bool { return true }

// ValidityDays: ZeroSSL's DV90 endpoint issues 90-day certs.
func (c *zerosslCA) ValidityDays() int { return 90 }

// SupportedChallenges: this adapter only wires dns-01 because it's the
// one challenge that works for wildcards and for non-public origins.
func (c *zerosslCA) SupportedChallenges() []ca.ChallengeType {
	return []ca.ChallengeType{ca.ChallengeDNS01}
}

func (c *zerosslCA) directoryURL() string {
	return zerosslDirectoryURL
}

// RequestCertificate runs the full ACME flow against ZeroSSL and returns
// the issued certificate. Errors are mapped to ca sentinel errors.
func (c *zerosslCA) RequestCertificate(ctx context.Context, req ca.CertificateRequest) (ca.CertificateResult, error) {
	if err := c.validateRequest(req); err != nil {
		return ca.CertificateResult{}, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	user := &zsUser{email: req.AccountEmail, key: req.AccountKey}
	client, err := newClient(c.directoryURL(), user)
	if err != nil {
		return ca.CertificateResult{}, mapErr(err)
	}

	if err := registerAccount(client, user, c.cfg); err != nil {
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
func (c *zerosslCA) Revoke(ctx context.Context, cert []byte, reason ca.RevokeReason, accountKey crypto.Signer) error {
	if len(cert) == 0 {
		return fmt.Errorf("%w: empty certificate", ca.ErrInvalidInput)
	}
	if accountKey == nil {
		return fmt.Errorf("%w: missing account key", ca.ErrInvalidInput)
	}

	user := &zsUser{key: accountKey}
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
// network I/O. ZeroSSL adds the EAB credential check on top of the
// generic ACME validations.
func (c *zerosslCA) validateRequest(req ca.CertificateRequest) error {
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
	if c.cfg.EABKID == "" || c.cfg.EABHMACKey == "" {
		return fmt.Errorf("%w: ZeroSSL EAB credentials required (EABKID + EABHMACKey)", ca.ErrInvalidInput)
	}
	return nil
}

// newClient wraps lego.NewClient with our directory URL.
func newClient(dirURL string, user *zsUser) (*lego.Client, error) {
	cfg := lego.NewConfig(user)
	cfg.CADirURL = dirURL
	cfg.UserAgent = "idcd-cert/1.0"
	return lego.NewClient(cfg)
}

// registerAccount registers the account if it has no registration yet,
// using ZeroSSL-mandated External Account Binding for the first
// registration. Returning either path leaves user.registration populated.
func registerAccount(client *lego.Client, user *zsUser, cfg Config) error {
	if user.registration != nil {
		return nil
	}
	// Try lookup first; succeeds for already-registered keys without
	// re-running EAB (ZeroSSL ties EAB to the initial registration only).
	if reg, err := client.Registration.ResolveAccountByKey(); err == nil {
		user.registration = reg
		return nil
	}
	reg, err := client.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
		TermsOfServiceAgreed: true,
		Kid:                  cfg.EABKID,
		HmacEncoded:          cfg.EABHMACKey,
	})
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

// zsUser adapts crypto.Signer + email into lego's registration.User.
type zsUser struct {
	email        string
	key          crypto.Signer
	registration *registration.Resource
}

func (u *zsUser) GetEmail() string                        { return u.email }
func (u *zsUser) GetRegistration() *registration.Resource { return u.registration }
func (u *zsUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

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
// the urn:ietf:params:acme:error:<kind> type tail. ZeroSSL surfaces
// the standard RFC 8555 problem types plus externalAccountRequired
// (when an account tries to register without EAB credentials).
func mapProblem(pd *legoacme.ProblemDetails) error {
	kind := problemKind(pd.Type)
	switch kind {
	case "rateLimited":
		return fmt.Errorf("%w: %s", ca.ErrCAQuotaExceeded, pd.Error())
	case "caa":
		return fmt.Errorf("%w: %s", ca.ErrCAATooStrict, pd.Error())
	case "unauthorized", "accountDoesNotExist", "externalAccountRequired":
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
var _ ca.AcmeCA = (*zerosslCA)(nil)
