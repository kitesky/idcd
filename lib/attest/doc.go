// Package attest is the umbrella for idcd's Evidence / Attestation
// subsystem (S2 — see docs/prd/18-evidence-and-attestation.md).
//
// Subpackages:
//
//   - record    — attestation_record WAL types + sentinel errors (D4).
//   - tsa       — RFC3161 TSA client + DigiCert / GlobalSign adapters.
//   - sign      — KMS sign / GetPublicKey wrapper, AWS + Aliyun adapters
//                 with idempotency tokens for crash-safe re-runs (D4).
//   - pdfsign   — PDF generation + PAdES B-T signing helper.
//
// The umbrella module has no exported API of its own; depend on the
// subpackages directly.
package attest
