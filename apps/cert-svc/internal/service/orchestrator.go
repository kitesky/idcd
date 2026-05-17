package service

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
	"github.com/kite365/idcd/lib/cert/dns/manual"
	"github.com/kite365/idcd/lib/cert/vault"
)

// State machine WAL action names. Each transition writes a single event.
// The replay pass uses these to compute "where to resume" on a crash.
const (
	actionOrderPicked         = "order_picked"
	actionKeyGenerated        = "key_generated"
	actionCSRBuilt            = "csr_built"
	actionDNSSolverBuilt      = "dns_solver_built"
	actionACMERequestStarted  = "acme_request_started"
	actionACMERequestComplete = "acme_request_completed"
	actionACMERequestFailed   = "acme_request_failed"
	actionCertPersisted       = "cert_persisted"
	actionRenewalEnqueued     = "renewal_job_enqueued"
	actionRevokeStarted       = "revoke_started"
	actionRevokeCompleted     = "revoke_completed"
)

// driveState captures the in-memory replay of an order's event log. The
// boolean "needs" flags drive the state machine — every step is a no-op
// when its result has already been recorded in the WAL.
type driveState struct {
	NextSeq int

	// allActions is the chronological list of every recorded action,
	// used by branches (e.g. revoke) that need a full history scan
	// rather than the derived "needs" flags.
	allActions []string

	// Step results recovered from the WAL.
	EncryptedKey *vault.EncryptedKey
	PrivKey      crypto.Signer
	CSRPEM       []byte
	SolverReady  bool
	CertResult   *ca.CertificateResult
	CertID       int64

	// Derived flags after replay.
	NeedsKey       bool
	NeedsCSR       bool
	NeedsDNSSolver bool
	NeedsCA        bool
	NeedsPersist   bool
	NeedsRenewal   bool

	// LastFailure is set when the most recent terminal event was an
	// ACME failure — DriveOrder returns immediately so retries go
	// through RetryOrder.
	LastFailure string
}

// DriveOrder advances one order through its remaining lifecycle. It is
// safe to call multiple times for the same orderID; the WAL replay
// short-circuits every already-recorded step.
func (s *Service) DriveOrder(ctx context.Context, orderID int64) error {
	order, err := s.repos().Orders.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("drive order %d: load: %w", orderID, err)
	}

	switch order.Status {
	case repo.OrderStatusDraft, repo.OrderStatusValidating, repo.OrderStatusIssuing, repo.OrderStatusFailed:
		return s.driveIssue(ctx, order)
	case repo.OrderStatusRevoking:
		return s.driveRevoke(ctx, order)
	case repo.OrderStatusIssued, repo.OrderStatusRevoked:
		// Terminal; nothing to do.
		return nil
	default:
		return fmt.Errorf("%w: status=%s", ErrOrderNotPickable, order.Status)
	}
}

