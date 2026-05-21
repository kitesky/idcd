// S3 fetcher for the independent attest-verify service.
//
// D6 note: this file is a deliberate copy of the s3fetcher logic that
// also lives in apps/attest/cmd/verifier/s3fetcher.go. The two binaries
// MUST NOT share Go packages — copying ~80 lines of plumbing is the cost
// of compile-time independence (and is checked by go.mod: this module
// must not require github.com/kite365/idcd/apps/attest).
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3GetObjectAPI interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// s3Fetcher resolves s3://{bucket}/{key} URLs via GetObject. The bucket
// comes from the URL (not configuration) so a single fetcher can read
// from any bucket reachable via the configured AWS credentials.
type s3Fetcher struct {
	api      s3GetObjectAPI
	maxBytes int64
}

// defaultS3MaxBytes mirrors httpFetcher's cap so an archive URL swap can't
// be used to OOM the verifier.
const defaultS3MaxBytes = 64 << 20

func newS3Fetcher(api s3GetObjectAPI) *s3Fetcher {
	return &s3Fetcher{api: api, maxBytes: defaultS3MaxBytes}
}

// newS3FetcherFromRegion builds a production fetcher with an explicit region
// and optional endpoint. region is required; endpoint is optional and only
// used for S3-compatible backends (MinIO/LocalStack).
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

// parseS3URL splits s3://{bucket}/{key} into its parts.
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
