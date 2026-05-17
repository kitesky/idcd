package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/kite365/idcd/apps/attest/internal/service"
)

// defaultObjectLockDays defines the WORM retention floor (docs/prd/18
// §3.3). Override via ATTEST_S3_OBJECT_LOCK_DAYS only with explicit
// legal sign-off.
const defaultObjectLockDays = 3650 // 10 years

// s3ArchiverInfo captures fields the wiring layer logs when the S3
// backend is selected. The S3 implementation lives in
// apps/attest/internal/service/s3archiver.go; this file only adapts the
// constructor signature to wireArchiver and exposes the env-var contract
// in one place.
type s3ArchiverInfo struct {
	Bucket         string
	Region         string
	ObjectLockMode string
}

// newS3ArchiverFromEnv constructs an S3+ObjectLock archiver from the
// ATTEST_S3_* environment variables and the standard AWS credential
// chain.
//
// Env vars (documented here, consumed by the constructor):
//
//	ATTEST_S3_BUCKET              required
//	ATTEST_S3_REGION              required
//	ATTEST_S3_OBJECT_LOCK_MODE    "COMPLIANCE" (default) | "GOVERNANCE"
//	ATTEST_S3_OBJECT_LOCK_DAYS    int days (default 3650 = 10y)
//	ATTEST_S3_KEY_PREFIX          optional prefix for the object key
//	ATTEST_S3_ENDPOINT            optional S3-compatible endpoint
//	                              (MinIO / LocalStack)
//	AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN
//	                              standard AWS SDK credential chain
func newS3ArchiverFromEnv() (service.Archiver, s3ArchiverInfo, error) {
	bucket := strings.TrimSpace(os.Getenv("ATTEST_S3_BUCKET"))
	if bucket == "" {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: ATTEST_S3_BUCKET is required")
	}
	region := strings.TrimSpace(os.Getenv("ATTEST_S3_REGION"))
	if region == "" {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: ATTEST_S3_REGION is required")
	}

	modeStr := strings.ToUpper(strings.TrimSpace(os.Getenv("ATTEST_S3_OBJECT_LOCK_MODE")))
	if modeStr == "" {
		modeStr = "COMPLIANCE"
	}
	var mode s3types.ObjectLockMode
	switch modeStr {
	case "COMPLIANCE":
		mode = s3types.ObjectLockModeCompliance
	case "GOVERNANCE":
		mode = s3types.ObjectLockModeGovernance
	default:
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: invalid ATTEST_S3_OBJECT_LOCK_MODE %q (want COMPLIANCE or GOVERNANCE)", modeStr)
	}

	days := defaultObjectLockDays
	if v := strings.TrimSpace(os.Getenv("ATTEST_S3_OBJECT_LOCK_DAYS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: ATTEST_S3_OBJECT_LOCK_DAYS = %q: %w", v, err)
		}
		if n <= 0 {
			return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: ATTEST_S3_OBJECT_LOCK_DAYS must be > 0, got %d", n)
		}
		days = n
	}

	prefix := os.Getenv("ATTEST_S3_KEY_PREFIX") // preserve leading/trailing whitespace? no — but allow trailing /
	prefix = strings.TrimLeft(prefix, " \t")

	endpoint := strings.TrimSpace(os.Getenv("ATTEST_S3_ENDPOINT"))

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
			// S3-compatible endpoints (MinIO / LocalStack) typically
			// require path-style addressing.
			o.UsePathStyle = true
		}
	})

	a, err := service.NewS3Archiver(client, service.S3ArchiverConfig{
		Bucket:         bucket,
		KeyPrefix:      prefix,
		ObjectLockMode: mode,
		RetainDuration: time.Duration(days) * 24 * time.Hour,
	})
	if err != nil {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: %w", err)
	}

	return a, s3ArchiverInfo{
		Bucket:         bucket,
		Region:         region,
		ObjectLockMode: string(mode),
	}, nil
}
