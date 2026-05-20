package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeS3 captures the PutObject input and returns a canned response or
// error. It implements s3PutObjectAPI so NewS3Archiver accepts it.
type fakeS3 struct {
	gotIn  *s3.PutObjectInput
	gotBody []byte
	etag   string
	err    error
	calls  int
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.calls++
	f.gotIn = in
	if in.Body != nil {
		b, err := io.ReadAll(in.Body)
		if err != nil {
			return nil, err
		}
		f.gotBody = b
	}
	if f.err != nil {
		return nil, f.err
	}
	out := &s3.PutObjectOutput{}
	if f.etag != "" {
		out.ETag = &f.etag
	}
	return out, nil
}

func TestS3Archiver_DefaultsAndHappyPath(t *testing.T) {
	fixedNow := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	fake := &fakeS3{etag: `"abc123def456"`}

	a, err := NewS3Archiver(fake, S3ArchiverConfig{
		Bucket:         "idcd-verdict",
		KeyPrefix:      "verdict/",
		RetainDuration: 3650 * 24 * time.Hour,
		Now:            func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}

	pdf := []byte("%PDF-1.7 test bytes")
	url, etag, err := a.Archive(context.Background(), "vr_42.pdf", pdf)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	// URL shape
	want := "s3://idcd-verdict/verdict/vr_42.pdf"
	if url != want {
		t.Errorf("url = %q, want %q", url, want)
	}
	// ETag stripped of quotes
	if etag != "abc123def456" {
		t.Errorf("etag = %q, want %q", etag, "abc123def456")
	}
	// PutObject input
	if fake.gotIn == nil {
		t.Fatal("PutObject was not called")
	}
	if got := strDeref(fake.gotIn.Bucket); got != "idcd-verdict" {
		t.Errorf("Bucket = %q, want %q", got, "idcd-verdict")
	}
	if got := strDeref(fake.gotIn.Key); got != "verdict/vr_42.pdf" {
		t.Errorf("Key = %q, want %q", got, "verdict/vr_42.pdf")
	}
	if got := strDeref(fake.gotIn.ContentType); got != "application/pdf" {
		t.Errorf("ContentType = %q, want %q", got, "application/pdf")
	}
	if fake.gotIn.ObjectLockMode != s3types.ObjectLockModeCompliance {
		t.Errorf("ObjectLockMode = %q, want %q", fake.gotIn.ObjectLockMode, s3types.ObjectLockModeCompliance)
	}
	if fake.gotIn.ObjectLockRetainUntilDate == nil {
		t.Fatal("ObjectLockRetainUntilDate is nil, expected ~now+3650d")
	}
	wantUntil := fixedNow.Add(3650 * 24 * time.Hour)
	if !fake.gotIn.ObjectLockRetainUntilDate.Equal(wantUntil) {
		t.Errorf("RetainUntilDate = %s, want %s", fake.gotIn.ObjectLockRetainUntilDate, wantUntil)
	}
	if string(fake.gotBody) != string(pdf) {
		t.Errorf("body bytes mismatch: got %q want %q", fake.gotBody, pdf)
	}
}

func TestS3Archiver_GovernanceMode(t *testing.T) {
	fake := &fakeS3{etag: `"x"`}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{
		Bucket:         "b",
		ObjectLockMode: s3types.ObjectLockModeGovernance,
	})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	if _, _, err := a.Archive(context.Background(), "k", []byte("p")); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if fake.gotIn.ObjectLockMode != s3types.ObjectLockModeGovernance {
		t.Errorf("mode = %q, want GOVERNANCE", fake.gotIn.ObjectLockMode)
	}
}

func TestS3Archiver_NoBucketDefaultRetention(t *testing.T) {
	// RetainDuration = 0 means rely on bucket default.
	fake := &fakeS3{}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	if _, _, err := a.Archive(context.Background(), "k", []byte("p")); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if fake.gotIn.ObjectLockRetainUntilDate != nil {
		t.Errorf("expected nil RetainUntilDate when RetainDuration=0, got %s", fake.gotIn.ObjectLockRetainUntilDate)
	}
}

func TestS3Archiver_NoPrefix(t *testing.T) {
	fake := &fakeS3{}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	url, _, err := a.Archive(context.Background(), "raw.pdf", []byte("p"))
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if url != "s3://b/raw.pdf" {
		t.Errorf("url = %q, want s3://b/raw.pdf", url)
	}
	if strDeref(fake.gotIn.Key) != "raw.pdf" {
		t.Errorf("Key = %q, want raw.pdf", strDeref(fake.gotIn.Key))
	}
}

