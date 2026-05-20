package service

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	attestrec "github.com/kite365/idcd/lib/attest/record"
	"github.com/kite365/idcd/lib/attest/sign"
	"github.com/kite365/idcd/lib/attest/tsa"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeOrderRepo struct {
	mu        sync.Mutex
	order     *Order
	delivered time.Time
	failed    time.Time
	failedMsg string
	updates   []string // "from->to"
}

func (f *fakeOrderRepo) GetByID(_ context.Context, id string) (*Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.order == nil || f.order.ID != id {
		return nil, fmt.Errorf("not found %s", id)
	}
	cp := *f.order
	return &cp, nil
}
func (f *fakeOrderRepo) UpdateStatus(_ context.Context, _ string, from, to string, _ *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, from+"->"+to)
	if f.order != nil {
		f.order.Status = to
	}
	return nil
}
func (f *fakeOrderRepo) SetDelivered(_ context.Context, _ string, t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delivered = t
	if f.order != nil {
		f.order.Status = "delivered"
	}
	return nil
}
func (f *fakeOrderRepo) SetFailed(_ context.Context, _ string, t time.Time, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = t
	f.failedMsg = reason
	if f.order != nil {
		f.order.Status = "failed"
	}
	return nil
}

type fakeReportRepo struct {
	mu      sync.Mutex
	byOrder map[string]*Report
	inserts int
}

func (f *fakeReportRepo) Insert(_ context.Context, r *Report) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.byOrder == nil {
		f.byOrder = map[string]*Report{}
	}
	cp := *r
	f.byOrder[r.OrderID] = &cp
	f.inserts++
	return r.ID, nil
}
func (f *fakeReportRepo) GetByOrderID(_ context.Context, orderID string) (*Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.byOrder[orderID]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, nil
}

// inMemAttestationRepo is a minimal implementation of
// attestrec.Repository keyed by (report_id, action). Mirrors the
// constraints documented on the interface — including
// ErrDuplicateAction on conflicting Insert.
type inMemAttestationRepo struct {
	mu   sync.Mutex
	rows map[string]*attestrec.Record // key = reportID + "|" + action
}

func newInMemAttestationRepo() *inMemAttestationRepo {
	return &inMemAttestationRepo{rows: map[string]*attestrec.Record{}}
}
func keyOf(reportID string, action attestrec.Action) string {
	return reportID + "|" + string(action)
}
func (r *inMemAttestationRepo) Insert(_ context.Context, rec *attestrec.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyOf(rec.ReportID, rec.Action)
	if _, ok := r.rows[k]; ok {
		return attestrec.ErrDuplicateAction
	}
	cp := *rec
	r.rows[k] = &cp
	return nil
}
func (r *inMemAttestationRepo) Get(_ context.Context, reportID string, action attestrec.Action) (*attestrec.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.rows[keyOf(reportID, action)]; ok {
		cp := *rec
		return &cp, nil
	}
	return nil, attestrec.ErrNotFound
}
func (r *inMemAttestationRepo) Update(_ context.Context, rec *attestrec.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rec
	r.rows[keyOf(rec.ReportID, rec.Action)] = &cp
	return nil
}
func (r *inMemAttestationRepo) ListByReport(_ context.Context, reportID string) ([]*attestrec.Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*attestrec.Record
	for _, rec := range r.rows {
		if rec.ReportID == reportID {
			cp := *rec
			out = append(out, &cp)
		}
	}
	return out, nil
}

// rsaSigner is a sign.Signer backed by a local RSA key, suitable as a
// stand-in for AWS / Aliyun KMS in unit tests. signCalls counts
// invocations so WAL-resume tests can assert "Sign was NOT called
// twice".
type rsaSigner struct {
	key       *rsa.PrivateKey
	keyID     string
	algorithm string
	signCalls int32
	failOnce  atomic.Bool // when true, first Sign returns failErr, then clears
	failErr   error
}

