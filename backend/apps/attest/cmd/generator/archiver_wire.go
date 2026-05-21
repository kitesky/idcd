package main

import (
	"log/slog"
	"strings"

	"github.com/kite365/idcd/apps/attest/internal/config"
	"github.com/kite365/idcd/apps/attest/internal/service"
)

// wireArchiver picks the verdict-archive backend at startup. Defaults to
// a local-filesystem archiver suitable for S2-MVP / pre-prod; when
// cfg.ArchiverBackend == "s3" the S3+ObjectLock implementation takes over.
//
// Config fields consumed (P1-8 migration — previously env vars):
//
//	cfg.ArchiverBackend    "local" (default) | "s3"
//	cfg.LocalArchiveDir    directory for the local backend (default defaultArchiveDir)
//
// S3-specific fields are in cfg.S3* and documented next to newS3ArchiverFromConfig.
func wireArchiver(cfg *config.Config, log *slog.Logger) (service.Archiver, error) {
	backend := strings.ToLower(strings.TrimSpace(cfg.ArchiverBackend))
	if backend == "" {
		backend = "local"
	}

	switch backend {
	case "local":
		dir := strings.TrimSpace(cfg.LocalArchiveDir)
		if dir == "" {
			dir = defaultArchiveDir
		}
		log.Info("attest-generator: archiver wired", "type", "local", "dir", dir)
		return service.NewLocalArchiver(dir), nil
	case "s3":
		a, info, err := newS3ArchiverFromConfig(cfg)
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
		log.Error("attest-generator: unknown archiver_backend", "value", backend)
		return nil, errUnknownArchiverBackend(backend)
	}
}

type errUnknownArchiverBackend string

func (e errUnknownArchiverBackend) Error() string {
	return "unknown archiver_backend: " + string(e)
}