// driveIssue runs the issuance state machine.
func (s *Service) driveIssue(ctx context.Context, order *repo.Order) error {
	logger := s.cfg.Logger.With("order_id", order.ID)

	state, err := s.replayEvents(ctx, order)
	if err != nil {
		return fmt.Errorf("replay events: %w", err)
	}

	if state.LastFailure != "" && order.Status == repo.OrderStatusFailed {
		// Don't auto-retry; RetryOrder is the explicit path.
		return fmt.Errorf("order in failed state: %s", state.LastFailure)
	}

	// Mark order as picked on first run.
	if state.NextSeq == 1 {
		if err := s.appendEvent(ctx, order.ID, &state, actionOrderPicked, nil); err != nil {
			return err
		}
	}

	// Step 1: key.
	if state.NeedsKey {
		plainPEM, ek, err := s.vaultV().GenerateKey(ctx, vault.KeyAlgECDSAP256)
		if err != nil {
			return fmt.Errorf("generate key: %w", err)
		}
		priv, err := parsePrivateKey(plainPEM)
		if err != nil {
			return fmt.Errorf("parse generated key: %w", err)
		}
		payload, err := json.Marshal(ek)
		if err != nil {
			return fmt.Errorf("marshal encrypted key: %w", err)
		}
		if err := s.appendEvent(ctx, order.ID, &state, actionKeyGenerated, payload); err != nil {
			return err
		}
		state.EncryptedKey = &ek
		state.PrivKey = priv
		state.NeedsKey = false
	}

	// Step 2: CSR.
	if state.NeedsCSR {
		if state.PrivKey == nil {
			return fmt.Errorf("internal: NeedsCSR=true but PrivKey nil")
		}
		csrPEM, err := buildCSR(state.PrivKey, order.SANs)
		if err != nil {
			return fmt.Errorf("build csr: %w", err)
		}
		if err := s.appendEvent(ctx, order.ID, &state, actionCSRBuilt, csrPEM); err != nil {
			return err
		}
		state.CSRPEM = csrPEM
		state.NeedsCSR = false
	}

	// Step 3: solver. The solver itself isn't recoverable from the WAL —
	// it's an in-memory object — but writing the event lets the replay
	// path know whether the operator passed validating (e.g. user
	// confirmed manual TXT). We always rebuild the solver in the same
	// invocation if NeedsCA is also true.
	var solver ca.DnsSolver
	if state.NeedsCA {
		sv, err := s.buildSolver(ctx, order)
		if err != nil {
			return fmt.Errorf("build dns solver: %w", err)
		}
		solver = sv
		if state.NeedsDNSSolver {
			if err := s.appendEventDirect(ctx, order.ID, &state, actionDNSSolverBuilt, nil); err != nil {
				return err
			}
			state.NeedsDNSSolver = false
		}
	}

	// Step 4: CA call.
	if state.NeedsCA {
		// Status: draft/validating → validating → issuing.
		if order.Status == repo.OrderStatusDraft {
			if err := s.repos().Orders.UpdateStatus(ctx, order.ID, repo.OrderStatusDraft, repo.OrderStatusValidating, nil); err != nil {
				if !errors.Is(err, repo.ErrInvalidStatus) {
					return fmt.Errorf("set status validating: %w", err)
				}
			}
			order.Status = repo.OrderStatusValidating
		}
		if order.Status == repo.OrderStatusValidating {
			if err := s.repos().Orders.UpdateStatus(ctx, order.ID, repo.OrderStatusValidating, repo.OrderStatusIssuing, nil); err != nil {
				if !errors.Is(err, repo.ErrInvalidStatus) {
					return fmt.Errorf("set status issuing: %w", err)
				}
			}
			order.Status = repo.OrderStatusIssuing
		}

		if err := s.appendEvent(ctx, order.ID, &state, actionACMERequestStarted, nil); err != nil {
			return err
		}

		caImpl, err := s.caPick(ctx, order)
		if err != nil {
			return fmt.Errorf("ca pick: %w", err)
		}

		result, err := caImpl.RequestCertificate(ctx, ca.CertificateRequest{
			AccountKey:   s.accountKey(),
			AccountEmail: s.accountEmail(),
			Domains:      normalizeSANs(order.SANs),
			CSR:          state.CSRPEM,
			PrivateKey:   state.PrivKey,
			DNS:          solver,
			Timeout:      s.caRequestTimeout(),
		})
		if err != nil {
			errStr := err.Error()
			_ = s.appendEvent(ctx, order.ID, &state, actionACMERequestFailed, []byte(errStr))
			_ = s.repos().Orders.UpdateStatus(ctx, order.ID, repo.OrderStatusIssuing, repo.OrderStatusFailed, &errStr)
			_ = s.repos().Orders.IncrementRetryCount(ctx, order.ID)
			logger.Error("acme request failed", "error", err)
			return fmt.Errorf("acme request: %w", err)
		}

		payload, _ := json.Marshal(struct {
			Serial    string    `json:"serial"`
			NotBefore time.Time `json:"not_before"`
			NotAfter  time.Time `json:"not_after"`
		}{result.Serial, result.NotBefore, result.NotAfter})
		if err := s.appendEvent(ctx, order.ID, &state, actionACMERequestComplete, payload); err != nil {
			return err
		}
		state.CertResult = &result
		state.NeedsCA = false
		state.NeedsPersist = true
	}

	// Step 5: persist cert row.
	if state.NeedsPersist {
		if state.CertResult == nil {
			return fmt.Errorf("internal: NeedsPersist=true but CertResult nil")
		}
		if state.EncryptedKey == nil {
			return fmt.Errorf("internal: NeedsPersist=true but EncryptedKey nil")
		}
		handle, err := encodeKeyHandle(*state.EncryptedKey)
		if err != nil {
			return fmt.Errorf("encode key handle: %w", err)
		}
		fingerprint := sha256Fingerprint(state.CertResult.LeafPEM)

		cert := &repo.Cert{
			OrderID:           order.ID,
			AccountID:         order.AccountID,
			SANs:              order.SANs,
			Issuer:            "lets-encrypt",
			SerialHex:         state.CertResult.Serial,
			FingerprintSHA256: fingerprint,
			LeafPEM:           string(state.CertResult.LeafPEM),
			ChainPEM:          string(state.CertResult.ChainPEM),
			KeyKMSHandle:      handle,
			NotBefore:         state.CertResult.NotBefore,
			NotAfter:          state.CertResult.NotAfter,
			Status:            repo.CertStatusIssued,
		}
		certID, err := s.repos().Certs.Insert(ctx, cert)
		if err != nil && !errors.Is(err, repo.ErrConflict) {
			return fmt.Errorf("certs insert: %w", err)
		}
		state.CertID = certID
		payload, _ := json.Marshal(struct {
			CertID int64 `json:"cert_id"`
		}{certID})
		if err := s.appendEvent(ctx, order.ID, &state, actionCertPersisted, payload); err != nil {
			return err
		}
		if err := s.repos().Orders.SetCertID(ctx, order.ID, certID); err != nil && !errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("set cert id: %w", err)
		}
		if err := s.repos().Orders.UpdateStatus(ctx, order.ID, repo.OrderStatusIssuing, repo.OrderStatusIssued, nil); err != nil && !errors.Is(err, repo.ErrInvalidStatus) {
			return fmt.Errorf("set issued: %w", err)
		}
		now := time.Now().UTC()
		if err := s.repos().Orders.SetFinalizedAt(ctx, order.ID, now); err != nil && !errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("set finalized_at: %w", err)
		}
		state.NeedsPersist = false
		state.NeedsRenewal = true
	}

	// Step 6: renewal queue.
	if state.NeedsRenewal {
		if state.CertResult == nil {
			// Recovered after persist event but before renewal — we
			// don't have NotAfter; load the cert.
			cert, err := s.repos().Certs.GetByID(ctx, state.CertID)
			if err != nil {
				return fmt.Errorf("reload cert for renewal: %w", err)
			}
			state.CertResult = &ca.CertificateResult{NotAfter: cert.NotAfter}
		}
		job := &repo.RenewalJob{
			CertID:      state.CertID,
			ScheduledAt: state.CertResult.NotAfter.Add(-30 * 24 * time.Hour),
			Status:      "queued",
		}
		if _, err := s.repos().RenewalJobs.Insert(ctx, job); err != nil && !errors.Is(err, repo.ErrConflict) {
			return fmt.Errorf("renewal job insert: %w", err)
		}
		if err := s.appendEvent(ctx, order.ID, &state, actionRenewalEnqueued, nil); err != nil {
			return err
		}
		state.NeedsRenewal = false
	}

	s.dropManualCoordinator(order.ID)
	return nil
}