func (s *rsaSigner) KeyID() string                             { return s.keyID }
func (s *rsaSigner) KeyVersion(_ context.Context) (int, error) { return 1, nil }
func (s *rsaSigner) Algorithm() string                         { return s.algorithm }
func (s *rsaSigner) Sign(_ context.Context, digest []byte, idem string) ([]byte, error) {
	atomic.AddInt32(&s.signCalls, 1)
	if idem == "" {
		return nil, sign.ErrInvalidInput
	}
	if s.failOnce.Load() {
		s.failOnce.Store(false)
		return nil, s.failErr
	}
	return rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest)
}

// fakeTSA is a tsa.Provider that returns deterministic bytes (raw
// digest with a prefix is plenty) plus a fixed time. stampCalls
// supports WAL-resume assertions.
type fakeTSA struct {
	name       string
	stampCalls int32
	fixedTime  time.Time
	failOnce   atomic.Bool
	failErr    error
}

func (f *fakeTSA) Name() string { return f.name }
func (f *fakeTSA) Stamp(_ context.Context, _ crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	atomic.AddInt32(&f.stampCalls, 1)
	if f.failOnce.Load() {
		f.failOnce.Store(false)
		return nil, time.Time{}, f.failErr
	}
	tok := append([]byte("FAKETSA:"), digest...)
	return tok, f.fixedTime, nil
}

// fakeArchiver records every Archive call so tests can assert on
// behaviour without touching disk.
type fakeArchiver struct {
	mu           sync.Mutex
	archiveCalls int32
	lastKey      string
	lastBytes    []byte
	failOnce     atomic.Bool
	failErr      error
}

func (a *fakeArchiver) Archive(_ context.Context, key string, pdf []byte) (string, string, error) {
	atomic.AddInt32(&a.archiveCalls, 1)
	if a.failOnce.Load() {
		a.failOnce.Store(false)
		return "", "", a.failErr
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastKey = key
	a.lastBytes = append([]byte(nil), pdf...)
	return "s3://idcd-evidence/" + key, "etag-deadbeef", nil
}

// ---------------------------------------------------------------------------
// shared cert helper
// ---------------------------------------------------------------------------

func testCertAndKey(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idcd-test-orchestrator"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert, key
}

// harness wires fakes + RSA-backed Signer + httptest TSA into a
// ready-to-call Service. The PAdES-T step is exercised end-to-end via
// pdfsign + a local timestamp server so test coverage spans the inner
// pdfsign call too.
type harness struct {
	svc       *Service
	orders    *fakeOrderRepo
	reports   *fakeReportRepo
	wal       *inMemAttestationRepo
	signer    *rsaSigner
	tsa       *fakeTSA
	archiver  *fakeArchiver
	tsaServer *tsaTestServer // used only when TSAEndpoint is set
	cert      *x509.Certificate
	key       *rsa.PrivateKey
}

func newHarness(t *testing.T, order *Order, withRealTSAEndpoint bool) *harness {
	t.Helper()
	cert, key := testCertAndKey(t)

	h := &harness{
		orders:   &fakeOrderRepo{order: order},
		reports:  &fakeReportRepo{},
		wal:      newInMemAttestationRepo(),
		signer:   &rsaSigner{key: key, keyID: "kms-key-test", algorithm: sign.AlgorithmRSAPKCS1SHA256},
		tsa:      &fakeTSA{name: "faketsa", fixedTime: time.Unix(1_700_000_000, 0).UTC()},
		archiver: &fakeArchiver{},
		cert:     cert,
		key:      key,
	}

	tsaEndpoint := ""
	if withRealTSAEndpoint {
		h.tsaServer = startTSAServer(t, key, cert)
		tsaEndpoint = h.tsaServer.URL()
	}

	h.svc = New(Config{
		Orders:             h.orders,
		Reports:            h.reports,
		AttestationRecords: h.wal,
		Observations:       syntheticObservationPool{},
		Signer:             h.signer,
		TSA:                h.tsa,
		SignerCert:         cert,
		TSAEndpoint:        tsaEndpoint,
		Archiver:           h.archiver,
	})
	return h
}

// ---------------------------------------------------------------------------
// happy path
// ---------------------------------------------------------------------------

func TestGenerateVerdict_HappyPath(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_1", Status: "paid"}, true)

	if err := h.svc.GenerateVerdict(context.Background(), "vo_1"); err != nil {
		t.Fatalf("GenerateVerdict: %v", err)
	}

	if h.reports.inserts != 1 {
		t.Fatalf("reports.inserts = %d, want 1", h.reports.inserts)
	}
	if atomic.LoadInt32(&h.signer.signCalls) == 0 {
		t.Fatalf("signer.Sign never called")
	}
	if atomic.LoadInt32(&h.tsa.stampCalls) != 1 {
		t.Fatalf("tsa.Stamp calls = %d, want 1", h.tsa.stampCalls)
	}
	if atomic.LoadInt32(&h.archiver.archiveCalls) != 1 {
		t.Fatalf("archiver.Archive calls = %d, want 1", h.archiver.archiveCalls)
	}

	// WAL must contain success rows for signed / tsa_stamped /
	// s3_archived.
	ids := allReportIDs(h.reports)
	if len(ids) != 1 {
		t.Fatalf("want 1 report ID, got %d", len(ids))
	}
	for _, action := range []attestrec.Action{attestrec.ActionSigned, attestrec.ActionTSAStamped, attestrec.ActionS3Archived} {
		row, err := h.wal.Get(context.Background(), ids[0], action)
		if err != nil {
			t.Fatalf("WAL get %s: %v", action, err)
		}
		if row.Status != attestrec.StatusSuccess {
			t.Fatalf("WAL %s status = %s, want success", action, row.Status)
		}
	}

	// Order should be delivered.
	if h.orders.order.Status != "delivered" {
		t.Fatalf("order status = %q, want delivered", h.orders.order.Status)
	}
	// And UpdateStatus("paid"->"generating") was issued.
	if len(h.orders.updates) == 0 || h.orders.updates[0] != "paid->generating" {
		t.Fatalf("expected paid->generating update, got %v", h.orders.updates)
	}
}

// allReportIDs grabs persisted report IDs from the fake repo for test
// assertions. Used because the orchestrator generates the ID
// internally on first run.
func allReportIDs(r *fakeReportRepo) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.byOrder))
	for _, rep := range r.byOrder {
		out = append(out, rep.ID)
	}
	return out
}

