// Package tsa is the RFC3161 Time-Stamp Authority client used by the
// Evidence / Attestation subsystem (S2 — see docs/prd/18-evidence-and-attestation.md).
//
// One Provider implementation per TSA vendor (DigiCert, GlobalSign, NTSC),
// each in its own subpackage. The Multi wrapper performs ordered fall-over
// across providers: primary first, then backups, switching on transient
// upstream failures (network / 5xx / malformed) and aborting fast on
// configuration / input bugs (4xx / digest length mismatch).
//
// All implementations MUST be stateless — every Stamp call issues an
// independent HTTP POST and never caches connections, tokens, or
// certificates. This keeps recovery semantics simple for the Verdict
// pipeline (D4 WAL) and matches the public "no auth" model of free
// RFC3161 services we rely on in S2.
package tsa

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Sentinel errors. Every Provider implementation MUST wrap one of these
// using fmt.Errorf("%w: ...", ErrXxx). Multi inspects the wrapping with
// errors.Is to decide whether to fall over to the next provider.
var (
	// ErrUpstreamUnavailable is returned when the TSA endpoint cannot be
	// reached or replied with a 5xx / timed out. Multi falls over to the
	// next provider in line.
	ErrUpstreamUnavailable = errors.New("tsa: upstream unreachable / 5xx / timeout")

	// ErrInvalidResponse is returned when the TSA replied with a body
	// that cannot be parsed as an RFC3161 response or whose PKIStatus
	// is not "granted" / "granted with mods". Multi falls over.
	ErrInvalidResponse = errors.New("tsa: response parse failed / status != granted")

	// ErrAuthFailed is returned when the TSA replied with 401 / 403 /
	// similar (bad token, expired client cert). Multi aborts — this is
	// almost always a configuration bug that the other providers will
	// not magically fix.
	ErrAuthFailed = errors.New("tsa: authentication failed (token / cert)")

	// ErrInvalidInput is returned when the caller passed a digest whose
	// length does not match the declared hash algorithm or used an
	// unsupported hash. Multi aborts — same call against the next
	// provider would fail identically.
	ErrInvalidInput = errors.New("tsa: invalid digest length or hash alg")
)

// Provider is a single TSA vendor. Implementations live in subpackages.
type Provider interface {
	// Name returns a stable lowercase identifier used as the
	// attestation_record.external_id prefix and the
	// tsa_response.provider column. Examples: "digicert",
	// "globalsign", "ntsc".
	Name() string

	// Stamp submits digest to the TSA and returns the resulting
	// RFC3161 TimeStampToken.
	//
	//   - hashAlg is the algorithm used to produce digest (recommend
	//     crypto.SHA256).
	//   - digest is the hash of the original PDF bytes — NOT the PDF
	//     itself. Length must match hashAlg.Size().
	//   - token is the full ASN.1 DER encoded TimeStampToken,
	//     embeddable directly in a PAdES B-T signature.
	//   - issuedAt is the TSA's claimed signing time.
	//
	// ctx controls the whole call. Implementations SHOULD honour a
	// 10s timeout if ctx has no deadline; TSA round-trips are
	// typically 1-2s.
	Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) (token []byte, issuedAt time.Time, err error)
}

// DefaultPerCallTimeout is the per-provider deadline applied by Multi
// when the caller's context has none. RFC3161 TSAs normally answer in
// 1-2 seconds; 10s leaves plenty of headroom while bounding the worst
// case across N fall-over attempts.
const DefaultPerCallTimeout = 10 * time.Second

// Multi wraps an ordered list of Providers and performs fall-over. The
// first provider is the primary; the rest are tried in order on
// transient failure.
//
// Multi is the type the Verdict pipeline depends on; the actual
// providers are injected at process start.
type Multi struct {
	providers []Provider

	// PerCallTimeout is the hard deadline applied to each individual
	// provider's Stamp call. Defaults to DefaultPerCallTimeout. Set to
	// a non-positive value to inherit ctx unchanged.
	PerCallTimeout time.Duration

	// Logger is optional; when non-nil, fall-over events and
	// per-attempt failures are logged at WARN. nil-safe.
	Logger *slog.Logger
}

