-- name: CreatePayout :one
INSERT INTO payouts (
    id, partner_id, quote_id, batch_id, reference, status, channel, provider,
    source_currency, source_amount_minor, fee_minor, dest_currency, dest_amount_minor,
    usd_amount_minor, rate, recipient_name, recipient_country, bank_code, account_number,
    msisdn, narration
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21
)
RETURNING *;

-- name: GetPayout :one
SELECT * FROM payouts WHERE id = $1;

-- name: LockPayout :one
SELECT * FROM payouts WHERE id = $1 FOR UPDATE;

-- name: GetPayoutForPartner :one
SELECT * FROM payouts WHERE id = $1 AND partner_id = $2;

-- name: GetPayoutByProviderRef :one
SELECT * FROM payouts WHERE provider = $1 AND provider_ref = $2;

-- name: SetPayoutStatus :exec
UPDATE payouts
SET status = $2,
    failure_code = $3,
    failure_message = $4,
    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'reversed') THEN now() ELSE completed_at END,
    updated_at = now()
WHERE id = $1;

-- name: SetProviderRef :exec
UPDATE payouts
SET provider_ref = $2, submitted_at = COALESCE(submitted_at, now()), updated_at = now()
WHERE id = $1;

-- name: InsertPayoutEvent :exec
INSERT INTO payout_events (payout_id, status, source, detail) VALUES ($1, $2, $3, $4);

-- name: ListPayoutEvents :many
SELECT * FROM payout_events WHERE payout_id = $1 ORDER BY id;

-- name: ListStuckPayouts :many
SELECT * FROM payouts
WHERE status IN ('submitted', 'processing')
  AND updated_at < $1
ORDER BY updated_at
LIMIT $2;

-- name: TouchPayout :exec
UPDATE payouts SET updated_at = now() WHERE id = $1;

-- name: CreateBatch :one
INSERT INTO payout_batches (id, partner_id, reference, item_count)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetBatchForPartner :one
SELECT * FROM payout_batches WHERE id = $1 AND partner_id = $2;