// ---------------------------------------------------------------------------
// WAL resume — after a previous successful sign, the second invocation
// must not call Signer.Sign again for the WAL ActionSigned slot.
// ---------------------------------------------------------------------------

func TestGenerateVerdict_WALResume_SkipSign(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_2", Status: "paid"}, true)

	// Seed: pretend a prior run already completed the sign step. We
	// need a known report ID for that, so insert one into the report
	// repo first; the orchestrator will resume against it.
	rep := &Report{ID: "vr_preseed", OrderID: "vo_2", ReportType: "observation_only"}
	h.reports.byOrder = map[string]*Report{"vo_2": rep}

	// Pre-compute the digest the orchestrator will produce
	// (renderPDF is deterministic) so the WAL row holds a signature
	// that decodes cleanly.
	pdfBytes, err := renderPDF(h.orders.order, nil, nil)
	if err != nil {
		t.Fatalf("renderPDF: %v", err)
	}
	digest := sha256Bytes(pdfBytes)
	preSig, err := h.signer.Sign(context.Background(), digest, "vr_preseed:signed")
	if err != nil {
		t.Fatalf("seed sign: %v", err)
	}
	// Record success directly in the WAL so the orchestrator sees it.
	replayer := &attestrec.Replayer{Repo: h.wal}
	if err := replayer.Record(context.Background(), "vr_preseed", attestrec.ActionSigned, attestrec.StatusSuccess, hexEncode(preSig), ""); err != nil {
		t.Fatalf("seed WAL record: %v", err)
	}

	before := atomic.LoadInt32(&h.signer.signCalls)

	if err := h.svc.GenerateVerdict(context.Background(), "vo_2"); err != nil {
		t.Fatalf("GenerateVerdict: %v", err)
	}

	after := atomic.LoadInt32(&h.signer.signCalls)
	// Step 6 must be skipped. The embed step inside pdfsign still
	// calls Signer.Sign once (different idempotency key), so we expect
	// at most +1 invocation, never +2. Anything > 1 means step 6
	// wasn't skipped.
	if after-before > 1 {
		t.Fatalf("signer.Sign delta = %d, want <= 1 (step 6 must be skipped)", after-before)
	}
}

