-- name: GetReceiverConfig :one
SELECT * FROM receiver_config WHERE slug = $1;

-- name: UpsertReceiverConfig :exec
INSERT INTO receiver_config (slug, respond_with, note) VALUES ($1, $2, $3)
ON CONFLICT (slug) DO UPDATE SET respond_with = EXCLUDED.respond_with, note = EXCLUDED.note;

-- name: ListReceiverConfig :many
SELECT * FROM receiver_config ORDER BY slug;

-- name: InsertReceiverHit :exec
INSERT INTO receiver_hits (slug, event_id, event_type, payout_id, signature, responded, body)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListReceiverHits :many
SELECT * FROM receiver_hits
WHERE (sqlc.narg(slug)::text IS NULL OR slug = sqlc.narg(slug))
  AND (sqlc.narg(payout_id)::text IS NULL OR payout_id = sqlc.narg(payout_id))
ORDER BY id DESC
LIMIT sqlc.arg(max_rows);
