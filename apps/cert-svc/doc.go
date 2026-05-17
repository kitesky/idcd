// Package certsvc is the entry package for the idcd certificate platform service.
//
// Binaries (under cmd/):
//   - server:  HTTP API server, /v1/cert/* endpoints
//   - worker:  ACME orchestrator, consumes order events from Redis Stream
//   - renewer: cron-driven renewal scheduler, enqueues renewal jobs
//
// See docs/prd/20-free-cert.md for the full module spec.
package certsvc
