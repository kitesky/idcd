package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3Get implements s3GetObjectAPI for unit tests.
type fakeS3Get struct {
	gotIn *s3.GetObjectInput
	body  []byte
	err   error
	calls int
}

func (f *fakeS3Get) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.calls++
	f.gotIn = in
	if f.err != nil {
		return nil, f.err
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(f.body))}, nil
}

func TestParseS3URL_HappyPath(t *testing.T) {
	bucket, key, err := parseS3URL("s3://idcd-verdict/verdict/vr_42.pdf")
	if err != nil {
		t.Fatalf("parseS3URL: %v", err)
	}
	if bucket != "idcd-verdict" {
		t.Errorf("bucket = %q, want %q", bucket, "idcd-verdict")
	}
	if key != "verdict/vr_42.pdf" {
		t.Errorf("key = %q, want %q", key, "verdict/vr_42.pdf")
	}
}

func TestParseS3URL_DeepPrefix(t *testing.T) {
	bucket, key, err := parseS3URL("s3://bkt/a/b/c/d.pdf")
	if err != nil {
		t.Fatalf("parseS3URL: %v", err)
	}
	if bucket != "bkt" || key != "a/b/c/d.pdf" {
		t.Errorf("bucket=%q key=%q", bucket, key)
	}
}

func TestParseS3URL_Errors(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"missing scheme", "https://foo/bar", "not an s3"},
		{"no slash", "s3://onlybucket", "missing key"},
		{"empty bucket", "s3:///some/key", "empty bucket"},
		{"empty key", "s3://bkt/", "empty key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseS3URL(tc.url)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestS3Fetcher_Fetch_HappyPath(t *testing.T) {
	pdf := []byte("%PDF-1.7 verdict bytes")
	fake := &fakeS3Get{body: pdf}
	f := newS3Fetcher(fake)

	got, err := f.Fetch(context.Background(), "s3://idcd-verdict/verdict/vr_42.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !bytes.Equal(got, pdf) {
		t.Errorf("body mismatch: got %q want %q", got, pdf)
	}
	if fake.calls != 1 {
		t.Errorf("calls = %d, want 1", fake.calls)
	}
	if fake.gotIn == nil {
		t.Fatal("GetObject not called")
	}
	if fake.gotIn.Bucket == nil || *fake.gotIn.Bucket != "idcd-verdict" {
		t.Errorf("bucket = %v", fake.gotIn.Bucket)
	}
	if fake.gotIn.Key == nil || *fake.gotIn.Key != "verdict/vr_42.pdf" {
		t.Errorf("key = %v", fake.gotIn.Key)
	}
}

func TestS3Fetcher_Fetch_BadURL(t *testing.T) {
	fake := &fakeS3Get{}
	f := newS3Fetcher(fake)

	_, err := f.Fetch(context.Background(), "http://nope.example/x.pdf")
	if err == nil {
		t.Fatal("expected error for non-s3 URL")
	}
	if fake.calls != 0 {
		t.Errorf("GetObject should not be called for invalid URL; calls=%d", fake.calls)
	}
}

func TestS3Fetcher_Fetch_GetObjectError(t *testing.T) {
	want := errors.New("access denied")
	fake := &fakeS3Get{err: want}
	f := newS3Fetcher(fake)

	_, err := f.Fetch(context.Background(), "s3://bkt/k.pdf")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want wraps %v", err, want)
	}
	if !strings.Contains(err.Error(), "fetch s3://bkt/k.pdf") {
		t.Errorf("err = %v, want contains canonical prefix", err)
	}
}

func TestS3Fetcher_Fetch_MaxBytesCap(t *testing.T) {
	// 10 bytes payload, fetcher capped at 4 bytes — must truncate.
	fake := &fakeS3Get{body: []byte("0123456789")}
	f := newS3Fetcher(fake)
	f.maxBytes = 4

	got, err := f.Fetch(context.Background(), "s3://bkt/k.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != "0123" {
		t.Errorf("body = %q, want %q (truncated to maxBytes)", got, "0123")
	}
}

func TestS3Fetcher_Fetch_NegativeMaxBytesUsesDefault(t *testing.T) {
	fake := &fakeS3Get{body: []byte("hello")}
	f := newS3Fetcher(fake)
	f.maxBytes = -1 // falls back to defaultS3MaxBytes

	got, err := f.Fetch(context.Background(), "s3://bkt/k.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
}

// errReader returns err on every Read so we exercise the ReadAll error
// path in Fetch.
type errReader struct{ err error }

func (e *errReader) Read(_ []byte) (int, error) { return 0, e.err }

type bodyErrS3 struct{ readErr error }

func (b *bodyErrS3) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{Body: io.NopCloser(&errReader{err: b.readErr})}, nil
}

func TestS3Fetcher_Fetch_BodyReadError(t *testing.T) {
	want := errors.New("network blip")
	f := newS3Fetcher(&bodyErrS3{readErr: want})

	_, err := f.Fetch(context.Background(), "s3://bkt/k.pdf")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want wraps %v", err, want)
	}
	if !strings.Contains(err.Error(), "fetch s3://bkt/k.pdf") {
		t.Errorf("err = %v, want contains canonical prefix", err)
	}
}

func TestNewS3FetcherFromEnv_MissingRegion(t *testing.T) {
	t.Setenv("ATTEST_S3_REGION", "")
	_, err := newS3FetcherFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error when ATTEST_S3_REGION is unset")
	}
	if !strings.Contains(err.Error(), "ATTEST_S3_REGION") {
		t.Errorf("err = %v, want mention of ATTEST_S3_REGION", err)
	}
}

func TestNewS3FetcherFromEnv_WithRegion(t *testing.T) {
	t.Setenv("ATTEST_S3_REGION", "us-east-1")
	t.Setenv("ATTEST_S3_ENDPOINT", "http://localhost:9000")
	// Disable any local AWS config / credential lookups that would fail
	// in a hermetic CI box.
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	f, err := newS3FetcherFromEnv(context.Background())
	if err != nil {
		t.Fatalf("newS3FetcherFromEnv: %v", err)
	}
	if f == nil || f.api == nil {
		t.Fatal("fetcher / api not initialised")
	}
}
