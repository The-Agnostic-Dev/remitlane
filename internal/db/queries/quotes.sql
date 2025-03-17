-- name: CreateQuote :one
INSERT INTO quotes (
    id, partner_id, source_currency, dest_currency, channel,
    source_amount_minor, dest_amount_minor, fee_minor, rate, mid_rate, spread_bps, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetQuoteForPartner :one
SELECT * FROM quotes WHERE id = $1 AND partner_id = $2;

-- name: LockQuote :one
SELECT * FROM quotes WHERE id = $1 AND partner_id = $2 FOR UPDATE;

-- name: MarkQuoteUsed :exec
UPDATE quotes SET used_at = now() WHERE id = $1;
