package service

import (
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"time"

	attestrec "github.com/kite365/idcd/lib/attest/record"
	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/tsa"
)

// Config bundles every external dependency the orchestrator needs. The
// zero value is not usable; callers must populate at least Orders,
// Reports, AttestationRecords, Signer, TSA, SignerCert, TSAEndpoint and
// Archiver. New() fills in safe defaults for timeouts and Logger.
type Config struct {
	// Persistence
	Orders             OrderRepo
	Reports            ReportRepo
	AttestationRecords attestrec.Repository

	// Observations is the cross-schema (idcd_main.monitor_check) read
	// pool used by step 1 of the pipeline. Required when GenerateVerdict
	// is exercised; tests that stop before step 1 may leave it nil.
	// Construct via service.NewObservationPoolFromEnv(ctx) or pass a
	// custom ObservationPool for integration tests.
	//
	// Replaces the old package-level singleton (observationPoolOnce /
	// observationPoolMu) which made parallel testing fragile and made
	// graceful shutdown hard.
	Observations ObservationPool

	// External adapters
	Signer sign.Signer
	// TSA is a single-provider view. Wrap *tsa.Multi behind a small
	// adapter at process start if fall-over is required (Multi.Stamp
	// returns the chosen providerName as an extra value and therefore
	// does not directly satisfy tsa.Provider).
	TSA        tsa.Provider
	SignerCert *x509.Certificate
	CertChain  []*x509.Certificate

	// TSAEndpoint is the URL pdfsign re-fetches during step 8 to embed
	// the RFC3161 token into the PDF. Step 7 fetches its OWN token over
	// the raw PDF digest for D4 WAL audit; step 8's token is over the
	// inner CMS EncryptedDigest. The two tokens cover different message
	// imprints by construction, so pdfsign cannot reuse step 7's blob.
	// The duplicate-fetch cost is exposed via TSADuplicateFetchTotal()
	// so ops can size TSA quotas correctly.
	TSAEndpoint string

	Archiver Archiver

	// Tuning
	SignTimeout time.Duration // default 30s
	TSATimeout  time.Duration // default 10s

	Logger *slog.Logger
}

// Default per-step timeouts. Match the SLA budgets in
// docs/prd/14-tech-architecture.md §SLA — KMS sign normally completes in
// <2s, TSA in <2s; the timeouts are headroom, not target.
const (
	defaultSignTimeout = 30 * time.Second
	defaultTSATimeout  = 10 * time.Second
)

// Service is the orchestrator. Methods on *Service are safe for
// concurrent use as long as the injected repos / adapters are.
type Service struct {
	cfg Config
}

// New returns a Service ready to run GenerateVerdict. It panics on the
// few clearly-required fields (Orders / Reports / AttestationRecords /
// Signer / TSA / SignerCert / Archiver) so misconfigured callers fail
// fast at process start rather than mid-pipeline.
func New(cfg Config) *Service {
	if cfg.Orders == nil {
		panic("attest/service: Orders repo is required")
	}
	if cfg.Reports == nil {
		panic("attest/service: Reports repo is required")
	}
	if cfg.AttestationRecords == nil {
		panic("attest/service: AttestationRecords repo is required")
	}
	if cfg.Signer == nil {
		panic("attest/service: Signer is required")
	}
	if cfg.TSA == nil {
		panic("attest/service: TSA provider is required")
	}
	if cfg.SignerCert == nil {
		panic("attest/service: SignerCert is required")
	}
	if cfg.Archiver == nil {
		panic("attest/service: Archiver is required")
	}
	if cfg.SignTimeout <= 0 {
		cfg.SignTimeout = defaultSignTimeout
	}
	if cfg.TSATimeout <= 0 {
		cfg.TSATimeout = defaultTSATimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{cfg: cfg}
}

// ErrUnexpectedOrderStatus is returned by GenerateVerdict when the
// order's current status is not eligible for verdict generation (must be
// "paid" — first run — or "generating" — replay after crash).
var ErrUnexpectedOrderStatus = errors.New("attest/service: order not eligible for verdict generation")
