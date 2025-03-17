-- name: GetAccountByCode :one
SELECT * FROM ledger_accounts WHERE code = $1;

-- name: UpsertAccount :one
INSERT INTO ledger_accounts (code, partner_id, kind, currency)
VALUES ($1, $2, $3, $4)
ON CONFLICT (code) DO UPDATE SET code = EXCLUDED.code
RETURNING *;

-- name: InsertTxn :one
INSERT INTO ledger_txns (kind, payout_id, memo) VALUES ($1, $2, $3) RETURNING id, created_at;

-- name: InsertEntry :exec
INSERT INTO ledger_entries (txn_id, account_id, amount) VALUES ($1, $2, $3);

-- name: ListTxnsForPayout :many
SELECT t.id, t.kind, t.memo, t.created_at, a.code, a.currency, e.amount
FROM ledger_txns t
JOIN ledger_entries e ON e.txn_id = t.id
JOIN ledger_accounts a ON a.id = e.account_id
WHERE t.payout_id = $1
ORDER BY t.id, e.id;

-- name: CountTxnsForPayout :one
SELECT count(*) FROM ledger_txns WHERE payout_id = $1 AND kind = $2;
