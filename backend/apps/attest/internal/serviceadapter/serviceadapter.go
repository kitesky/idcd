// Package serviceadapter bridges the rich row projections returned by
// apps/attest/internal/repo to the trimmed consumer-defined interfaces
// declared in apps/attest/internal/service.
//
// The service package intentionally narrows its dependencies to the
// fields the orchestrator actually consumes (see iface.go); this keeps
// the orchestrator's unit tests free of database schema knowledge and
// lets the repo evolve independently. The adapters here perform the
// straightforward struct-to-struct copy plus two semantic translations:
//
//   - SetDelivered / SetFailed live on the orchestrator's OrderRepo
//     interface but the underlying *repo.VerdictOrdersRepo only exposes
//     the generic UpdateStatus helper. The adapter routes both calls
//     through UpdateStatus with the matching idcd_attest.verdict_order
//     status enums (see DECISIONS.md §M and STATE-MACHINES.md).
//
//   - ReportRepo.GetByOrderID is documented to return (nil, nil) on
//     missing rows. The underlying repo follows the Go convention of
//     returning a sentinel ErrNotFound. The adapter translates that one
//     case so the orchestrator's resume logic Just Works.
package serviceadapter

import (
	"context"
	"errors"
	"time"

	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/apps/attest/internal/service"
)

// ----- Orders adapter ------------------------------------------------

// ordersAdapter satisfies service.OrderRepo by delegating to a
// *repo.VerdictOrdersRepo.
type ordersAdapter struct {
	r *repo.VerdictOrdersRepo
}

// WrapOrders returns a service.OrderRepo backed by the given DB repo.
// Panics if r is nil — wiring bugs should fail at process start, not
// when the first verdict job runs.
func WrapOrders(r *repo.VerdictOrdersRepo) service.OrderRepo {
	if r == nil {
		panic("serviceadapter: WrapOrders requires non-nil repo")
	}
	return &ordersAdapter{r: r}
}

// GetByID maps the repo's Order projection onto the trimmed
// service.Order view. The repo struct carries refund / payment / audit
// columns the orchestrator does not consume — we deliberately drop
// them so the orchestrator stays decoupled from the wider verdict_order
// schema.
func (a *ordersAdapter) GetByID(ctx context.Context, id string) (*service.Order, error) {
	o, err := a.r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &service.Order{
		ID:              o.ID,
		OwnerID:         o.OwnerID,
		Template:        o.Template,
		Target:          o.Target,
		TimeWindowStart: o.TimeWindowStart,
		TimeWindowEnd:   o.TimeWindowEnd,
		Status:          o.Status,
		PriceCNY:        o.PriceCNY,
	}, nil
}

// UpdateStatus is a pass-through; the repo already enforces the
// optimistic "from" check and surfaces ErrInvalidStatus when the row's
// current status no longer matches.
func (a *ordersAdapter) UpdateStatus(ctx context.Context, id, from, to string, errReason *string) error {
	return a.r.UpdateStatus(ctx, id, from, to, errReason)
}

// SetDelivered routes through UpdateStatus with from="generating",
// to="delivered". The orchestrator only calls this on the happy path
// (after the order has already transitioned paid→generating in step 0),
// so a non-matching current status (e.g. another worker raced ahead and
// also marked delivered) surfaces as repo.ErrInvalidStatus — which the
// orchestrator already logs and treats as non-fatal because the
// verdict is durable in WORM regardless.
//
// The delivered_at timestamp is intentionally ignored: the repo's
// verdict_order schema fills delivered_at via a follow-up trigger /
// reconciler. A future iteration can extend VerdictOrdersRepo with a
// dedicated SetDelivered that stamps the column atomically.
func (a *ordersAdapter) SetDelivered(ctx context.Context, id string, _ time.Time) error {
	return a.r.UpdateStatus(ctx, id, repo.OrderStatusGenerating, repo.OrderStatusDelivered, nil)
}

