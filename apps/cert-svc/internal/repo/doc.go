// Package repo holds the Postgres data-access layer for cert.* tables.
//
// S1 W1 is intentionally empty — the concrete repositories (orders,
// dns_credentials, certs, renewal_jobs, audit_logs) land alongside the
// ACME state machine in W2. The directory exists now so go.work can
// resolve `apps/cert-svc/internal/repo` once the next agent adds files
// without restructuring imports.
package repo
