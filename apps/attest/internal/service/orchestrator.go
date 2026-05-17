package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kite365/idcd/lib/attest/pdfsign"
	attestrec "github.com/kite365/idcd/lib/attest/record"
)

// tsaDuplicateFetchTotal counts pipeline runs in which step 7 fetched a
// TSA token for WAL audit and step 8 (pdfsign.Sign) fetched a SECOND
// token for embedding into the PAdES-T signature. See orchestrator step
// 7 comment for the architectural reason — the two tokens cover
// different bytes (raw PDF digest vs CMS EncryptedDigest), so they
// cannot be the same blob; this counter just exposes the operational
// cost (2× TSA quota per verdict) for ops dashboards / alerts.
//
// Exposed as an atomic uint64 (no Prometheus dep in lib/attest today);
// /healthz handlers or future metrics handlers can read it via the
// TSADuplicateFetchTotal accessor.
var tsaDuplicateFetchTotal atomic.Uint64

// TSADuplicateFetchTotal returns the cumulative number of duplicate
// TSA fetches observed since process start. See tsaDuplicateFetchTotal.
func TSADuplicateFetchTotal() uint64 { return tsaDuplicateFetchTotal.Load() }

// GenerateVerdict drives the 10-step verdict pipeline for one order. It
// is safe to call repeatedly on the same orderID — the attestation_record
// WAL (D4) lets the orchestrator resume from the last successful
// externally-effecting step (KMS sign / TSA stamp / WORM archive).
//
// Pipeline (per docs/prd/18 §3.2):
//
//  1. Fetch raw observations (TimescaleDB)        — pure read; not in WAL
//  2. Cross-validate across nodes                 — pure compute; not in WAL
//  3. LLM interpret                               — skipped in S2 MVP
//  4. Render PDF                                  — pure compute; not in WAL
//  5. Hash content                                — pure compute; not in WAL
//  6. KMS sign                                    — WAL: ActionSigned
//  7. RFC3161 TSA stamp                           — WAL: ActionTSAStamped
//  8. Embed signature + TSA into PDF              — pure compute; not in WAL
//     (pdfsign re-fetches TSA — NECESSARY: step 7's token is over the
//     raw PDF digest, step 8's must be over the inner CMS
//     EncryptedDigest; same imprint impossible. Cost surfaced via
//     TSADuplicateFetchTotal counter.)
//  9. Blockchain anchor                           — skipped in S2 MVP
// 10. S3 WORM archive                             — WAL: ActionS3Archived
//
// Steps 1/2/4/5/8 are deterministic functions of their inputs (or stubs
// in S2 MVP) and re-running them produces identical bytes, so they need
// no WAL row. Steps 6/7/10 cross trust boundaries (KMS audit log, TSA
// serial number, S3 object) and therefore consult the WAL.
func (s *Service) GenerateVerdict(ctx context.Context, orderID string) error {
	order, err := s.cfg.Orders.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("orders get: %w", err)
	}
	if order.Status != "paid" && order.Status != "generating" {
		return fmt.Errorf("%w: orderID=%s status=%q", ErrUnexpectedOrderStatus, orderID, order.Status)
	}

	// Best-effort transition into the generating state. Failure here is
	// usually a transient DB hiccup or a concurrent worker that already
	// flipped it; either way the orchestrator can proceed safely because
	// the WAL serialises external side effects.
	if order.Status == "paid" {
		if err := s.cfg.Orders.UpdateStatus(ctx, orderID, "paid", "generating", nil); err != nil {
			s.cfg.Logger.Warn("attest/service: status paid->generating failed (will proceed)",
				slog.String("order_id", orderID),
				slog.String("err", err.Error()),
			)
		}
	}

	// Resume: re-use any existing report row so its ID anchors the WAL.
	rep, err := s.cfg.Reports.GetByOrderID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("reports get-by-order: %w", err)
	}
	if rep == nil {
		rep = &Report{
			ID:         newReportID(),
			OrderID:    orderID,
			ReportType: "observation_only",
		}
	}

	replayer := &attestrec.Replayer{Repo: s.cfg.AttestationRecords}

	// ----- Steps 1-4: deterministic reads / renders ------------------
	obs, err := fetchObservations(ctx, order)
	if err != nil {
		return s.failPipeline(ctx, order, "fetch observations", err)
	}
	nodes, consistency := crossValidate(ctx, obs)
	rep.NodeConsistencyPct = consistency
	rep.NodesUsed = encodeNodesJSON(nodes)

	pdfBytes, err := renderPDF(order, obs, nodes)
	if err != nil {
		return s.failPipeline(ctx, order, "render pdf", err)
	}

	// ----- Step 5: content hash -------------------------------------
	rep.ContentHash = sha256Hex(pdfBytes)

	// ----- Step 6: KMS sign -----------------------------------------
	signature, err := s.runSignStep(ctx, replayer, rep.ID, pdfBytes)
	if err != nil {
		return s.failPipeline(ctx, order, "kms sign", err)
	}
	rep.Signature = signature
	rep.SignatureKeyID = s.cfg.Signer.KeyID()
	if ver, verErr := s.cfg.Signer.KeyVersion(ctx); verErr == nil {
		rep.SignatureKeyVersion = ver
	} else {
		s.cfg.Logger.Warn("attest/service: signer KeyVersion failed; recording 0",
			slog.String("err", verErr.Error()),
		)
	}

	// ----- Step 7: RFC3161 TSA --------------------------------------
	tsaToken, tsaIssuedAt, err := s.runTSAStep(ctx, replayer, rep.ID, pdfBytes)
	if err != nil {
		return s.failPipeline(ctx, order, "tsa stamp", err)
	}
	rep.TSAProvider = s.cfg.TSA.Name()
	rep.TSATime = tsaIssuedAt

	// Architectural note (P1#8 follow-up): step 7's TSA token is over the
	// raw PDF digest (rep.ContentHash); step 8 (pdfsign) needs a token
	// over the inner CMS EncryptedDigest, which is computed AFTER the
	// KMS sign inside pdfsign — these are two DIFFERENT message imprints
	// by construction, so the upstream digitorus/pdfsign cannot be
	// taught to "reuse" tsaToken even with an API extension. Step 7
	// remains for D4 WAL audit (tsa_response row, replay safety); step
	// 8 must re-fetch. We surface the duplicate-fetch cost via a
	// counter + structured log so ops can size TSA quotas correctly.
	_ = tsaToken
	tsaDuplicateFetchTotal.Add(1)
	if s.cfg.TSAEndpoint != "" {
		s.cfg.Logger.Debug("attest/service: step-7 TSA token archived; step-8 pdfsign will fetch a second token (different message imprint by design)",
			slog.String("report_id", rep.ID),
			slog.String("tsa_provider", rep.TSAProvider),
			slog.Uint64("dup_fetch_total", tsaDuplicateFetchTotal.Load()),
		)
	}

	// ----- Step 8: embed signature + TSA into PDF -------------------
	signedPDF, err := pdfsign.Sign(ctx, pdfsign.SignRequest{
		Input: pdfBytes,
		// pdfsign's CMS layer hashes its own ByteRange and calls the
		// closure once. Returning the pre-computed step-6 signature
		// would yield a corrupt PAdES blob because the digests differ;
		// instead we call Signer.Sign here too, reusing a stable
		// idempotency key so KMS de-dupes (D4 second layer).
		KMSSign: func(innerCtx context.Context, digest []byte) ([]byte, error) {
			idem := fmt.Sprintf("%s:embed", rep.ID)
			return s.cfg.Signer.Sign(innerCtx, digest, idem)
		},
		SignerCertificate: s.cfg.SignerCert,
		CertificateChain:  s.cfg.CertChain,
		TSAEndpoint:       s.cfg.TSAEndpoint,
		Name:              "idcd Evidence",
		Reason:            "Verdict report",
	})
	if err != nil {
		return s.failPipeline(ctx, order, "pdf embed", err)
	}

	// ----- Step 10: WORM archive ------------------------------------
	archiveURL, err := s.runArchiveStep(ctx, replayer, rep.ID, signedPDF)
	if err != nil {
		return s.failPipeline(ctx, order, "s3 archive", err)
	}
	rep.PDFURL = archiveURL
	rep.ArchivedURL = archiveURL
	if rep.CreatedAt.IsZero() {
		rep.CreatedAt = time.Now().UTC()
	}

	// Persist the verdict_report row. ON CONFLICT (order_id) DO NOTHING
	// in the repo layer means a replay after a partial crash here is a
	// no-op rather than an error; log either way for diagnostics.
	if _, err := s.cfg.Reports.Insert(ctx, rep); err != nil {
		s.cfg.Logger.Warn("attest/service: report insert (possibly replay)",
			slog.String("order_id", orderID),
			slog.String("report_id", rep.ID),
			slog.String("err", err.Error()),
		)
	}

	if err := s.cfg.Orders.SetDelivered(ctx, orderID, time.Now().UTC()); err != nil {
		// Failing to mark delivered is logged but not propagated — the
		// verdict is durable in WORM and the customer can still fetch
		// it. The webhook / status reconciler will eventually fix the
		// order row.
		s.cfg.Logger.Warn("attest/service: SetDelivered failed",
			slog.String("order_id", orderID),
			slog.String("err", err.Error()),
		)
	}

	return nil
}

