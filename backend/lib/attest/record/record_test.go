package record

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- constant drift guards -------------------------------------------------

func TestActionConstants(t *testing.T) {
	cases := map[Action]string{
		ActionSigned:       "signed",
		ActionTSAStamped:   "tsa_stamped",
		ActionAnchored:     "anchored",
		ActionS3Archived:   "s3_archived",
		ActionSelfVerified: "self_verified",
		ActionRevoked:      "revoked",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("action %q != %q", got, want)
		}
	}
}

func TestStatusConstants(t *testing.T) {
	cases := map[Status]string{
		StatusPending: "pending",
		StatusSuccess: "success",
		StatusFailure: "failure",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("status %q != %q", got, want)
		}
	}
}

func TestResultConstants(t *testing.T) {
	cases := map[Result]string{
		ResultSuccess: "success",
		ResultFailure: "failure",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("result %q != %q", got, want)
		}
	}
}

func TestMaxRetries(t *testing.T) {
	if MaxRetries != 3 {
		t.Fatalf("MaxRetries = %d, want 3 (D4)", MaxRetries)
	}
}

// --- NewRecordID -----------------------------------------------------------

func TestNewRecordIDPrefixAndShape(t *testing.T) {
	id := NewRecordID()
	if !strings.HasPrefix(id, "att_") {
		t.Fatalf("id %q missing att_ prefix", id)
	}
	body := strings.TrimPrefix(id, "att_")
	if len(body) != 24 {
		t.Fatalf("id body %q len = %d, want 24", body, len(body))
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		ok := (c >= 'a' && c <= 'z') || (c >= '2' && c <= '7')
		if !ok {
			t.Fatalf("id body %q contains non-base32-lower char %q", body, c)
		}
	}
}

func TestNewRecordIDUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := NewRecordID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

// --- mock Repository -------------------------------------------------------

type mockRepo struct {
	getFn    func(ctx context.Context, reportID string, action Action) (*Record, error)
	insertFn func(ctx context.Context, r *Record) error
	updateFn func(ctx context.Context, r *Record) error
	listFn   func(ctx context.Context, reportID string) ([]*Record, error)

	getCalls    int
	insertCalls int
	updateCalls int

	lastInserted *Record
	lastUpdated  *Record
}

func (m *mockRepo) Insert(ctx context.Context, r *Record) error {
	m.insertCalls++
	m.lastInserted = r
	if m.insertFn != nil {
		return m.insertFn(ctx, r)
	}
	return nil
}

func (m *mockRepo) Get(ctx context.Context, reportID string, action Action) (*Record, error) {
	m.getCalls++
	if m.getFn != nil {
		return m.getFn(ctx, reportID, action)
	}
	return nil, ErrNotFound
}

func (m *mockRepo) Update(ctx context.Context, r *Record) error {
	m.updateCalls++
	m.lastUpdated = r
	if m.updateFn != nil {
		return m.updateFn(ctx, r)
	}
	return nil
}

func (m *mockRepo) ListByReport(ctx context.Context, reportID string) ([]*Record, error) {
	if m.listFn != nil {
		return m.listFn(ctx, reportID)
	}
	return nil, nil
}

// --- Replayer.ShouldRun ----------------------------------------------------

func TestShouldRunNotFound(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return nil, ErrNotFound
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !cont {
		t.Fatal("expected cont=true on NotFound")
	}
	if ext != "" {
		t.Fatalf("expected empty externalID, got %q", ext)
	}
}

func TestShouldRunSuccessSkips(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusSuccess, ExternalID: "kms-req-123"}, nil
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if cont {
		t.Fatal("expected cont=false on success")
	}
	if ext != "kms-req-123" {
		t.Fatalf("externalID = %q, want kms-req-123", ext)
	}
}

func TestShouldRunFailureWithRetriesLeft(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusFailure, RetryCount: 1}, nil
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !cont {
		t.Fatal("expected cont=true when retries remain")
	}
	if ext != "" {
		t.Fatalf("expected empty externalID, got %q", ext)
	}
}

