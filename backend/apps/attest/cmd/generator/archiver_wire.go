package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/kite365/idcd/apps/attest/internal/service"
)

// wireArchiver picks the verdict-archive backend at startup. Defaults to
// a local-filesystem archiver suitable for S2-MVP / pre-prod; when
// ATTEST_ARCHIVER_BACKEND=s3 is set the S3+ObjectLock implementation in
// s3archiver.go takes over.
//
// Env vars consumed:
//
//	ATTEST_ARCHIVER_BACKEND   "local" (default) | "s3"
//	ATTEST_LOCAL_ARCHIVE_DIR  directory for the local backend
//	                          (default defaultArchiveDir)
//
// S3-specific env vars are documented next to NewS3Archiver in
// s3archiver.go so the local backend keeps a flat surface.
func wireArchiver(log *slog.Logger) (service.Archiver, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("ATTEST_ARCHIVER_BACKEND")))
	if backend == "" {
		backend = "local"
	}

	switch backend {
	case "local":
		dir := strings.TrimSpace(os.Getenv("ATTEST_LOCAL_ARCHIVE_DIR"))
		if dir == "" {
			dir = defaultArchiveDir
		}
		log.Info("attest-generator: archiver wired", "type", "local", "dir", dir)
		return service.NewLocalArchiver(dir), nil
	case "s3":
		a, info, err := newS3ArchiverFromEnv()
		if err != nil {
			return nil, err
		}
		log.Info("attest-generator: archiver wired",
			"type", "s3",
			"bucket", info.Bucket,
			"region", info.Region,
			"object_lock_mode", info.ObjectLockMode,
		)
		return a, nil
	default:
		// Surface a clear error rather than silently falling back —
		// silent fallback can hide a typo'd env var in prod.
		log.Error("attest-generator: unknown ATTEST_ARCHIVER_BACKEND", "value", backend)
		return nil, errUnknownArchiverBackend(backend)
	}
}

type errUnknownArchiverBackend string

func (e errUnknownArchiverBackend) Error() string {
	return "unknown ATTEST_ARCHIVER_BACKEND: " + string(e)
}
