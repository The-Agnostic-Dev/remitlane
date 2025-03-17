-- name: GetOpsUserByEmail :one
SELECT * FROM ops_users WHERE email = lower($1);

-- name: GetOpsUser :one
SELECT * FROM ops_users WHERE id = $1;

-- name: CreateOpsUser :exec
INSERT INTO ops_users (email, name, role, password_hash) VALUES (lower($1), $2, $3, $4);

-- name: CreateSession :exec
INSERT INTO console_sessions (token_hash, user_id, csrf_token, expires_at) VALUES ($1, $2, $3, $4);

-- name: GetSession :one
SELECT s.token_hash, s.csrf_token, s.expires_at, u.id AS user_id, u.email, u.name, u.role
FROM console_sessions s
JOIN ops_users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM console_sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM console_sessions WHERE expires_at <= now();