// driveRevoke handles a revocation request. The cert.certs row's
// key_kms_handle holds the encrypted account key handle of the cert key,
// but revocation in S1 uses the platform's account key (RFC 8555 §7.6).
func (s *Service) driveRevoke(ctx context.Context, order *repo.Order) error {
	if order.CertID == nil {
		return fmt.Errorf("revoke: order %d has no cert_id", order.ID)
	}
	cert, err := s.repos().Certs.GetByID(ctx, *order.CertID)
	if err != nil {
		return fmt.Errorf("revoke: load cert: %w", err)
	}

	state, err := s.replayEvents(ctx, order)
	if err != nil {
		return fmt.Errorf("revoke: replay: %w", err)
	}

	hasStart := false
	hasDone := false
	for _, ev := range state.allActions {
		if ev == actionRevokeStarted {
			hasStart = true
		}
		if ev == actionRevokeCompleted {
			hasDone = true
		}
	}

	if !hasStart {
		if err := s.appendEvent(ctx, order.ID, &state, actionRevokeStarted, nil); err != nil {
			return err
		}
	}

	if !hasDone {
		caImpl, err := s.caPick(ctx, order)
		if err != nil {
			return err
		}
		reason := ca.RevokeUnspecified
		if cert.RevokeReason != nil {
			// pass-through; we don't parse it back to enum in S1
			_ = *cert.RevokeReason
		}
		if err := caImpl.Revoke(ctx, []byte(cert.LeafPEM), reason, s.accountKey()); err != nil {
			return fmt.Errorf("revoke ca: %w", err)
		}
		if err := s.appendEvent(ctx, order.ID, &state, actionRevokeCompleted, nil); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	_ = s.repos().Certs.UpdateStatus(ctx, cert.ID, repo.CertStatusIssued, repo.CertStatusRevoked, &now, cert.RevokeReason)
	_ = s.repos().Orders.UpdateStatus(ctx, order.ID, repo.OrderStatusRevoking, repo.OrderStatusRevoked, nil)
	_ = s.repos().Orders.SetFinalizedAt(ctx, order.ID, now)
	return nil
}

// RetryOrder is the explicit retry entry-point: it moves a failed order
// back to validating and re-enqueues it. The state machine itself is
// idempotent — anything already written stays.
func (s *Service) RetryOrder(ctx context.Context, orderID int64) error {
	order, err := s.repos().Orders.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("retry order %d: load: %w", orderID, err)
	}
	if order.Status != repo.OrderStatusFailed {
		return fmt.Errorf("%w: status=%s", ErrOrderNotPickable, order.Status)
	}
	// Reset to validating; CSR / key events stay so the replay reuses them.
	if err := s.repos().Orders.UpdateStatus(ctx, orderID, repo.OrderStatusFailed, repo.OrderStatusValidating, nil); err != nil {
		return fmt.Errorf("retry update status: %w", err)
	}
	return s.EnqueueOrder(ctx, orderID)
}

