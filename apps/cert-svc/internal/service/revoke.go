package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
)

// Sentinel errors surfaced by RevokeCert. The HTTP handler branches on
// these via errors.Is to map them to canonical CERT_* error codes.
var (
	// ErrForbidden is returned when the caller's account does not own the
	// target certificate.
	ErrForbidden = errors.New("service: cert not owned by account")

	// ErrInvalidStatus is returned when the certificate is not in the
	// expected status for the requested transition. Reused for both the
	// initial issued→revoking guard and idempotent retries.
	ErrInvalidStatus = errors.New("service: cert status invalid for revoke")

	// ErrNotConfigured is returned when the service is missing wiring
	// required to revoke (typically the ACME account key). Surfaces as
	// 503 so operators can fix the deploy without losing the request.
	ErrNotConfigured = errors.New("service: revoke not configured")

	// ErrNotFound mirrors repo.ErrNotFound at the service surface so the
	// handler does not need to import repo just to branch on it.
	ErrNotFound = errors.New("service: cert not found")
)

// certStatusRevoking is the transient cert.status value held between the
// optimistic-lock CAS and the CA's acknowledgement. The repo package does
// not export a constant for it (the column is TEXT-typed) — we define it
// here so the value is canonical in one place and search-greppable.
const certStatusRevoking repo.CertStatus = "revoking"

// reasonString returns the canonical wire-name for an ACME revocation
// reason. The repo column is TEXT; we keep these stable across releases.
func reasonString(r ca.RevokeReason) string {
	switch r {
	case ca.RevokeKeyCompromise:
		return "keyCompromise"
	case ca.RevokeCessationOfOperation:
		return "cessationOfOperation"
	case ca.RevokeCertificateHold:
		return "certificateHold"
	default:
		return "unspecified"
	}
}

// RevokeCert drives the revoke state machine for a single certificate.
//
// Flow:
//
//	issued ──user_revoke──► revoking ──ca_ack──► revoked
//	                          │
//	                          │ ca_fail
//	                          ▼
//	                       issued (rolled back; last_error recorded; user
//	                               can retry)
//
// Side effects on success:
//   - cert.certs.status flips to revoked with revoked_at + revoke_reason
//   - any queued / in-flight renewal_jobs for this cert are marked abandoned
//   - one cert.audit_logs row is appended with action=cert.revoke
//
// Side effects on CA failure: status rolls back to issued, an audit row is
// still appended (action=cert.revoke_failed) so the timeline survives.
//
// Idempotency: a cert that is already revoked surfaces ErrInvalidStatus —
// the handler maps that to 409 so a repeat client sees a stable contract.
func (s *Service) RevokeCert(ctx context.Context, accountID, certID int64, reason ca.RevokeReason) error {
	if s.repos() == nil {
		return ErrNotConfigured
	}
	if s.accountKey() == nil {
		return ErrNotConfigured
	}

	cert, err := s.repos().Certs.GetByID(ctx, certID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("revoke: load cert: %w", err)
	}
	if cert.AccountID != accountID {
		return ErrForbidden
	}
	if cert.Status != repo.CertStatusIssued {
		// Already revoked / expired — not a valid starting state. Caller
		// branches on this to surface a 409.
		return ErrInvalidStatus
	}

	reasonStr := reasonString(reason)

	// Optimistic-lock issued → revoking. A concurrent revoke racing this
	// one trips ErrInvalidStatus from the repo; surface the same sentinel
	// so the HTTP layer maps it to 409.
	if err := s.repos().Certs.UpdateStatus(ctx, cert.ID, repo.CertStatusIssued, certStatusRevoking, nil, &reasonStr); err != nil {
		if errors.Is(err, repo.ErrInvalidStatus) {
			return ErrInvalidStatus
		}
		return fmt.Errorf("revoke: status to revoking: %w", err)
	}

	// Audit log — revoke initiated. This is the user-visible WAL entry
	// for the revoke action; the order_events table is per-order and
	// would lose the cert-level context.
	startPayload, _ := json.Marshal(map[string]any{
		"reason": reasonStr,
		"state":  "started",
	})
	_ = s.appendRevokeAudit(ctx, accountID, cert.ID, "cert.revoke", startPayload)

	// Call the CA. lego's RevokeWithReason accepts the leaf PEM directly;
	// the adapter performs the PEM decode internally so we pass the
	// stored cert.leaf_pem bytes verbatim.
	caImpl, err := s.caPick(ctx, &repo.Order{CA: cert.Issuer})
	if err != nil {
		s.rollbackRevoke(ctx, cert.ID, accountID, reasonStr, err)
		return fmt.Errorf("revoke: ca pick: %w", err)
	}

	if err := caImpl.Revoke(ctx, []byte(cert.LeafPEM), reason, s.accountKey()); err != nil {
		s.rollbackRevoke(ctx, cert.ID, accountID, reasonStr, err)
		// Propagate the CA sentinel verbatim so the handler can branch
		// on ca.ErrCAQuotaExceeded / ca.ErrNetwork etc.
		return fmt.Errorf("revoke: ca revoke: %w", err)
	}

	// Persist the terminal status.
	now := time.Now().UTC()
	if err := s.repos().Certs.UpdateStatus(ctx, cert.ID, certStatusRevoking, repo.CertStatusRevoked, &now, &reasonStr); err != nil {
		// Highly unlikely (we just owned the transition). If it does
		// happen — e.g. someone hand-edited the row — log via audit and
		// return so an operator can investigate.
		return fmt.Errorf("revoke: status to revoked: %w", err)
	}

	// Best-effort cancel of any active renewal jobs for this cert. The
	// repo has no per-cert filter; we scan ListQueued and match on
	// CertID. Failures here are non-fatal — the worker tolerates an
	// abandoned job at pickup time and the cert is already revoked.
	s.cancelRenewalJobs(ctx, cert.ID)

	// Audit log — revoke completed.
	donePayload, _ := json.Marshal(map[string]any{
		"reason": reasonStr,
		"state":  "completed",
	})
	_ = s.appendRevokeAudit(ctx, accountID, cert.ID, "cert.revoke", donePayload)

	return nil
}

