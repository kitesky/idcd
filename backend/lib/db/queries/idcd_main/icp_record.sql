-- name: GetICPRecordByDomain :one
SELECT * FROM icp.records WHERE domain = $1;

-- name: UpsertICPRecord :one
INSERT INTO icp.records (domain, icp_number, company, filing_type, filed_at, source, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (domain) DO UPDATE
SET icp_number  = EXCLUDED.icp_number,
    company     = EXCLUDED.company,
    filing_type = EXCLUDED.filing_type,
    filed_at    = EXCLUDED.filed_at,
    source      = EXCLUDED.source,
    note        = EXCLUDED.note,
    updated_at  = NOW()
RETURNING *;

-- name: DeleteICPRecord :exec
DELETE FROM icp.records WHERE domain = $1;

-- name: ListICPRecords :many
SELECT * FROM icp.records
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2;

-- name: CountICPRecords :one
SELECT COUNT(*) FROM icp.records;
