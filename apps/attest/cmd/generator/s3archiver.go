package main

import (
	"fmt"

	"github.com/kite365/idcd/apps/attest/internal/service"
)

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
// ATTEST_S3_* environment variables. Returns an error until the real
// implementation lands (S2 W7+) so an operator who sets
// ATTEST_ARCHIVER_BACKEND=s3 prematurely gets a clear "not implemented"
// message instead of a silent local fallback.
//
// Env vars (documented here, consumed by the constructor):
//
//	ATTEST_S3_BUCKET              required
//	ATTEST_S3_REGION              required
//	ATTEST_S3_OBJECT_LOCK_MODE    "COMPLIANCE" (default) | "GOVERNANCE"
//	ATTEST_S3_OBJECT_LOCK_DAYS    int days (default 3650 = 10y)
//	ATTEST_S3_KEY_PREFIX          optional prefix for the object key
//	ATTEST_S3_ENDPOINT            optional S3-compatible endpoint
//	AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN
//	                              standard AWS SDK credential chain
func newS3ArchiverFromEnv() (service.Archiver, s3ArchiverInfo, error) {
	return nil, s3ArchiverInfo{}, fmt.Errorf("s3 archiver: not implemented yet (set ATTEST_ARCHIVER_BACKEND=local for now)")
}
