-- name: GetIdempotency :one
SELECT * FROM idempotency_keys WHERE partner_id = $1 AND key = $2;

-- name: SaveIdempotency :exec
INSERT INTO idempotency_keys (partner_id, key, request_hash, response_status, response_body)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (partner_id, key) DO NOTHING;

-- name: InsertProviderEvent :exec
INSERT INTO provider_events (provider, reference, status, signature_ok, outcome, body)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListProviderEvents :many
SELECT * FROM provider_events WHERE provider = $1 ORDER BY id DESC LIMIT $2;

-- name: InsertOutbox :exec
INSERT INTO outbox_messages (channel, recipient, subject, body) VALUES ($1, $2, $3, $4);

-- name: ListOutbox :many
SELECT * FROM outbox_messages ORDER BY id DESC LIMIT $1;

-- name: InsertReconRun :exec
INSERT INTO recon_runs (business_date, payout_count, usd_total_minor, triggered_by)
VALUES ($1, $2, $3, $4);

-- name: ListReconRuns :many
SELECT * FROM recon_runs ORDER BY id DESC LIMIT $1;

-- name: InsertAlert :execrows
INSERT INTO alert_log (partner_id, kind, day) VALUES ($1, $2, $3)
ON CONFLICT (partner_id, kind, day) DO NOTHING;