// replayEvents loads the WAL for an order and computes a driveState.
func (s *Service) replayEvents(ctx context.Context, order *repo.Order) (driveState, error) {
	events, err := s.repos().OrderEvents.ListByOrder(ctx, order.ID)
	if err != nil {
		return driveState{}, err
	}

	state := driveState{
		NextSeq:        1,
		NeedsKey:       true,
		NeedsCSR:       true,
		NeedsDNSSolver: true,
		NeedsCA:        true,
		NeedsPersist:   false,
		NeedsRenewal:   false,
	}

	for _, ev := range events {
		state.allActions = append(state.allActions, ev.Action)
		if ev.ActionSeq >= state.NextSeq {
			state.NextSeq = ev.ActionSeq + 1
		}
		switch ev.Action {
		case actionOrderPicked:
			// no-op
		case actionKeyGenerated:
			var ek vault.EncryptedKey
			if err := json.Unmarshal(ev.Payload, &ek); err != nil {
				return state, fmt.Errorf("replay key event: %w", err)
			}
			plainPEM, err := s.vaultV().DecryptKey(ctx, ek)
			if err != nil {
				return state, fmt.Errorf("replay decrypt key: %w", err)
			}
			priv, err := parsePrivateKey(plainPEM)
			if err != nil {
				return state, fmt.Errorf("replay parse key: %w", err)
			}
			state.EncryptedKey = &ek
			state.PrivKey = priv
			state.NeedsKey = false
		case actionCSRBuilt:
			state.CSRPEM = append([]byte(nil), ev.Payload...)
			state.NeedsCSR = false
		case actionDNSSolverBuilt:
			state.SolverReady = true
			state.NeedsDNSSolver = false
		case actionACMERequestStarted:
			// Marker only; if no completed event follows we still need
			// to redo the CA call.
		case actionACMERequestComplete:
			var p struct {
				Serial    string    `json:"serial"`
				NotBefore time.Time `json:"not_before"`
				NotAfter  time.Time `json:"not_after"`
			}
			_ = json.Unmarshal(ev.Payload, &p)
			state.CertResult = &ca.CertificateResult{
				Serial:    p.Serial,
				NotBefore: p.NotBefore,
				NotAfter:  p.NotAfter,
			}
			state.NeedsCA = false
			state.NeedsPersist = true
		case actionACMERequestFailed:
			state.LastFailure = string(ev.Payload)
		case actionCertPersisted:
			var p struct {
				CertID int64 `json:"cert_id"`
			}
			_ = json.Unmarshal(ev.Payload, &p)
			state.CertID = p.CertID
			state.NeedsPersist = false
			state.NeedsRenewal = true
		case actionRenewalEnqueued:
			state.NeedsRenewal = false
		}
	}
	return state, nil
}

// appendEvent writes one WAL entry and bumps state.NextSeq. ErrConflict
// (someone else already wrote this step) is treated as success.
func (s *Service) appendEvent(ctx context.Context, orderID int64, state *driveState, action string, payload []byte) error {
	return s.appendEventDirect(ctx, orderID, state, action, payload)
}