// NewMulti constructs a Multi over the given providers in order. The
// first argument is the primary; subsequent arguments are backups tried
// only when the previous one returns a transient error.
func NewMulti(providers ...Provider) *Multi {
	cp := make([]Provider, len(providers))
	copy(cp, providers)
	return &Multi{
		providers:      cp,
		PerCallTimeout: DefaultPerCallTimeout,
	}
}

// Names returns the provider names in fall-over order. Useful for
// /healthz output and ops dashboards. The returned slice is a copy and
// safe to retain / mutate.
func (m *Multi) Names() []string {
	out := make([]string, len(m.providers))
	for i, p := range m.providers {
		out[i] = p.Name()
	}
	return out
}

// Stamp tries each provider in order. It returns on the first success,
// or after all providers have failed. The returned providerName tells
// the caller which provider produced the token — record this in
// tsa_response.provider.
//
// Behaviour on errors:
//
//   - errors.Is(err, ErrUpstreamUnavailable) → fall over to next
//   - errors.Is(err, ErrInvalidResponse)     → fall over to next
//   - errors.Is(err, ErrAuthFailed)          → abort, return as-is
//   - errors.Is(err, ErrInvalidInput)        → abort, return as-is
//   - any other error                        → fall over to next (be
//     liberal with unknown failures from third-party adapters)
//
// When all providers fail, the last transient error is returned
// (already wrapping ErrUpstreamUnavailable or ErrInvalidResponse).
// When there are no providers, ErrUpstreamUnavailable is returned
// immediately.
func (m *Multi) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) (token []byte, issuedAt time.Time, providerName string, err error) {
	if len(m.providers) == 0 {
		return nil, time.Time{}, "", fmt.Errorf("%w: no providers configured", ErrUpstreamUnavailable)
	}

	var lastErr error
	for i, p := range m.providers {
		callCtx := ctx
		var cancel context.CancelFunc
		if m.PerCallTimeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, m.PerCallTimeout)
		}

		tok, ts, err := p.Stamp(callCtx, hashAlg, digest)
		if cancel != nil {
			cancel()
		}
		if err == nil {
			return tok, ts, p.Name(), nil
		}

		// Fatal errors abort immediately — backups will fail the same way.
		if errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrInvalidInput) {
			m.log("tsa: abort fall-over on fatal error",
				slog.String("provider", p.Name()),
				slog.Int("attempt", i+1),
				slog.String("err", err.Error()),
			)
			return nil, time.Time{}, p.Name(), err
		}

		// Transient: log and try next.
		m.log("tsa: provider failed, falling over",
			slog.String("provider", p.Name()),
			slog.Int("attempt", i+1),
			slog.Int("remaining", len(m.providers)-i-1),
			slog.String("err", err.Error()),
		)
		lastErr = err
	}

	// All providers exhausted. lastErr is guaranteed non-nil here
	// because len(providers) > 0 and we did not return early.
	if !errors.Is(lastErr, ErrUpstreamUnavailable) && !errors.Is(lastErr, ErrInvalidResponse) {
		// Wrap unknown errors so callers can use errors.Is for retry decisions.
		lastErr = fmt.Errorf("%w: %v", ErrUpstreamUnavailable, lastErr)
	}
	return nil, time.Time{}, "", lastErr
}

func (m *Multi) log(msg string, attrs ...slog.Attr) {
	if m.Logger == nil {
		return
	}
	m.Logger.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
}

// ValidateDigest is a small helper exposed for adapter implementations:
// it returns ErrInvalidInput if digest does not match hashAlg.Size() or
// if hashAlg is unavailable in this build. Adapters SHOULD call this
// before issuing an HTTP request.
func ValidateDigest(hashAlg crypto.Hash, digest []byte) error {
	if !hashAlg.Available() {
		return fmt.Errorf("%w: hash %d unavailable", ErrInvalidInput, hashAlg)
	}
	if len(digest) != hashAlg.Size() {
		return fmt.Errorf("%w: hash %d expects %d bytes, got %d",
			ErrInvalidInput, hashAlg, hashAlg.Size(), len(digest))
	}
	return nil
}
