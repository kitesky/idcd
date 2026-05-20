-- billing.sql — sqlc queries for billing tables.
-- subscriptions / invoices / payments 一律使用 provider + ext_* 命名（聚合支付）。

-- name: GetSubscriptionByUserID :one
SELECT id, user_id, plan, status, provider, ext_sub_id,
       current_period_start, current_period_end, cancel_at, created_at, updated_at
FROM subscriptions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateSubscription :one
INSERT INTO subscriptions (
  id, user_id, plan, status, provider, ext_sub_id,
  current_period_start, current_period_end, created_at, updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
)
RETURNING *;

-- name: UpdateSubscriptionStatus :one
UPDATE subscriptions
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: ListInvoicesByUser :many
SELECT id, user_id, subscription_id, provider, ext_invoice_id,
       amount_cents, currency, status, paid_at, created_at
FROM invoices
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateInvoice :one
INSERT INTO invoices (
  id, user_id, subscription_id, provider, ext_invoice_id,
  amount_cents, currency, status, paid_at, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW()
)
RETURNING *;

-- name: CreatePayment :one
INSERT INTO payments (
  id, user_id, invoice_id, provider, ext_txn_id,
  amount_cents, currency, status, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, NOW()
)
RETURNING *;

-- name: UpdatePaymentStatus :one
UPDATE payments
SET status = $2
WHERE id = $1
RETURNING *;
