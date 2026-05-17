package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/metrics"
	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// Defaults for RenewalScheduler. Per PRD §7.3 we attempt renewal once an
// hour and target certs whose not_after is within 30 days of "now".
const (
	DefaultRenewalInterval = time.Hour
	DefaultRenewalLead     = 30 * 24 * time.Hour
	defaultRenewalLimit    = 100
)

// OrderEnqueuer is the minimal "schedule one order onto the work stream"
// surface the renewer depends on. *Service satisfies this via
// EnqueueOrder, but a fake in renewal_test.go does too — the renewer must
// be testable without standing up Redis.
type OrderEnqueuer interface {
	EnqueueOrder(ctx context.Context, orderID int64) error
}

// RenewalScheduler periodically scans cert.certs for certs that will
// expire within LeadTime and enqueues a renewal order + renewal_job row
// for each. It runs in its own process (cmd/renewer) so it does not
// share the worker's resources, but it pushes work onto the same
// Redis Stream the worker drains.
type RenewalScheduler struct {
	repos     *repo.Repos
	enqueuer  OrderEnqueuer
	interval  time.Duration
	leadTime  time.Duration
	scanLimit int
	logger    *slog.Logger
	now       func() time.Time
}

// RenewalOption tunes a RenewalScheduler at construction time. Zero
// values cause the scheduler to fall back to package defaults.
type RenewalOption func(*RenewalScheduler)

// WithRenewalInterval overrides the scan tick interval.
func WithRenewalInterval(d time.Duration) RenewalOption {
	return func(r *RenewalScheduler) {
		if d > 0 {
			r.interval = d
		}
	}
}

// WithRenewalLead overrides how far ahead of not_after the scheduler
// starts scheduling renewals.
func WithRenewalLead(d time.Duration) RenewalOption {
	return func(r *RenewalScheduler) {
		if d > 0 {
			r.leadTime = d
		}
	}
}

// WithRenewalScanLimit overrides the max number of expiring certs scanned
// per tick.
func WithRenewalScanLimit(n int) RenewalOption {
	return func(r *RenewalScheduler) {
		if n > 0 {
			r.scanLimit = n
		}
	}
}

// WithRenewalLogger swaps the structured logger.
func WithRenewalLogger(l *slog.Logger) RenewalOption {
	return func(r *RenewalScheduler) {
		if l != nil {
			r.logger = l
		}
	}
}

// withRenewalNow is a test-only seam letting unit tests pin "now". Not
// exported — production paths use time.Now.
func withRenewalNow(fn func() time.Time) RenewalOption {
	return func(r *RenewalScheduler) {
		if fn != nil {
			r.now = fn
		}
	}
}

// NewRenewalScheduler wires a RenewalScheduler. repos and enqueuer are
// required; opts override the defaults.
func NewRenewalScheduler(repos *repo.Repos, enqueuer OrderEnqueuer, opts ...RenewalOption) *RenewalScheduler {
	r := &RenewalScheduler{
		repos:     repos,
		enqueuer:  enqueuer,
		interval:  DefaultRenewalInterval,
		leadTime:  DefaultRenewalLead,
		scanLimit: defaultRenewalLimit,
		logger:    slog.Default(),
		now:       time.Now,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Run kicks off one immediate scan then ticks at Interval until ctx is
// cancelled. It never returns a non-nil error in the normal path — the
// shutdown signal is ctx.Done. Run swallows per-tick errors so a single
// flaky DB call does not stop the scheduler.
func (r *RenewalScheduler) Run(ctx context.Context) error {
	if r.repos == nil || r.enqueuer == nil {
		return fmt.Errorf("renewal scheduler: not configured")
	}

	r.logger.Info("renewal scheduler starting",
		"interval", r.interval.String(),
		"lead", r.leadTime.String())

	if err := r.scanAndEnqueue(ctx); err != nil {
		r.logger.Warn("renewal scan failed", "err", err)
	}

	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("renewal scheduler stopping")
			return nil
		case <-t.C:
			if err := r.scanAndEnqueue(ctx); err != nil {
				r.logger.Warn("renewal scan failed", "err", err)
			}
		}
	}
}