func TestS3Archiver_PutObjectErrorWrapped(t *testing.T) {
	fake := &fakeS3{err: errors.New("AccessDenied")}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b", KeyPrefix: "p/"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	_, _, err = a.Archive(context.Background(), "k", []byte("x"))
	if err == nil {
		t.Fatal("expected error from PutObject failure")
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Errorf("error %q should wrap original AccessDenied", err)
	}
	if !strings.Contains(err.Error(), "s3://b/p/k") {
		t.Errorf("error %q should reference s3 URL for debugging", err)
	}
}

func TestS3Archiver_EmptyKeyRejected(t *testing.T) {
	fake := &fakeS3{}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	if _, _, err := a.Archive(context.Background(), "", []byte("x")); err == nil {
		t.Fatal("expected error for empty key")
	}
	if fake.calls != 0 {
		t.Errorf("PutObject should not be called when key is empty (calls=%d)", fake.calls)
	}
}

func TestS3Archiver_MissingETagIsBlank(t *testing.T) {
	fake := &fakeS3{} // no etag set
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	_, etag, err := a.Archive(context.Background(), "k", []byte("p"))
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if etag != "" {
		t.Errorf("etag = %q, want empty when S3 returns nil ETag", etag)
	}
}

// --- constructor validation ---

func TestNewS3Archiver_NilClient(t *testing.T) {
	if _, err := NewS3Archiver(nil, S3ArchiverConfig{Bucket: "b"}); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestNewS3Archiver_MissingBucket(t *testing.T) {
	if _, err := NewS3Archiver(&fakeS3{}, S3ArchiverConfig{}); err == nil {
		t.Fatal("expected error for empty bucket")
	}
	if _, err := NewS3Archiver(&fakeS3{}, S3ArchiverConfig{Bucket: "   "}); err == nil {
		t.Fatal("expected error for whitespace bucket")
	}
}

func TestNewS3Archiver_InvalidMode(t *testing.T) {
	_, err := NewS3Archiver(&fakeS3{}, S3ArchiverConfig{Bucket: "b", ObjectLockMode: s3types.ObjectLockMode("NONSENSE")})
	if err == nil {
		t.Fatal("expected error for invalid ObjectLockMode")
	}
	if !strings.Contains(err.Error(), "COMPLIANCE") || !strings.Contains(err.Error(), "GOVERNANCE") {
		t.Errorf("error %q should mention valid modes", err)
	}
}

func TestNewS3Archiver_NegativeRetention(t *testing.T) {
	if _, err := NewS3Archiver(&fakeS3{}, S3ArchiverConfig{Bucket: "b", RetainDuration: -time.Hour}); err == nil {
		t.Fatal("expected error for negative RetainDuration")
	}
}

func TestNewS3Archiver_DefaultsModeToCompliance(t *testing.T) {
	fake := &fakeS3{}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b"})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	if _, _, err := a.Archive(context.Background(), "k", []byte("p")); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if fake.gotIn.ObjectLockMode != s3types.ObjectLockModeCompliance {
		t.Errorf("default mode = %q, want COMPLIANCE", fake.gotIn.ObjectLockMode)
	}
}

func TestNewS3Archiver_DefaultsNowToTimeNow(t *testing.T) {
	// When Now is nil, RetainUntilDate should be ~time.Now()+RetainDuration.
	fake := &fakeS3{}
	a, err := NewS3Archiver(fake, S3ArchiverConfig{Bucket: "b", RetainDuration: time.Hour})
	if err != nil {
		t.Fatalf("NewS3Archiver: %v", err)
	}
	before := time.Now()
	if _, _, err := a.Archive(context.Background(), "k", []byte("p")); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	after := time.Now()
	got := fake.gotIn.ObjectLockRetainUntilDate
	if got == nil {
		t.Fatal("ObjectLockRetainUntilDate nil")
	}
	lower := before.Add(time.Hour).Add(-time.Second)
	upper := after.Add(time.Hour).Add(time.Second)
	if got.Before(lower) || got.After(upper) {
		t.Errorf("RetainUntilDate %s not within [%s, %s]", got, lower, upper)
	}
}

// helper
func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
