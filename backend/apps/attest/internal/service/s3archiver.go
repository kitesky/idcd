package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3PutObjectAPI is the subset of the S3 client surface the archiver
// needs. Defined as an interface so tests can swap in a fake without
// reaching for real network calls (MinIO / LocalStack are an
// integration-test concern, not a unit-test one).
type s3PutObjectAPI interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3ArchiverConfig fully describes an S3+ObjectLock archiver. The
// generator cmd populates this from ATTEST_S3_* env vars; see
// apps/attest/cmd/generator/s3archiver.go for the env contract.
type S3ArchiverConfig struct {
	// Bucket is the destination bucket. Required. The bucket MUST have
	// Object Lock enabled at creation time (S3 does not allow enabling
	// it after the fact).
	Bucket string
	// KeyPrefix is prepended to every Archive(key) call. Optional; pass
	// "" for no prefix. A trailing slash is preserved if supplied.
	KeyPrefix string
	// ObjectLockMode applied to every PutObject. Must be "COMPLIANCE"
	// (default, recommended for D4 WORM) or "GOVERNANCE".
	ObjectLockMode s3types.ObjectLockMode
	// RetainDuration is added to time.Now() to compute
	// ObjectLockRetainUntilDate. Use the bucket's default by passing 0.
	// docs/prd/18 §3.3 mandates 10y = 3650d.
	RetainDuration time.Duration
	// Now is the clock used to compute RetainUntilDate; nil means
	// time.Now. Exposed for deterministic tests.
	Now func() time.Time
}

// s3Archiver implements the Archiver interface against an S3-compatible
// PutObject API. The constructor enforces all field-level validation;
// Archive is a thin wrapper that materialises a single PutObject call
// with Object Lock attributes pinned per docs/prd/18 §3.3.
type s3Archiver struct {
	api s3PutObjectAPI
	cfg S3ArchiverConfig
}

// NewS3Archiver builds an Archiver bound to the supplied client and
// config. It validates that Bucket is non-empty and ObjectLockMode is
// one of the two S3 enum values; KeyPrefix and RetainDuration are
// optional.
//
// The client is injected (rather than constructed here) so the calling
// cmd retains control over the credential chain, retry policy, and any
// S3-compatible endpoint override. See apps/attest/cmd/generator for
// the production wiring.
func NewS3Archiver(api s3PutObjectAPI, cfg S3ArchiverConfig) (Archiver, error) {
	if api == nil {
		return nil, errors.New("s3archiver: nil PutObject client")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, errors.New("s3archiver: bucket is required")
	}
	switch cfg.ObjectLockMode {
	case s3types.ObjectLockModeCompliance, s3types.ObjectLockModeGovernance:
		// ok
	case "":
		// Default to COMPLIANCE — the WORM mode mandated by D4.
		cfg.ObjectLockMode = s3types.ObjectLockModeCompliance
	default:
		return nil, fmt.Errorf("s3archiver: invalid ObjectLockMode %q (want COMPLIANCE or GOVERNANCE)", cfg.ObjectLockMode)
	}
	if cfg.RetainDuration < 0 {
		return nil, fmt.Errorf("s3archiver: RetainDuration must be >= 0, got %s", cfg.RetainDuration)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &s3Archiver{api: api, cfg: cfg}, nil
}

// Archive uploads pdf to s3://{bucket}/{prefix}{key} with Object Lock
// metadata applied. The returned URL is the canonical s3:// form (NOT
// presigned) because the Self-Verify Worker resolves it via the SDK
// using its own credentials. The returned etag has the surrounding
// quotes that the S3 API includes stripped, so it can be stored as-is
// in attestation_record.external_id.
func (a *s3Archiver) Archive(ctx context.Context, key string, pdf []byte) (string, string, error) {
	if key == "" {
		return "", "", errors.New("s3archiver: key is required")
	}
	fullKey := a.cfg.KeyPrefix + key

	in := &s3.PutObjectInput{
		Bucket:         aws.String(a.cfg.Bucket),
		Key:            aws.String(fullKey),
		Body:           bytes.NewReader(pdf),
		ContentType:    aws.String("application/pdf"),
		ObjectLockMode: a.cfg.ObjectLockMode,
	}
	if a.cfg.RetainDuration > 0 {
		t := a.cfg.Now().Add(a.cfg.RetainDuration).UTC()
		in.ObjectLockRetainUntilDate = &t
	}

	out, err := a.api.PutObject(ctx, in)
	if err != nil {
		return "", "", fmt.Errorf("s3archiver: PutObject s3://%s/%s: %w", a.cfg.Bucket, fullKey, err)
	}

	etag := ""
	if out.ETag != nil {
		// AWS returns ETag with surrounding double quotes (it's the raw
		// HTTP header value). Strip them so downstream consumers see a
		// clean hex string.
		etag = strings.Trim(*out.ETag, `"`)
	}
	url := fmt.Sprintf("s3://%s/%s", a.cfg.Bucket, fullKey)
	return url, etag, nil
}