// scanAndEnqueue runs one cycle of the scheduler. It selects expiring
// certs, filters out those that already have an active renewal_job, then
// inserts a fresh order + renewal_job and enqueues the new order onto the
// worker stream.
//
// Skip-set construction: ListQueued is used to find renewal_jobs already
// queued / in flight. (status='succeeded' is fine — that means a renewal
// completed; the new cert has its own row.) We never re-enqueue a cert
// whose latest queued job is still pending. The repo layer does not
// expose ExistsActiveJobForCert(), and per task constraints we must not
// add new repo methods — so we filter in-memory.
func (r *RenewalScheduler) scanAndEnqueue(ctx context.Context) error {
	now := r.now()
	cutoff := now.Add(r.leadTime)

	expiring, err := r.repos.Certs.ListExpiringBefore(ctx, cutoff, r.scanLimit)
	if err != nil {
		return fmt.Errorf("list expiring: %w", err)
	}
	if len(expiring) == 0 {
		r.logger.Debug("renewal scan: no expiring certs", "cutoff", cutoff)
		return nil
	}

	queued, err := r.repos.RenewalJobs.ListQueued(ctx, r.scanLimit*4)
	if err != nil {
		return fmt.Errorf("list queued jobs: %w", err)
	}
	skip := make(map[int64]struct{}, len(queued))
	for _, j := range queued {
		skip[j.CertID] = struct{}{}
	}

	for _, c := range expiring {
		if _, ok := skip[c.ID]; ok {
			r.logger.Debug("renewal scan: skipping (active job exists)",
				"cert_id", c.ID)
			continue
		}
		if err := r.enqueueOne(ctx, c); err != nil {
			r.logger.Warn("renewal enqueue failed",
				"cert_id", c.ID, "err", err)
			// Continue with the next cert — a single failure must not
			// stop the batch.
			continue
		}
	}
	return nil
}

// enqueueOne copies one cert's order template into a new draft order,
// inserts a renewal_jobs row pointing at it, and pushes the order id onto
// the worker stream. Best-effort cleanup: if EnqueueOrder fails we still
// keep the rows so the worker can pick them up later via a retry path.
func (r *RenewalScheduler) enqueueOne(ctx context.Context, cert *repo.Cert) error {
	oldOrder, err := r.repos.Orders.GetByID(ctx, cert.OrderID)
	if err != nil {
		return fmt.Errorf("load source order %d: %w", cert.OrderID, err)
	}

	newOrder := &repo.Order{
		AccountID:        oldOrder.AccountID,
		SANs:             append([]string(nil), oldOrder.SANs...),
		SANsUnicode:      append([]string(nil), oldOrder.SANsUnicode...),
		CommonName:       oldOrder.CommonName,
		Tier:             oldOrder.Tier,
		CA:               oldOrder.CA,
		ResellerChannel:  oldOrder.ResellerChannel,
		ResellerOrderRef: oldOrder.ResellerOrderRef,
		OrganizationID:   oldOrder.OrganizationID,
		ValidityDays:     oldOrder.ValidityDays,
		ChallengeType:    oldOrder.ChallengeType,
		DNSCredentialID:  oldOrder.DNSCredentialID,
		Status:           repo.OrderStatusDraft,
		// IdempotencyKey deliberately nil: the renewer is the only
		// producer here and we want each tick to be its own attempt.
	}

	newOrderID, err := r.repos.Orders.Insert(ctx, newOrder)
	if err != nil {
		return fmt.Errorf("insert new order: %w", err)
	}

	job := &repo.RenewalJob{
		CertID:      cert.ID,
		ScheduledAt: r.now(),
		Status:      "queued",
		NewOrderID:  &newOrderID,
	}
	if _, err := r.repos.RenewalJobs.Insert(ctx, job); err != nil {
		return fmt.Errorf("insert renewal job: %w", err)
	}
	// Once the job is persisted, attach new_order_id explicitly — Insert
	// does not write it (the column defaults NULL on create).
	if err := r.repos.RenewalJobs.UpdateStatus(ctx, job.ID, "queued", nil, &newOrderID); err != nil {
		return fmt.Errorf("attach new_order_id: %w", err)
	}

	if err := r.enqueuer.EnqueueOrder(ctx, newOrderID); err != nil {
		// Do not roll back — the worker can also pick up draft orders
		// via the periodic poll, and the renewer will retry next tick.
		return fmt.Errorf("enqueue order %d: %w", newOrderID, err)
	}

	metrics.RecordRenewalJob("scheduled")
	r.logger.Info("renewal enqueued",
		"cert_id", cert.ID,
		"new_order_id", newOrderID,
		"not_after", cert.NotAfter)
	return nil
}