// rollbackRevoke flips cert.status back to issued and records the CA
// failure in audit_logs. All failures are swallowed — the caller is
// already returning an error to the user, and a noisy rollback path
// would just compound the problem.
func (s *Service) rollbackRevoke(ctx context.Context, certID, accountID int64, reason string, caErr error) {
	_ = s.repos().Certs.UpdateStatus(ctx, certID, certStatusRevoking, repo.CertStatusIssued, nil, nil)
	payload, _ := json.Marshal(map[string]any{
		"reason": reason,
		"state":  "failed",
		"error":  caErr.Error(),
	})
	_ = s.appendRevokeAudit(ctx, accountID, certID, "cert.revoke_failed", payload)
}

// cancelRenewalJobs marks every queued renewal_job whose cert_id matches
// the revoked cert as "abandoned". The renewer's ListQueued filter skips
// abandoned rows so no further work is scheduled for the dead cert.
func (s *Service) cancelRenewalJobs(ctx context.Context, certID int64) {
	// 200 is the default renewer scan window x 2; a single revoke can
	// reasonably affect at most one or two queued jobs in practice.
	jobs, err := s.repos().RenewalJobs.ListQueued(ctx, 200)
	if err != nil {
		return
	}
	for _, j := range jobs {
		if j.CertID != certID {
			continue
		}
		_ = s.repos().RenewalJobs.UpdateStatus(ctx, j.ID, "abandoned", nil, j.NewOrderID)
	}
}

// appendRevokeAudit writes one row to cert.audit_logs. Failures are
// returned to the caller (the public RevokeCert always discards the
// error — audit failure must not block the revoke itself).
func (s *Service) appendRevokeAudit(ctx context.Context, accountID, certID int64, action string, payload []byte) error {
	if s.repos() == nil || s.repos().AuditLogs == nil {
		return nil
	}
	actor := fmt.Sprintf("user:%d", accountID)
	target := "cert"
	tid := certID
	acc := accountID
	return s.repos().AuditLogs.Append(ctx, &repo.AuditLog{
		AccountID:  &acc,
		Actor:      actor,
		Action:     action,
		TargetKind: &target,
		TargetID:   &tid,
		Payload:    payload,
	})
}