// runSignStep consults the WAL for ActionSigned and either reuses the
// stored signature or invokes Signer.Sign with a stable idempotency key.
func (s *Service) runSignStep(ctx context.Context, replayer *attestrec.Replayer, reportID string, pdfBytes []byte) ([]byte, error) {
	cont, ext, err := replayer.ShouldRun(ctx, reportID, attestrec.ActionSigned)
	if err != nil {
		return nil, fmt.Errorf("wal signed: %w", err)
	}
	if !cont {
		sig, decErr := hex.DecodeString(ext)
		if decErr != nil {
			return nil, fmt.Errorf("wal signed: decode external_id: %w", decErr)
		}
		return sig, nil
	}

	signCtx, cancel := context.WithTimeout(ctx, s.cfg.SignTimeout)
	defer cancel()
	digest := sha256Bytes(pdfBytes)
	idemKey := fmt.Sprintf("%s:signed", reportID)
	sig, err := s.cfg.Signer.Sign(signCtx, digest, idemKey)
	if err != nil {
		if recErr := replayer.Record(ctx, reportID, attestrec.ActionSigned, attestrec.StatusFailure, "", err.Error()); recErr != nil {
			s.cfg.Logger.Warn("attest/service: WAL record failure write failed",
				slog.String("action", string(attestrec.ActionSigned)),
				slog.String("err", recErr.Error()),
			)
		}
		return nil, err
	}
	if recErr := replayer.Record(ctx, reportID, attestrec.ActionSigned, attestrec.StatusSuccess, hex.EncodeToString(sig), ""); recErr != nil {
		// Success was written to KMS but not to WAL. Returning the
		// signature is safe (next replay will dedupe via KMS
		// idempotency key) but we log loudly.
		s.cfg.Logger.Error("attest/service: WAL record success write failed (KMS already signed)",
			slog.String("action", string(attestrec.ActionSigned)),
			slog.String("err", recErr.Error()),
		)
	}
	return sig, nil
}

