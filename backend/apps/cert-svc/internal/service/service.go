// Package service hosts the cert-svc ACME orchestrator. The state machine,
// Redis-Stream consumer and CA router live here; HTTP handlers in
// apps/cert-svc/internal/handler call into Service for write-paths.
//
// Per CLAUDE.md D1 / D2 the service layer joins cert.* with account.*
// only in application code. Per PRD §6 every state transition is written
// to cert.order_events as a WAL entry first; crash recovery replays.
package service

import (
	"context"
	"crypto"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/vault"
)

// Errors surfaced by the service package. Worker / HTTP handler branch on
// these via errors.Is.
var (
	// ErrOrderNotPickable is returned when DriveOrder is called for an
	// order whose status is terminal (issued / revoked) — nothing to do.
	ErrOrderNotPickable = errors.New("service: order not pickable")
	// ErrMissingDNSCredential is returned when an order needs a credential
	// but none is attached (manual mode is a different code path).
	ErrMissingDNSCredential = errors.New("service: order has no dns credential")
	// ErrManualCoordinator is returned when a manual-mode caller hits an
	// orderID we have no live Coordinator for (worker restarted, etc).
	ErrManualCoordinator = errors.New("service: no manual coordinator for order")
)

// Config bundles everything Service needs at construction time. Every
// field is required.
type Config struct {
	Repos        *repo.Repos
	Redis        redis.UniversalClient
	Vault        vault.Vault
	DNSReg       *dns.Registry
	Router       *Router
	AccountKey   crypto.Signer // long-lived ACME account key
	AccountEmail string

	// Stream / Group override the Redis Stream / consumer-group names.
	// Empty falls back to the package defaults.
	Stream       string
	Group        string
	ConsumerName string

	// BlockTimeout is how long XREADGROUP blocks per poll. Defaults to 5s.
	BlockTimeout time.Duration

	// ManualPollInterval / ManualTimeout configure freshly-created manual
	// Coordinators. Zero values fall back to manual package defaults.
	ManualPollInterval time.Duration
	ManualTimeout      time.Duration

	// CARequestTimeout caps a single AcmeCA.RequestCertificate call.
	CARequestTimeout time.Duration

	// DownloadSecret is the HMAC-SHA256 key used to sign one-shot
	// download tokens (W5). Empty means downloads are disabled —
	// Service.Downloads will be nil and the handler must surface a
	// 503 rather than mint forgeable tokens.
	DownloadSecret []byte
	// DownloadTTL overrides the default 5-minute lifetime.
	DownloadTTL time.Duration

	// Abuse is the rate / reputation gate run in front of order
	// creation (W5). When nil the handler skips abuse checks
	// entirely; production wiring in cmd/server always sets it.
	Abuse *AbuseDetector

	Logger *slog.Logger
}

// Default stream and consumer-group names.
const (
	DefaultStream       = "cert:order_events"
	DefaultGroup        = "cert-worker"
	DefaultBlockTimeout = 5 * time.Second
	DefaultCATimeout    = 5 * time.Minute
)

// Service is the orchestrator. Construct once at startup; concurrent
// callers safe.
type Service struct {
	cfg Config

	mu                 sync.Mutex
	manualCoordinators map[int64]*manual.Coordinator

	// Downloads mints / consumes the one-shot tokens behind
	// /v1/cert/dl/{token} (W5). Optional in tests; the server main
	// always wires it.
	Downloads *DownloadTokenManager

	// Abuse is the rate / reputation gate; mirrors cfg.Abuse so the
	// handler can dereference Service.Abuse rather than reaching
	// into Config.
	Abuse *AbuseDetector
}

// New returns a Service with defaults filled in. It does NOT touch
// Redis / DB — callers may construct without a live backend (tests).
func New(cfg Config) *Service {
	if cfg.Stream == "" {
		cfg.Stream = DefaultStream
	}
	if cfg.Group == "" {
		cfg.Group = DefaultGroup
	}
	if cfg.BlockTimeout <= 0 {
		cfg.BlockTimeout = DefaultBlockTimeout
	}
	if cfg.CARequestTimeout <= 0 {
		cfg.CARequestTimeout = DefaultCATimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Service{
		cfg:                cfg,
		manualCoordinators: map[int64]*manual.Coordinator{},
	}
	if len(cfg.DownloadSecret) > 0 && cfg.Redis != nil {
		var opts []DownloadOption
		if cfg.DownloadTTL > 0 {
			opts = append(opts, WithDownloadTTL(cfg.DownloadTTL))
		}
		s.Downloads = NewDownloadTokenManager(cfg.Redis, cfg.DownloadSecret, opts...)
	}
	s.Abuse = cfg.Abuse
	return s
}

// ManualCoordinator returns the live Coordinator for an order, creating it
// if absent. The caller (HTTP handler / worker) uses this to inject a
// challenge-ready signal when the user confirms a TXT record.
func (s *Service) ManualCoordinator(orderID int64) *manual.Coordinator {
	s.mu.Lock()
	defer s.mu.Unlock()
	if co, ok := s.manualCoordinators[orderID]; ok {
		return co
	}
	co := manual.NewCoordinator(manual.Config{
		Timeout:      s.cfg.ManualTimeout,
		PollInterval: s.cfg.ManualPollInterval,
	})
	s.manualCoordinators[orderID] = co
	return co
}

// MarkManualChallengeReady signals the Coordinator for orderID that a
// user-installed TXT record is now visible. Returns ErrManualCoordinator
// if no Coordinator exists for the order (worker restarted mid-flow).
func (s *Service) MarkManualChallengeReady(orderID int64, fqdn, value string) error {
	s.mu.Lock()
	co, ok := s.manualCoordinators[orderID]
	s.mu.Unlock()
	if !ok {
		return ErrManualCoordinator
	}
	co.InjectReady(fqdn, value)
	return nil
}

// dropManualCoordinator clears the coordinator once an order reaches a
// terminal state.
func (s *Service) dropManualCoordinator(orderID int64) {
	s.mu.Lock()
	delete(s.manualCoordinators, orderID)
	s.mu.Unlock()
}

// Account / vault accessors used by orchestrator (kept on Service so tests
// can substitute via Config).
func (s *Service) accountKey() crypto.Signer { return s.cfg.AccountKey }
func (s *Service) accountEmail() string      { return s.cfg.AccountEmail }
func (s *Service) vaultV() vault.Vault       { return s.cfg.Vault }
func (s *Service) repos() *repo.Repos        { return s.cfg.Repos }
func (s *Service) router() *Router           { return s.cfg.Router }
func (s *Service) dnsReg() *dns.Registry     { return s.cfg.DNSReg }

// caRequestTimeout returns the timeout to pass into RequestCertificate.
func (s *Service) caRequestTimeout() time.Duration { return s.cfg.CARequestTimeout }

// caPick returns the AcmeCA for a given order. S2 introduces multi-CA
// dispatch: the Router selects by order.CA, falling back to the registered
// default when the field is empty. S3 will additionally branch on
// order.Tier / reseller_channel.
func (s *Service) caPick(_ context.Context, order *repo.Order) (ca.AcmeCA, error) {
	return s.router().Pick(order)
}
