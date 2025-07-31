package store

import (
	"context"
	"time"
)

// ProviderStats summarises a provider's recent traffic for the health page.
type ProviderStats struct {
	Provider      string
	Total         int
	Completed     int
	Failed        int
	InFlight      int
	AvgSeconds    float64
	Webhooks      int
	BadSignatures int
	LastWebhookAt *time.Time
}

// ProviderHealth returns stats per provider since t.
func (s *Store) ProviderHealth(ctx context.Context, since time.Time) ([]ProviderStats, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH p AS (
			SELECT provider,
			       count(*) AS total,
			       count(*) FILTER (WHERE status IN ('completed', 'reversed')) AS completed,
			       count(*) FILTER (WHERE status = 'failed') AS failed,
			       count(*) FILTER (WHERE status IN ('pending', 'submitted', 'processing')) AS in_flight,
			       COALESCE(avg(EXTRACT(EPOCH FROM completed_at - created_at))
			                FILTER (WHERE status = 'completed'), 0)::float8 AS avg_seconds
			FROM payouts WHERE created_at >= $1 GROUP BY provider
		), w AS (
			SELECT provider, count(*) AS hooks,
			       count(*) FILTER (WHERE NOT signature_ok) AS bad,
			       max(received_at) AS last_at
			FROM provider_events WHERE received_at >= $1 GROUP BY provider
		)
		SELECT names.provider, COALESCE(p.total, 0), COALESCE(p.completed, 0), COALESCE(p.failed, 0),
		       COALESCE(p.in_flight, 0), COALESCE(p.avg_seconds, 0), COALESCE(w.hooks, 0), COALESCE(w.bad, 0), w.last_at
		FROM (VALUES ('swiftpay'), ('momobridge')) AS names(provider)
		LEFT JOIN p ON p.provider = names.provider
		LEFT JOIN w ON w.provider = names.provider
		ORDER BY names.provider DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProviderStats
	for rows.Next() {
		var st ProviderStats
		if err := rows.Scan(&st.Provider, &st.Total, &st.Completed, &st.Failed, &st.InFlight, &st.AvgSeconds,
			&st.Webhooks, &st.BadSignatures, &st.LastWebhookAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// StatusCounts counts payouts per status created since t.
func (s *Store) StatusCounts(ctx context.Context, since time.Time) (map[string]int, error) {
	rows, err := s.Pool.Query(ctx, `SELECT status, count(*) FROM payouts WHERE created_at >= $1 GROUP BY status`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// LedgerTxn is a ledger transaction with its entries, for the console.
type LedgerTxn struct {
	ID        int64
	Kind      string
	PayoutID  string
	Memo      string
	CreatedAt time.Time
	Entries   []LedgerEntry
}

// LedgerEntry is one leg of a transaction.
type LedgerEntry struct {
	Account  string
	Currency string
	Amount   string
}

// RecentLedger returns the newest n transactions, optionally for one account.
func (s *Store) RecentLedger(ctx context.Context, account string, n int) ([]LedgerTxn, error) {
	rows, err := s.Pool.Query(ctx, `
		WITH t AS (
			SELECT DISTINCT t.id, t.kind, COALESCE(t.payout_id, '') AS payout_id, t.memo, t.created_at
			FROM ledger_txns t
			JOIN ledger_entries e ON e.txn_id = t.id
			JOIN ledger_accounts a ON a.id = e.account_id
			WHERE $1 = '' OR a.code = $1
			ORDER BY t.id DESC LIMIT $2
		)
		SELECT t.id, t.kind, t.payout_id, t.memo, t.created_at, a.code, a.currency, e.amount::text
		FROM t
		JOIN ledger_entries e ON e.txn_id = t.id
		JOIN ledger_accounts a ON a.id = e.account_id
		ORDER BY t.id DESC, e.id`, account, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LedgerTxn
	for rows.Next() {
		var t LedgerTxn
		var e LedgerEntry
		if err := rows.Scan(&t.ID, &t.Kind, &t.PayoutID, &t.Memo, &t.CreatedAt, &e.Account, &e.Currency, &e.Amount); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].ID != t.ID {
			out = append(out, t)
		}
		last := &out[len(out)-1]
		last.Entries = append(last.Entries, e)
	}
	return out, rows.Err()
}
