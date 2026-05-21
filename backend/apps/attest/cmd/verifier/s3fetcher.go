package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3GetObjectAPI is the subset of the S3 client surface s3Fetcher uses.
// Defined as an interface so tests can swap in a fake without doing real
// network IO. Mirrors the s3PutObjectAPI pattern in
// apps/attest/internal/service/s3archiver.go.
type s3GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// s3Fetcher implements selfverify.PDFFetcher for s3:// URLs. It expects
// URLs of the form s3://{bucket}/{key}; the bucket in the URL is honoured
// directly so the same fetcher can read from any bucket reachable via the
// configured credentials.
//
// MaxBytes caps a single object read to defend against a malicious /
// corrupted archive. Verdict PDFs are typically <10 MiB; we default to 64.
type s3Fetcher struct {
	api      s3GetObjectAPI
	maxBytes int64
}

// defaultS3MaxBytes mirrors the cap in httpFetcher (main.go) so the two
// fetchers behave consistently when an attacker swaps an archive URL.
const defaultS3MaxBytes = 64 << 20

// newS3Fetcher constructs an s3Fetcher with an injected client. The
// caller owns credential / endpoint configuration so tests stay
// dependency-free.
func newS3Fetcher(api s3GetObjectAPI) *s3Fetcher {
	return &s3Fetcher{api: api, maxBytes: defaultS3MaxBytes}
}

// newS3FetcherFromRegion builds a production fetcher with an explicit region
// and optional endpoint. Use this when S3 config comes from the YAML config
// (P1-8 migration). The bucket comes from the s3:// URL on each Fetch call.
func newS3FetcherFromRegion(ctx context.Context, region, endpoint string) (*s3Fetcher, error) {
	if region == "" {
		return nil, fmt.Errorf("s3 fetcher: region is required")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("s3 fetcher: load AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = true
		}
	})
	return newS3Fetcher(client), nil
}

// newS3FetcherFromEnv builds a production fetcher from ATTEST_S3_REGION /
// ATTEST_S3_ENDPOINT env vars. Kept for backward compat and tests; prefer
// newS3FetcherFromRegion when the config is already loaded.
func newS3FetcherFromEnv(ctx context.Context) (*s3Fetcher, error) {
	region := strings.TrimSpace(os.Getenv("ATTEST_S3_REGION"))
	if region == "" {
		return nil, fmt.Errorf("s3 fetcher: ATTEST_S3_REGION is required")
	}
	endpoint := strings.TrimSpace(os.Getenv("ATTEST_S3_ENDPOINT"))
	return newS3FetcherFromRegion(ctx, region, endpoint)
}

// parseS3URL splits s3://{bucket}/{key} into its parts. Returns an error
// when the scheme, bucket, or key is missing.
func parseS3URL(raw string) (bucket, key string, err error) {
	if !strings.HasPrefix(raw, "s3://") {
		return "", "", fmt.Errorf("s3 fetcher: not an s3:// URL: %q", raw)
	}
	rest := strings.TrimPrefix(raw, "s3://")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return "", "", fmt.Errorf("s3 fetcher: missing key in %q", raw)
	}
	bucket = rest[:slash]
	key = rest[slash+1:]
	if bucket == "" {
		return "", "", fmt.Errorf("s3 fetcher: empty bucket in %q", raw)
	}
	if key == "" {
		return "", "", fmt.Errorf("s3 fetcher: empty key in %q", raw)
	}
	return bucket, key, nil
}

// Fetch resolves an s3:// URL via GetObject and returns the body bytes,
// bounded by maxBytes. Error wrapping uses the contract documented in the
// task: fetch s3://{bucket}/{key}: %w.
func (f *s3Fetcher) Fetch(ctx context.Context, pdfURL string) ([]byte, error) {
	bucket, key, err := parseS3URL(pdfURL)
	if err != nil {
		return nil, err
	}
	out, err := f.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("fetch s3://%s/%s: %w", bucket, key, err)
	}
	defer out.Body.Close()

	cap := f.maxBytes
	if cap <= 0 {
		cap = defaultS3MaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(out.Body, cap))
	if err != nil {
		return nil, fmt.Errorf("fetch s3://%s/%s: %w", bucket, key, err)
	}
	return body, nil
}
