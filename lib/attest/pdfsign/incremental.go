package pdfsign

// This file is a reserved home for low-level PDF incremental-update
// helpers (manual ByteRange / xref / trailer composition) needed when
// the upstream github.com/digitorus/pdfsign library cannot satisfy a
// specific PAdES edge case — for example:
//
//   - Inputs that already carry one or more signatures and require
//     a second incremental section (LTV "long-term validation" sweeps).
//   - PDF 2.0 ByteRange semantics that differ from PDF 1.7.
//   - Embedding DSS (Document Security Store) dictionaries for
//     PAdES-LT / PAdES-LTA upgrade paths beyond the B-T baseline.
//
// MVP scope (S2 launch — see docs/prd/18 §3.2): we use digitorus/pdfsign
// for the full B-T pipeline and keep this file as a stub. Helpers will
// land here when the verdict generator surfaces concrete edge cases.
//
// Intentionally empty so test coverage tooling does not penalise an
// unimplemented surface.