func TestShouldRunFailureMaxRetries(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusFailure, RetryCount: MaxRetries}, nil
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("err = %v, want ErrMaxRetriesExceeded", err)
	}
	if cont {
		t.Fatal("expected cont=false at MaxRetries")
	}
	if ext != "" {
		t.Fatalf("expected empty externalID, got %q", ext)
	}
}

func TestShouldRunFailureOverMaxRetries(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusFailure, RetryCount: MaxRetries + 5}, nil
		},
	}
	r := &Replayer{Repo: repo}

	_, _, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("err = %v, want ErrMaxRetriesExceeded", err)
	}
}

func TestShouldRunPendingTreatedAsRetry(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusPending, RetryCount: 0}, nil
		},
	}
	r := &Replayer{Repo: repo}

	cont, _, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !cont {
		t.Fatal("pending should be re-runnable when retries remain")
	}
}

func TestShouldRunPendingAtMaxRetries(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: StatusPending, RetryCount: MaxRetries}, nil
		},
	}
	r := &Replayer{Repo: repo}

	_, _, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Fatalf("err = %v, want ErrMaxRetriesExceeded", err)
	}
}

func TestShouldRunUnknownStatus(t *testing.T) {
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return &Record{Status: Status("bogus")}, nil
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if err == nil {
		t.Fatal("expected error on unknown status")
	}
	if cont {
		t.Fatal("expected cont=false on error")
	}
	if ext != "" {
		t.Fatalf("expected empty externalID, got %q", ext)
	}
}

func TestShouldRunRepoErrorPropagates(t *testing.T) {
	sentinel := errors.New("db down")
	repo := &mockRepo{
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return nil, sentinel
		},
	}
	r := &Replayer{Repo: repo}

	cont, ext, err := r.ShouldRun(context.Background(), "vr_1", ActionSigned)
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if cont {
		t.Fatal("expected cont=false on repo err")
	}
	if ext != "" {
		t.Fatalf("expected empty externalID on repo err, got %q", ext)
	}
}

// --- Replayer.Record -------------------------------------------------------

func TestRecordInsertsTerminalSuccess(t *testing.T) {
	repo := &mockRepo{}
	r := &Replayer{Repo: repo}

	if err := r.Record(context.Background(), "vr_1", ActionSigned, StatusSuccess, "ext-1", ""); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("insertCalls = %d, want 1", repo.insertCalls)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("updateCalls = %d, want 0", repo.updateCalls)
	}
	got := repo.lastInserted
	if got == nil {
		t.Fatal("nil inserted record")
	}
	if got.Status != StatusSuccess {
		t.Errorf("Status = %q", got.Status)
	}
	if got.Result != ResultSuccess {
		t.Errorf("Result = %q, want success", got.Result)
	}
	if got.ExternalID != "ext-1" {
		t.Errorf("ExternalID = %q", got.ExternalID)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set on terminal write")
	}
	if !strings.HasPrefix(got.ID, "att_") {
		t.Errorf("ID = %q missing att_ prefix", got.ID)
	}
	if got.ReportID != "vr_1" {
		t.Errorf("ReportID = %q", got.ReportID)
	}
	if got.Action != ActionSigned {
		t.Errorf("Action = %q", got.Action)
	}
}

