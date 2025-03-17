-- name: GetPartner :one
SELECT * FROM partners WHERE id = $1;

-- name: GetPartnerBySlug :one
SELECT * FROM partners WHERE slug = $1;

-- name: ListPartners :many
SELECT * FROM partners ORDER BY name;

-- name: CreatePartner :one
INSERT INTO partners (id, slug, name, country, contact_email, daily_limit_usd_minor)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePartnerLimit :exec
UPDATE partners SET daily_limit_usd_minor = $2 WHERE id = $1;

-- name: GetAPIKeyByHash :one
SELECT k.id, k.partner_id, k.prefix, k.environment, p.status AS partner_status
FROM api_keys k
JOIN partners p ON p.id = k.partner_id
WHERE k.key_hash = $1 AND k.revoked_at IS NULL;

-- name: TouchAPIKey :exec
UPDATE api_keys SET last_used_at = now() WHERE id = $1;

-- name: CreateAPIKey :exec
INSERT INTO api_keys (partner_id, prefix, key_hash, label, environment)
VALUES ($1, $2, $3, $4, $5);

-- name: ListAPIKeys :many
SELECT id, prefix, label, environment, last_used_at, revoked_at, created_at
FROM api_keys WHERE partner_id = $1 ORDER BY created_at;
