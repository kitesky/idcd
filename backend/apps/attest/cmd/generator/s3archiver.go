package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/service"
)

// defaultObjectLockDays defines the WORM retention floor (docs/prd/18
// §3.3). Override via attest.s3.object_lock_days in YAML (or
// ATTEST_S3_OBJECT_LOCK_DAYS env var) only with explicit legal sign-off.
const defaultObjectLockDays = 3650 // 10 years

// s3ArchiverInfo captures fields the wiring layer logs when the S3
// backend is selected. The S3 implementation lives in
// apps/attest/internal/service/s3archiver.go; this file only adapts the
// constructor signature to wireArchiver and exposes the config contract.
type s3ArchiverInfo struct {
	Bucket         string
	Region         string
	ObjectLockMode string
}

// newS3ArchiverFromConfig constructs an S3+ObjectLock archiver from the
// config fields (P1-8 migration). Falls back to the standard AWS
// credential chain (env vars AWS_* / IRSA / instance profile).
//
// Config fields consumed:
//
//	cfg.S3Bucket              required
//	cfg.S3Region              required
//	cfg.S3ObjectLockMode      "COMPLIANCE" (default) | "GOVERNANCE"
//	cfg.S3ObjectLockDays      int days (default 3650 = 10y)
//	cfg.S3KeyPrefix           optional object key prefix
//	cfg.S3Endpoint            optional S3-compatible endpoint (MinIO/LocalStack)
func newS3ArchiverFromConfig(cfg *config.Config) (service.Archiver, s3ArchiverInfo, error) {
	bucket := strings.TrimSpace(cfg.S3Bucket)
	if bucket == "" {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: attest.s3.bucket (or ATTEST_S3_BUCKET) is required")
	}
	region := strings.TrimSpace(cfg.S3Region)
	if region == "" {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: attest.s3.region (or ATTEST_S3_REGION) is required")
	}

	modeStr := strings.ToUpper(strings.TrimSpace(cfg.S3ObjectLockMode))
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
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: invalid object_lock_mode %q (want COMPLIANCE or GOVERNANCE)", modeStr)
	}

	days := cfg.S3ObjectLockDays
	if days <= 0 {
		days = defaultObjectLockDays
	}

	prefix := strings.TrimLeft(cfg.S3KeyPrefix, " \t")
	endpoint := strings.TrimSpace(cfg.S3Endpoint)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
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
