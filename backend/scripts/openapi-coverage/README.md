# openapi-coverage

P0-5 (ARCHITECTURE-REVIEW-2026-05-21.md) contract drift gate.

Compares every `path + method` in `docs/prd/16-api-spec.yaml` against
the actual `chi.Router` registrations under `backend/apps/api/` and
`backend/apps/attest/`. Fails CI when new drift appears in either
direction:

- **spec-only**  — handler is missing (drop spec entry or add handler)
- **code-only**  — spec is out of date (add to spec or whitelist)

`cert-svc` is intentionally not scanned: `apps/api` mounts it via the
wildcard `r.Handle("/v1/cert/*", proxy)`, which the helper records as
a wildcard mount and uses to mark `/v1/cert/*` spec paths as covered.

## Running

```bash
# Check (CI default)
bash backend/scripts/check-openapi-coverage.sh

# Update baseline after intentionally accepting / fixing drift
bash backend/scripts/check-openapi-coverage.sh --write-baseline
```

Exit codes: `0` = no new drift, `1` = new drift, `2` = script error.

## Baseline

`baseline.txt` snapshots the current accepted drift. The check only
fails on entries *not* in the baseline, so a one-time PR landing this
gate doesn't trip on the 218 pre-existing drift entries. As handlers
get added / spec gets synced, regenerate the baseline to ratchet it
down. The goal is an empty baseline.

## Scope (not a full contract test)

This is the "B-path" weak gate from the architecture review:
1. Path + method existence only — request schemas / response shapes
   are not validated.
2. AST-walked, doesn't run the binary. A handler that registers
   correctly but panics at runtime still passes.
3. Doesn't follow `r chi.Router` parameters across function bodies
   (only inline `r.Route(prefix, func(r chi.Router) { ... })` closures
   are tracked). Same-file `mountXxx(r, deps)` helpers are why
   `cert-svc` is omitted from scan roots.

The "A-path" — running the actual server and asserting per-route
behavior matches the spec — is a separate task; see the architecture
review for context.