// ---------------------------------------------------------------------------
// WAL resume — TSA already stamped, second run must not call TSA again.
// ---------------------------------------------------------------------------

func TestGenerateVerdict_WALResume_SkipTSA(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_2t", Status: "paid"}, true)

	rep := &Report{ID: "vr_preseed_t", OrderID: "vo_2t", ReportType: "observation_only"}
	h.reports.byOrder = map[string]*Report{"vo_2t": rep}

	replayer := &attestrec.Replayer{Repo: h.wal}
	if err := replayer.Record(context.Background(), "vr_preseed_t", attestrec.ActionTSAStamped, attestrec.StatusSuccess,
		encodeTSAExternal([]byte("seed-token"), time.Unix(1_500_000_000, 0).UTC()), ""); err != nil {
		t.Fatalf("seed WAL: %v", err)
	}

	before := atomic.LoadInt32(&h.tsa.stampCalls)
	if err := h.svc.GenerateVerdict(context.Background(), "vo_2t"); err != nil {
		t.Fatalf("GenerateVerdict: %v", err)
	}
	if got := atomic.LoadInt32(&h.tsa.stampCalls) - before; got != 0 {
		t.Fatalf("tsa.Stamp delta = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// sign failure path
// ---------------------------------------------------------------------------

func TestGenerateVerdict_SignFailure(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_3", Status: "paid"}, false)
	h.signer.failErr = errors.New("kms boom")
	h.signer.failOnce.Store(true)

	err := h.svc.GenerateVerdict(context.Background(), "vo_3")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "kms sign") {
		t.Fatalf("expected 'kms sign' in error, got %v", err)
	}

	if h.orders.failedMsg == "" {
		t.Fatalf("SetFailed not called")
	}
	if h.orders.order.Status != "failed" {
		t.Fatalf("order status = %q, want failed", h.orders.order.Status)
	}
}

// ---------------------------------------------------------------------------
// TSA failure path
// ---------------------------------------------------------------------------

func TestGenerateVerdict_TSAFailure(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_4", Status: "paid"}, false)
	h.tsa.failErr = tsa.ErrUpstreamUnavailable
	h.tsa.failOnce.Store(true)

	err := h.svc.GenerateVerdict(context.Background(), "vo_4")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tsa stamp") {
		t.Fatalf("expected 'tsa stamp' in error, got %v", err)
	}
	if h.orders.order.Status != "failed" {
		t.Fatalf("order status = %q, want failed", h.orders.order.Status)
	}
}

// ---------------------------------------------------------------------------
// Archive failure path
// ---------------------------------------------------------------------------

func TestGenerateVerdict_ArchiveFailure(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_5", Status: "paid"}, true)
	h.archiver.failErr = errors.New("s3 boom")
	h.archiver.failOnce.Store(true)

	err := h.svc.GenerateVerdict(context.Background(), "vo_5")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "s3 archive") {
		t.Fatalf("expected 's3 archive' in error, got %v", err)
	}
	if h.orders.order.Status != "failed" {
		t.Fatalf("order status = %q, want failed", h.orders.order.Status)
	}
}

// ---------------------------------------------------------------------------
// Order status guard
// ---------------------------------------------------------------------------

func TestGenerateVerdict_RejectsWrongStatus(t *testing.T) {
	h := newHarness(t, &Order{ID: "vo_6", Status: "delivered"}, false)

	err := h.svc.GenerateVerdict(context.Background(), "vo_6")
	if !errors.Is(err, ErrUnexpectedOrderStatus) {
		t.Fatalf("expected ErrUnexpectedOrderStatus, got %v", err)
	}
	if atomic.LoadInt32(&h.signer.signCalls) != 0 {
		t.Fatalf("signer must not be called for ineligible order")
	}
}

// ---------------------------------------------------------------------------
// Config validation
// ---------------------------------------------------------------------------

func TestNew_PanicsOnMissingDeps(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"orders", Config{}},
		{"reports", Config{Orders: &fakeOrderRepo{}}},
		{"wal", Config{Orders: &fakeOrderRepo{}, Reports: &fakeReportRepo{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("New(%s) did not panic", tc.name)
				}
			}()
			_ = New(tc.cfg)
		})
	}
}