func TestRecordInsertsTerminalFailureFillsErrorDetail(t *testing.T) {
	repo := &mockRepo{}
	r := &Replayer{Repo: repo}

	if err := r.Record(context.Background(), "vr_1", ActionSigned, StatusFailure, "", "kms timeout"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.lastInserted.Result != ResultFailure {
		t.Errorf("Result = %q, want failure", repo.lastInserted.Result)
	}
	if repo.lastInserted.ErrorDetail != "kms timeout" {
		t.Errorf("ErrorDetail = %q", repo.lastInserted.ErrorDetail)
	}
}

func TestRecordDuplicateFallsBackToUpdate(t *testing.T) {
	existing := &Record{
		ID:          "att_old",
		ReportID:    "vr_1",
		Action:      ActionSigned,
		Status:      StatusFailure,
		RetryCount:  1,
		ErrorDetail: "previous fail",
	}
	repo := &mockRepo{
		insertFn: func(ctx context.Context, _ *Record) error {
			return ErrDuplicateAction
		},
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return existing, nil
		},
	}
	r := &Replayer{Repo: repo}

	if err := r.Record(context.Background(), "vr_1", ActionSigned, StatusSuccess, "ext-2", ""); err != nil {
		t.Fatalf("err = %v", err)
	}
	if repo.insertCalls != 1 {
		t.Fatalf("insertCalls = %d, want 1", repo.insertCalls)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("updateCalls = %d, want 1", repo.updateCalls)
	}
	if repo.lastUpdated == nil {
		t.Fatal("nil updated record")
	}
	if repo.lastUpdated.Status != StatusSuccess {
		t.Errorf("Status = %q after update", repo.lastUpdated.Status)
	}
	if repo.lastUpdated.Result != ResultSuccess {
		t.Errorf("Result = %q after update", repo.lastUpdated.Result)
	}
	if repo.lastUpdated.ExternalID != "ext-2" {
		t.Errorf("ExternalID = %q after update", repo.lastUpdated.ExternalID)
	}
	if repo.lastUpdated.RetryCount != 2 {
		t.Errorf("RetryCount = %d after update, want 2", repo.lastUpdated.RetryCount)
	}
	if repo.lastUpdated.CompletedAt == nil {
		t.Error("CompletedAt should be set after update")
	}
}

func TestRecordDuplicateGetErrorPropagates(t *testing.T) {
	sentinel := errors.New("get blew up")
	repo := &mockRepo{
		insertFn: func(ctx context.Context, _ *Record) error {
			return ErrDuplicateAction
		},
		getFn: func(ctx context.Context, _ string, _ Action) (*Record, error) {
			return nil, sentinel
		},
	}
	r := &Replayer{Repo: repo}

	err := r.Record(context.Background(), "vr_1", ActionSigned, StatusSuccess, "ext", "")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("updateCalls = %d, want 0 when Get failed", repo.updateCalls)
	}
}

func TestRecordInsertOtherErrorPropagates(t *testing.T) {
	sentinel := errors.New("disk full")
	repo := &mockRepo{
		insertFn: func(ctx context.Context, _ *Record) error {
			return sentinel
		},
	}
	r := &Replayer{Repo: repo}

	err := r.Record(context.Background(), "vr_1", ActionSigned, StatusSuccess, "ext", "")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
	if repo.getCalls != 0 {
		t.Fatalf("getCalls = %d, want 0 on non-duplicate error", repo.getCalls)
	}
}

func TestRecordRejectsPendingStatus(t *testing.T) {
	repo := &mockRepo{}
	r := &Replayer{Repo: repo}

	err := r.Record(context.Background(), "vr_1", ActionSigned, StatusPending, "", "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	if repo.insertCalls != 0 {
		t.Fatalf("insertCalls = %d, want 0 on invalid transition", repo.insertCalls)
	}
}

func TestRecordRejectsUnknownStatus(t *testing.T) {
	repo := &mockRepo{}
	r := &Replayer{Repo: repo}

	err := r.Record(context.Background(), "vr_1", ActionSigned, Status("weird"), "", "")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v, want ErrInvalidTransition", err)
	}
	if repo.insertCalls != 0 {
		t.Fatal("should not call Insert on invalid transition")
	}
}

// --- statusToResult helper -------------------------------------------------

func TestStatusToResult(t *testing.T) {
	if statusToResult(StatusSuccess) != ResultSuccess {
		t.Error("success → success")
	}
	if statusToResult(StatusFailure) != ResultFailure {
		t.Error("failure → failure")
	}
	// Anything else collapses to failure (defensive default; Record
	// itself rejects non-terminal statuses before reaching this).
	if statusToResult(StatusPending) != ResultFailure {
		t.Error("pending → failure (default)")
	}
}

// --- sentinel error identity ----------------------------------------------

func TestSentinelErrorsAreDistinct(t *testing.T) {
	errs := []error{
		ErrDuplicateAction,
		ErrNotFound,
		ErrMaxRetriesExceeded,
		ErrInvalidTransition,
	}
	for i, a := range errs {
		for j, b := range errs {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("errs[%d] should not Is errs[%d]", i, j)
			}
		}
	}
}