// runTSAStep consults the WAL for ActionTSAStamped and either reuses the
// stored token+time or calls the TSA provider.
func (s *Service) runTSAStep(ctx context.Context, replayer *attestrec.Replayer, reportID string, pdfBytes []byte) ([]byte, time.Time, error) {
	cont, ext, err := replayer.ShouldRun(ctx, reportID, attestrec.ActionTSAStamped)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("wal tsa: %w", err)
	}
	if !cont {
		tok, ts, decErr := decodeTSAExternal(ext)
		if decErr != nil {
			return nil, time.Time{}, fmt.Errorf("wal tsa: decode external_id: %w", decErr)
		}
		return tok, ts, nil
	}

	tsaCtx, cancel := context.WithTimeout(ctx, s.cfg.TSATimeout)
	defer cancel()
	digest := sha256Bytes(pdfBytes)
	token, issued, err := s.cfg.TSA.Stamp(tsaCtx, crypto.SHA256, digest)
	if err != nil {
		if recErr := replayer.Record(ctx, reportID, attestrec.ActionTSAStamped, attestrec.StatusFailure, "", err.Error()); recErr != nil {
			s.cfg.Logger.Warn("attest/service: WAL record failure write failed",
				slog.String("action", string(attestrec.ActionTSAStamped)),
				slog.String("err", recErr.Error()),
			)
		}
		return nil, time.Time{}, err
	}
	if recErr := replayer.Record(ctx, reportID, attestrec.ActionTSAStamped, attestrec.StatusSuccess, encodeTSAExternal(token, issued), ""); recErr != nil {
		s.cfg.Logger.Error("attest/service: WAL record success write failed (TSA already issued)",
			slog.String("action", string(attestrec.ActionTSAStamped)),
			slog.String("err", recErr.Error()),
		)
	}
	return token, issued, nil
}