// SetFailed routes through UpdateStatus with from="generating",
// to="failed". The reason is forwarded via the optimistic-locked update
// so refund_last_error captures the failure step for the D5 retry
// queue. The orchestrator only calls this from failPipeline, which
// always runs after the order entered the generating state.
func (a *ordersAdapter) SetFailed(ctx context.Context, id string, _ time.Time, reason string) error {
	r := reason
	return a.r.UpdateStatus(ctx, id, repo.OrderStatusGenerating, repo.OrderStatusFailed, &r)
}

// ----- Reports adapter -----------------------------------------------

// reportsAdapter satisfies service.ReportRepo by delegating to a
// *repo.VerdictReportsRepo.
type reportsAdapter struct {
	r *repo.VerdictReportsRepo
}

// WrapReports returns a service.ReportRepo backed by the given DB
// repo. Panics if r is nil for the same reason as WrapOrders.
func WrapReports(r *repo.VerdictReportsRepo) service.ReportRepo {
	if r == nil {
		panic("serviceadapter: WrapReports requires non-nil repo")
	}
	return &reportsAdapter{r: r}
}

// Insert copies the trimmed service.Report into the rich repo.Report
// shape and writes it. Returns the repo-assigned ID (which equals
// r.ID, since the orchestrator generates the vr_* prefix locally).
func (a *reportsAdapter) Insert(ctx context.Context, r *service.Report) (string, error) {
	rep := serviceReportToRepoReport(r)
	id, err := a.r.Insert(ctx, rep)
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetByOrderID translates repo.ErrNotFound to (nil, nil) per the
// service.ReportRepo contract. All other errors propagate unchanged.
// On success the rich repo.Report is collapsed to the trimmed
// service.Report.
func (a *reportsAdapter) GetByOrderID(ctx context.Context, orderID string) (*service.Report, error) {
	rep, err := a.r.GetByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return repoReportToServiceReport(rep), nil
}

// serviceReportToRepoReport copies the orchestrator's trimmed view
// into the full repo row shape. Fields not populated by the
// orchestrator (PDFSizeBytes, TSAResponseBlob, etc.) are left at their
// zero values; the DB DDL allows NULL for the nullable columns and the
// orchestrator-controlled columns are all required.
func serviceReportToRepoReport(s *service.Report) *repo.Report {
	consistency := s.NodeConsistencyPct
	return &repo.Report{
		ID:                  s.ID,
		OrderID:             s.OrderID,
		PDFURL:              s.PDFURL,
		ContentHash:         s.ContentHash,
		Signature:           s.Signature,
		SignatureKeyID:      s.SignatureKeyID,
		SignatureKeyVersion: s.SignatureKeyVersion,
		TSAProvider:         s.TSAProvider,
		TSATime:             s.TSATime,
		NodesUsed:           s.NodesUsed,
		NodeConsistencyPct:  &consistency,
		ReportType:          s.ReportType,
		ArchivedURL:         strPtrIfNonEmpty(s.ArchivedURL),
		CreatedAt:           s.CreatedAt,
	}
}

// repoReportToServiceReport is the inverse projection used on resume.
func repoReportToServiceReport(r *repo.Report) *service.Report {
	out := &service.Report{
		ID:                  r.ID,
		OrderID:             r.OrderID,
		PDFURL:              r.PDFURL,
		ContentHash:         r.ContentHash,
		Signature:           r.Signature,
		SignatureKeyID:      r.SignatureKeyID,
		SignatureKeyVersion: r.SignatureKeyVersion,
		TSAProvider:         r.TSAProvider,
		TSATime:             r.TSATime,
		NodesUsed:           r.NodesUsed,
		ReportType:          r.ReportType,
		CreatedAt:           r.CreatedAt,
	}
	if r.NodeConsistencyPct != nil {
		out.NodeConsistencyPct = *r.NodeConsistencyPct
	}
	if r.ArchivedURL != nil {
		out.ArchivedURL = *r.ArchivedURL
	}
	if r.SelfVerifyStatus != nil {
		out.SelfVerifyStatus = *r.SelfVerifyStatus
	}
	return out
}

func strPtrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
