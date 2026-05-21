# attest-verify — Independent Self-Verify Service

**D6 compliance**: This service exists to satisfy DECISIONS.md §D6. It is a
completely independent Go module with no shared code with `apps/attest`.

## Why independence matters

The Self-Verify Worker's value depends entirely on it NOT sharing code or
state with the signature generator. If a bug in the generator also exists in
the verifier, self-verification passes silently and the failure goes
undetected until a real user or auditor reports it.

This module achieves independence through:

- **Separate Go module** (`github.com/kite365/idcd/apps/attest-verify`) — the
  Go toolchain prevents accidental imports from `apps/attest` at build time.
- **Separate process / Docker container** — independent restart, resource
  limits, and crash domain.
- **Separate KMS client** — this service holds NO signing key material. It
  calls the public `POST /verify` endpoint and relies on `apps/attest` to
  do the cryptographic verification on its own independent KMS connection.
- **Separate log table** (`idcd_attest.self_verify_log`) — results are written
  to a table this service owns, not to `verdict_report.self_verify_status`
  (which is owned by `apps/attest`).
- **Public API only** — the only link between this service and `apps/attest`
  is the HTTP call to `POST attest.idcd.com/verify`. No internal RPC,
  no shared memory, no shared Redis streams.

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `ATTEST_VERIFIER_DB_DSN` | *(required)* | PostgreSQL DSN pointing at idcd DB |
| `ATTEST_VERIFIER_VERIFY_ENDPOINT` | `https://attest.idcd.com/verify` | Public verify URL |
| `ATTEST_VERIFIER_BIND_ADDR` | `:8090` | HTTP server bind address |
| `ATTEST_VERIFIER_POLL_INTERVAL` | `5m` | How often to poll for new records |
| `ATTEST_VERIFIER_BATCH_SIZE` | `20` | Max records per poll tick |
| `ATTEST_VERIFIER_ENV` | `development` | Environment label |
| `ATTEST_VERIFIER_LOG_LEVEL` | `info` | Logging level (debug/info/warn/error) |

## How it works

1. Every `ATTEST_VERIFIER_POLL_INTERVAL` the service queries
   `idcd_attest.attestation_record` for rows where:
   - `action = 's3_archived'` — the PDF has been archived and is fetchable.
   - `status = 'success'` — the archival step completed.
   - No corresponding row exists in `idcd_attest.self_verify_log` — this
     service has not yet verified it.

2. For each matched record, the service:
   a. Fetches the PDF from `verdict_report.pdf_url`.
   b. POSTs it as a multipart upload to `ATTEST_VERIFIER_VERIFY_ENDPOINT`.
   c. Parses the `VerifyResponse` JSON.
   d. Optionally cross-checks `content_sha256` against the stored
      `verdict_report.content_hash`.
   e. Writes a row to `idcd_attest.self_verify_log` with status
      `pass`, `fail`, or `error`.

3. Failures are logged at WARN level. The service does NOT update
   `verdict_report.self_verify_status` — that column is owned by
   `apps/attest/internal/selfverify`.

## HTTP endpoints

| Path | Method | Description |
|---|---|---|
| `/healthz` | GET | Liveness probe — always returns `ok` |
| `/readyz` | GET | Readiness probe — pings DB; returns 503 if unavailable |

## Deployment requirements (D6)

- Run as a separate Docker container from `attest-server` and `attest-generator`.
- Ideally place in a **separate VPC subnet** from the attest service.
  The only required network path is HTTPS egress to `attest.idcd.com/verify`
  and a connection to the shared PostgreSQL instance.
- Do NOT mount the same KMS credentials as the generator. This service has
  no KMS key material whatsoever — verification is delegated to the HTTP API.
- Set `ATTEST_VERIFIER_VERIFY_ENDPOINT` to the **public** URL, never an
  internal loopback. The point is to exercise the same code path a real
  auditor would use.

See `docs/RUNBOOKS/attest-verify-deploy.md` for step-by-step deployment.

## Running locally

```bash
# Requires: local attest-server on :8080, PostgreSQL with idcd_attest schema
export ATTEST_VERIFIER_DB_DSN="postgresql://idcd_dev:password@localhost:5432/idcd_dev"
export ATTEST_VERIFIER_VERIFY_ENDPOINT="http://localhost:8080/verify"
cd backend/apps/attest-verify
go run ./cmd/verifier/
```

## Running tests

```bash
cd backend/apps/attest-verify
go test ./...
```

The `internal/poller` tests use `httptest.Server` to mock the verify endpoint
and in-memory stubs for the DB interfaces — no real database or KMS required.
