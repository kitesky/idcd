// Package attest is the Evidence/Attestation service for idcd
// (attest.idcd.com).
//
// Subcommands:
//
//   - cmd/server    — HTTP API. Hosts the public /verify endpoint and
//                     the admin / billing-webhook surfaces. Stateless.
//   - cmd/generator — Verdict Generator Worker. Drives the 10-step
//                     WAL pipeline (PRD §3.2). Crash-safe via
//                     attestation_record idempotency (D4).
//   - cmd/verifier  — Self-Verify Worker. Runs in a distinct VPC
//                     subnet (D6) and uses ONLY the public /verify
//                     HTTP path so the "sign" and "check" code paths
//                     stay independent.
//
// Subpackages:
//
//   - internal/config  — env-driven runtime config + backend selection.
//   - internal/repo    — PostgreSQL data access for the five idcd_attest
//                        tables (verdict_order / verdict_report /
//                        attestation_record / tsa_response /
//                        key_ceremony_log).
//   - internal/service — Verdict orchestrator + Replayer wiring.
//   - internal/handler — HTTP handlers (verify / health / webhook).
package attest