// ---------------------------------------------------------------------------
// TSA external encoding round-trip
// ---------------------------------------------------------------------------

func TestEncodeDecodeTSAExternal(t *testing.T) {
	tok := []byte{0x01, 0x02, 0x03, 0xff, 0xab}
	ts := time.Date(2026, 5, 17, 10, 11, 12, 345_000_000, time.UTC)
	enc := encodeTSAExternal(tok, ts)
	gotTok, gotTS, err := decodeTSAExternal(enc)
	if err != nil {
		t.Fatalf("decodeTSAExternal: %v", err)
	}
	if string(gotTok) != string(tok) {
		t.Fatalf("token mismatch: %x vs %x", gotTok, tok)
	}
	if !gotTS.Equal(ts) {
		t.Fatalf("time mismatch: %v vs %v", gotTS, ts)
	}
}

func TestDecodeTSAExternal_Malformed(t *testing.T) {
	if _, _, err := decodeTSAExternal("no-colon-here"); err == nil {
		t.Fatalf("expected error for malformed external_id")
	}
	if _, _, err := decodeTSAExternal(":2026-05-17T10:11:12Z"); err == nil {
		t.Fatalf("expected error for empty token half")
	}
	if _, _, err := decodeTSAExternal("nothex:2026-05-17T10:11:12Z"); err == nil {
		t.Fatalf("expected error for non-hex token")
	}
}

// ---------------------------------------------------------------------------
// archive external encoding round-trip
// ---------------------------------------------------------------------------

func TestEncodeSplitArchiveExternal(t *testing.T) {
	enc := encodeArchiveExternal("s3://bucket/key", "etag-abc")
	url, etag := splitArchiveExternal(enc)
	if url != "s3://bucket/key" || etag != "etag-abc" {
		t.Fatalf("round-trip mismatch: %q / %q", url, etag)
	}
	// No separator → URL only, empty etag.
	url, etag = splitArchiveExternal("plain")
	if url != "plain" || etag != "" {
		t.Fatalf("plain split mismatch: %q / %q", url, etag)
	}
}

// ---------------------------------------------------------------------------
// newReportID shape
// ---------------------------------------------------------------------------

func TestNewReportID_PrefixAndLen(t *testing.T) {
	id := newReportID()
	if !strings.HasPrefix(id, "vr_") {
		t.Fatalf("id %q missing vr_ prefix", id)
	}
	body := strings.TrimPrefix(id, "vr_")
	if len(body) != 24 {
		t.Fatalf("id body %q len = %d, want 24", body, len(body))
	}
}

func TestEncodeNodesJSON(t *testing.T) {
	if got := string(encodeNodesJSON(nil)); got != "[]" {
		t.Fatalf("nil → %q, want []", got)
	}
	if got := string(encodeNodesJSON([]string{"a", "b"})); got != `["a","b"]` {
		t.Fatalf(`["a","b"] → %q`, got)
	}
}

// TestEncodeNodesJSON_SpecialChars locks in the contract that node IDs
// containing JSON-significant or control characters must round-trip
// through json.Unmarshal cleanly. Before the encoding/json conversion
// the function wrote raw bytes inside "..." which produced syntactically
// broken JSON for quotes / backslashes / control bytes.
func TestEncodeNodesJSON_SpecialChars(t *testing.T) {
	in := []string{
		`with"quote`,
		`back\slash`,
		"tab\there",
		"newline\nhere",
		"unicode-中文",
	}
	out := encodeNodesJSON(in)
	var round []string
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("encoded JSON is not parseable: %v\nbytes=%s", err, string(out))
	}
	if len(round) != len(in) {
		t.Fatalf("round trip len = %d, want %d", len(round), len(in))
	}
	for i := range in {
		if round[i] != in[i] {
			t.Fatalf("round[%d] = %q, want %q", i, round[i], in[i])
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// hexEncode is a tiny wrapper to avoid an import cycle with the
// orchestrator's encoding/hex import (kept terse on purpose).
func hexEncode(b []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexDigits[v>>4]
		out[i*2+1] = hexDigits[v&0x0f]
	}
	return string(out)
}