// runArchiveStep consults the WAL for ActionS3Archived and either reuses
// the recorded URL or invokes the Archiver.
func (s *Service) runArchiveStep(ctx context.Context, replayer *attestrec.Replayer, reportID string, signedPDF []byte) (string, error) {
	cont, ext, err := replayer.ShouldRun(ctx, reportID, attestrec.ActionS3Archived)
	if err != nil {
		return "", fmt.Errorf("wal archive: %w", err)
	}
	if !cont {
		// We stored "<url>|<etag>" in external_id so verifier flows can
		// audit the ETag independently of the URL.
		url, _ := splitArchiveExternal(ext)
		return url, nil
	}

	url, etag, err := s.cfg.Archiver.Archive(ctx, reportID+".pdf", signedPDF)
	if err != nil {
		if recErr := replayer.Record(ctx, reportID, attestrec.ActionS3Archived, attestrec.StatusFailure, "", err.Error()); recErr != nil {
			s.cfg.Logger.Warn("attest/service: WAL record failure write failed",
				slog.String("action", string(attestrec.ActionS3Archived)),
				slog.String("err", recErr.Error()),
			)
		}
		return "", err
	}
	if recErr := replayer.Record(ctx, reportID, attestrec.ActionS3Archived, attestrec.StatusSuccess, encodeArchiveExternal(url, etag), ""); recErr != nil {
		s.cfg.Logger.Error("attest/service: WAL record success write failed (object already in WORM)",
			slog.String("action", string(attestrec.ActionS3Archived)),
			slog.String("err", recErr.Error()),
		)
	}
	return url, nil
}

// failPipeline marks the order as failed and returns a wrapped error.
// Refund / customer notification is the Refund worker's responsibility
// (D5); this orchestrator only sets the terminal state on verdict_order.
func (s *Service) failPipeline(ctx context.Context, order *Order, step string, err error) error {
	msg := fmt.Sprintf("%s: %v", step, err)
	if setErr := s.cfg.Orders.SetFailed(ctx, order.ID, time.Now().UTC(), msg); setErr != nil {
		s.cfg.Logger.Error("attest/service: SetFailed failed",
			slog.String("order_id", order.ID),
			slog.String("step", step),
			slog.String("err", setErr.Error()),
		)
	}
	return fmt.Errorf("pipeline failed at %s: %w", step, err)
}

// -----------------------------------------------------------------------
// small helpers — kept here so the orchestrator file is self-contained
// -----------------------------------------------------------------------

func sha256Bytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// encodeTSAExternal serialises (token, issuedAt) as
// "<hex(token)>:<RFC3339Nano>" so the WAL row is plain ASCII and round
// trips through the DB unchanged.
func encodeTSAExternal(token []byte, issuedAt time.Time) string {
	return hex.EncodeToString(token) + ":" + issuedAt.UTC().Format(time.RFC3339Nano)
}

func decodeTSAExternal(s string) ([]byte, time.Time, error) {
	// Token is hex-only (no colons), so the first ':' is the boundary;
	// using strings.Index — not LastIndex — keeps the RFC3339 timestamp
	// (which itself contains colons) intact.
	idx := strings.Index(s, ":")
	if idx <= 0 {
		return nil, time.Time{}, fmt.Errorf("malformed tsa external_id %q", s)
	}
	tok, err := hex.DecodeString(s[:idx])
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("decode tsa token: %w", err)
	}
	ts, err := time.Parse(time.RFC3339Nano, s[idx+1:])
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("decode tsa time: %w", err)
	}
	return tok, ts, nil
}

func encodeArchiveExternal(url, etag string) string { return url + "|" + etag }

func splitArchiveExternal(s string) (url, etag string) {
	if i := strings.Index(s, "|"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// newReportID returns "vr_" + 24 base32 chars. Matches the format
// established by lib/attest/record.NewRecordID so dev tooling can grep
// both ID classes with a single regex.
func newReportID() string {
	var b [15]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Errorf("attest/service: crypto/rand failed: %w", err))
	}
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return "vr_" + strings.ToLower(enc)
}

// encodeNodesJSON renders a list of node IDs as a compact JSON array.
// Lives here (not in stubs) because the orchestrator owns the column
// shape even after the cross-validation logic is replaced with a real
// implementation.
//
// Node IDs flow in from idcd_main.monitor_check.node_results.node_id —
// nominally ASCII slugs ("node-cn-bj"), but the source column is text
// and operator typos or future renaming policies could introduce
// quotes, backslashes, or control bytes. We delegate to encoding/json
// so any such input round-trips correctly (JSON-escaped) instead of
// producing a syntactically broken array that breaks downstream
// verdict_report.nodes_used consumers.
func encodeNodesJSON(nodes []string) []byte {
	if len(nodes) == 0 {
		return []byte("[]")
	}
	// json.Marshal on a []string never fails for any UTF-8 input; for
	// non-UTF-8 bytes it emits U+FFFD which is still valid JSON and
	// preferable to a malformed array.
	out, err := json.Marshal(nodes)
	if err != nil {
		// Defensive: fall back to an empty array rather than panicking.
		// This branch is unreachable for json.Marshal([]string), but
		// keeps the function total in the face of future signature
		// changes.
		return []byte("[]")
	}
	return out
}