func (s *Service) appendEventDirect(ctx context.Context, orderID int64, state *driveState, action string, payload []byte) error {
	ev := &repo.OrderEvent{
		OrderID:   orderID,
		ActionSeq: state.NextSeq,
		Action:    action,
		Payload:   payload,
	}
	err := s.repos().OrderEvents.Append(ctx, ev)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			// Another worker / earlier crash already wrote this. Read the
			// next free seq and try once more — if that also fails the
			// caller's retry loop will pick it up.
			next, nerr := s.repos().OrderEvents.NextActionSeq(ctx, orderID)
			if nerr != nil {
				return fmt.Errorf("append conflict, next seq: %w", nerr)
			}
			state.NextSeq = next
			ev.ActionSeq = next
			if err2 := s.repos().OrderEvents.Append(ctx, ev); err2 != nil {
				return fmt.Errorf("append %s: %w", action, err2)
			}
		} else {
			return fmt.Errorf("append %s: %w", action, err)
		}
	}
	state.NextSeq = ev.ActionSeq + 1
	state.allActions = append(state.allActions, action)
	return nil
}

// buildSolver loads / decrypts the order's DNS credential and constructs
// a solver via the registry. Manual mode is handled by reusing this
// service's Coordinator map so the HTTP handler's
// MarkManualChallengeReady can unblock the in-flight challenge.
func (s *Service) buildSolver(ctx context.Context, order *repo.Order) (ca.DnsSolver, error) {
	if order.DNSCredentialID == nil {
		// No credential = manual mode. The per-order Coordinator must be
		// the same instance the HTTP handler injects into via
		// MarkManualChallengeReady — going through dnsReg().Get(KindManual)
		// would hand back the provider's built-in Coordinator, which is a
		// separate pending map. Build the solver directly from the
		// per-order Coordinator instead.
		co := s.ManualCoordinator(order.ID)
		solver, err := manual.SolverFromCoordinator(co)
		if err != nil {
			return nil, fmt.Errorf("manual solver: %w", err)
		}
		// Sanity check that the registry still knows about manual so
		// misconfigured deployments fail loudly rather than silently
		// drift between server / worker.
		if _, regErr := s.dnsReg().Get(dns.KindManual); regErr != nil {
			return nil, fmt.Errorf("manual provider not registered: %w", regErr)
		}
		return solver, nil
	}

	cred, err := s.repos().DNSCredentials.GetByID(ctx, *order.DNSCredentialID)
	if err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}

	// Decrypt the encrypted_blob into a map[string]string credential.
	var eb vault.EncryptedBlob
	if err := json.Unmarshal(cred.EncryptedBlob, &eb); err != nil {
		return nil, fmt.Errorf("decode encrypted blob: %w", err)
	}
	plaintext, err := s.vaultV().DecryptBlob(ctx, eb)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	credMap := map[string]string{}
	if err := json.Unmarshal(plaintext, &credMap); err != nil {
		return nil, fmt.Errorf("parse credential json: %w", err)
	}

	prov, err := s.dnsReg().Get(dns.ProviderKind(cred.Provider))
	if err != nil {
		return nil, fmt.Errorf("provider %q: %w", cred.Provider, err)
	}
	return prov.BuildSolver(ctx, credMap, normalizeSANs(order.SANs))
}

// encodeKeyHandle serialises an EncryptedKey for storage in
// cert.certs.key_kms_handle (TEXT). The renewer / revoker read it back
// via decodeKeyHandle.
func encodeKeyHandle(ek vault.EncryptedKey) (string, error) {
	b, err := json.Marshal(ek)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeKeyHandle reverses encodeKeyHandle; exported for renewer / revoker.
func DecodeKeyHandle(s string) (vault.EncryptedKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return vault.EncryptedKey{}, err
	}
	var ek vault.EncryptedKey
	if err := json.Unmarshal(raw, &ek); err != nil {
		return vault.EncryptedKey{}, err
	}
	return ek, nil
}

// DecodeAccountKey parses a PEM-encoded PKCS#8 key and returns a
// crypto.Signer; exported so cmd/worker can build the long-lived ACME
// account key without reaching into private helpers.
func DecodeAccountKey(pemBytes []byte) (crypto.Signer, error) {
	return parsePrivateKey(pemBytes)
}

// parsePrivateKey accepts a PEM-encoded PKCS#8 key and returns a Signer.
func parsePrivateKey(pemBytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("parse key: not PEM")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8: %w", err)
	}
	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("parse key: not a signer")
	}
	return signer, nil
}

// sha256Fingerprint computes the SHA-256 fingerprint of the DER-encoded
// leaf inside a PEM block.
func sha256Fingerprint(leafPEM []byte) string {
	block, _ := pem.Decode(leafPEM)
	if block == nil {
		// Fall back to hashing the whole PEM so we still write a
		// non-empty value; downstream tooling treats the fingerprint
		// as opaque text.
		sum := sha256.Sum256(leafPEM)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(block.Bytes)
	return hex.EncodeToString(sum[:])
}

