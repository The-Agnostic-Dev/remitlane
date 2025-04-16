-- name: ListActiveEndpoints :many
SELECT * FROM webhook_endpoints WHERE partner_id = $1 AND active ORDER BY id;

-- name: GetEndpoint :one
SELECT * FROM webhook_endpoints WHERE id = $1;

-- name: CreateEndpoint :one
INSERT INTO webhook_endpoints (partner_id, url, secret) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateEvent :exec
INSERT INTO webhook_events (id, partner_id, type, payout_id, payload) VALUES ($1, $2, $3, $4, $5);

-- name: CreateDelivery :one
INSERT INTO webhook_deliveries (event_id, endpoint_id) VALUES ($1, $2) RETURNING id;

-- name: GetDeliveryForSend :one
SELECT d.id, d.status, d.attempts, e.id AS event_id, e.type, e.payload,
       ep.id AS endpoint_id, ep.url, ep.secret
FROM webhook_deliveries d
JOIN webhook_events e ON e.id = d.event_id
JOIN webhook_endpoints ep ON ep.id = d.endpoint_id
WHERE d.id = $1;

-- name: RecordAttempt :exec
INSERT INTO webhook_attempts (delivery_id, status_code, duration_ms, error) VALUES ($1, $2, $3, $4);

-- name: MarkDelivered :exec
UPDATE webhook_deliveries
SET status = 'delivered', attempts = attempts + 1, last_status_code = $2, last_error = '',
    last_attempt_at = now(), delivered_at = now()
WHERE id = $1;

-- name: MarkAttemptFailed :exec
UPDATE webhook_deliveries
SET status = $2, attempts = attempts + 1, last_status_code = $3, last_error = $4, last_attempt_at = now()
WHERE id = $1;

-- name: ListDeliveriesForPayout :many
SELECT d.id, d.status, d.attempts, d.last_status_code, d.last_error, d.delivered_at, d.last_attempt_at,
       e.id AS event_id, e.type, ep.url
FROM webhook_deliveries d
JOIN webhook_events e ON e.id = d.event_id
JOIN webhook_endpoints ep ON ep.id = d.endpoint_id
WHERE e.payout_id = $1
ORDER BY d.id;

-- name: ListAttempts :many
SELECT * FROM webhook_attempts WHERE delivery_id = $1 ORDER BY id;
