package store

import (
	"context"
	"time"
)

// UsedUSD sums the USD value of a partner's payouts created in [from, to).
// Failed payouts give their funds back, so they do not count.
func (s *Store) UsedUSD(ctx context.Context, partnerID string, from, to time.Time) (int64, error) {
	var used int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(usd_amount_minor), 0)::bigint
		FROM payouts
		WHERE partner_id = $1 AND created_at >= $2 AND created_at < $3 AND status <> 'failed'`,
		partnerID, from, to).Scan(&used)
	return used, err
}
