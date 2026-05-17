package service

import (
	"context"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
)

// solverInvokingCA is a fakeCA that actually calls solver.Present with a
// well-known fqdn+value. The orchestrator's buildSolver must wire the
// per-order Coordinator into the solver, otherwise InjectReady fired
// against svc.ManualCoordinator(orderID) never reaches the goroutine
// blocking inside Present.
type solverInvokingCA struct {
	*fakeCA
	fqdn   string
	value  string
	called chan struct{}
}

func (s *solverInvokingCA) RequestCertificate(ctx context.Context, req ca.CertificateRequest) (ca.CertificateResult, error) {
	s.fakeCA.requestCalls++
	s.fakeCA.lastRequest = req
	close(s.called)
	if req.DNS == nil {
		return ca.CertificateResult{}, ca.ErrNetwork
	}
	if err := req.DNS.Present(ctx, s.fqdn, s.value); err != nil {
		return ca.CertificateResult{}, err
	}
	_ = req.DNS.CleanUp(ctx, s.fqdn, s.value)
	if s.fakeCA.requestErr != nil {
		return ca.CertificateResult{}, s.fakeCA.requestErr
	}
	return s.fakeCA.requestResult, nil
}

// TestBuildSolver_ManualMode_WiringToPerOrderCoordinator is the regression
// test for the bug discovered in W4-C smoke: buildSolver returned a
// solver bound to dnsReg().Get(KindManual)'s built-in Coordinator, while
// the HTTP handler injected ready signals on svc.ManualCoordinator(orderID).
// The two Coordinators are independent pending maps so the worker hung.
//
// The post-fix wiring builds the solver from svc.ManualCoordinator(orderID)
// directly; MarkManualChallengeReady on the same orderID must unblock
// Present.
func TestBuildSolver_ManualMode_WiringToPerOrderCoordinator(t *testing.T) {
	sans := []string{"foo.example.com"}
	leaf := makeLeafPEM(t, sans)
	const fqdn = "_acme-challenge.foo.example.com."
	const value = "manual-wire-value"

	caInvoking := &solverInvokingCA{
		fakeCA: &fakeCA{
			name: "lets-encrypt",
			requestResult: ca.CertificateResult{
				LeafPEM:   leaf,
				ChainPEM:  []byte("---chain---"),
				Serial:    "wire-1",
				NotBefore: time.Now(),
				NotAfter:  time.Now().Add(90 * 24 * time.Hour),
			},
		},
		fqdn:   fqdn,
		value:  value,
		called: make(chan struct{}),
	}
	svc, mock := newTestServiceWithCA(t, caInvoking)

	mock.ExpectQuery(`SELECT .+ FROM cert\.orders\s+WHERE id`).
		WithArgs(int64(1)).
		WillReturnRows(ordersColsRow().AddRow(orderRow(sans)...))
	mock.ExpectQuery(`SELECT .+ FROM cert\.order_events\s+WHERE order_id`).
		WithArgs(int64(1)).WillReturnRows(emptyEventsRow())

	expectAppendEvent(mock, 1, actionOrderPicked)
	expectAppendEvent(mock, 2, actionKeyGenerated)
	expectAppendEvent(mock, 3, actionCSRBuilt)
	expectAppendEvent(mock, 4, actionDNSSolverBuilt)
	expectUpdateStatus(mock, repo.OrderStatusDraft, repo.OrderStatusValidating, nil)
	expectUpdateStatus(mock, repo.OrderStatusValidating, repo.OrderStatusIssuing, nil)
	expectAppendEvent(mock, 5, actionACMERequestStarted)
	expectAppendEvent(mock, 6, actionACMERequestComplete)

	mock.ExpectQuery(`INSERT INTO cert\.certs`).
		WithArgs(int64(1), int64(42), sans, "lets-encrypt", "wire-1",
			pgxmock.AnyArg(), string(leaf), "---chain---",
			pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			string(repo.CertStatusIssued)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(7), time.Now().UTC()))
	expectAppendEvent(mock, 7, actionCertPersisted)
	mock.ExpectExec(`UPDATE cert\.orders\s+SET cert_id`).
		WithArgs(int64(7), int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	expectUpdateStatus(mock, repo.OrderStatusIssuing, repo.OrderStatusIssued, nil)
	mock.ExpectExec(`UPDATE cert\.orders\s+SET finalized_at`).
		WithArgs(pgxmock.AnyArg(), int64(1)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectQuery(`INSERT INTO cert\.renewal_jobs`).
		WithArgs(int64(7), pgxmock.AnyArg(), "queued").
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).
			AddRow(int64(3), time.Now().UTC()))
	expectAppendEvent(mock, 8, actionRenewalEnqueued)

	// Drive in a goroutine; the solver.Present call will block until we
	// fire MarkManualChallengeReady. Without the wiring fix the solver
	// is bound to a different Coordinator and the worker hangs.
	driveErr := make(chan error, 1)
	go func() {
		driveErr <- svc.DriveOrder(context.Background(), 1)
	}()

	// Wait for the fake CA to be invoked (so the solver has been built
	// and Present has had time to register).
	select {
	case <-caInvoking.called:
	case <-time.After(2 * time.Second):
		t.Fatal("CA RequestCertificate was never called within 2s")
	}

	// Poll-until-registered then inject via the public service method —
	// this is the exact code path the HTTP handler exercises.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := svc.MarkManualChallengeReady(1, fqdn, value); err == nil {
			// The wrapper returns nil whether or not the pending entry
			// exists; spin until we see the cert call return.
		}
		select {
		case err := <-driveErr:
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			require.Equal(t, 1, caInvoking.fakeCA.requestCalls)
			return
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	t.Fatal("DriveOrder never returned within 2s after MarkManualChallengeReady; solver wired to wrong Coordinator?")
}

// newTestServiceWithCA mirrors newTestService but lets the caller pass an
// arbitrary ca.AcmeCA implementation (not just *fakeCA).
func newTestServiceWithCA(t *testing.T, c ca.AcmeCA) (*Service, pgxmock.PgxPoolIface) {
	t.Helper()
	svc, pool := newTestService(t, &fakeCA{name: "lets-encrypt"})
	// Replace the router with one pointing at the supplied CA so we
	// reuse the registry + vault wiring from newTestService.
	svc.cfg.Router = NewRouter(c)
	return svc, pool
}
