package store

import (
	"context"
	"sort"

	"github.com/remitlane/remitlane/internal/money"
)

// Balance is a partner's position in one currency.
type Balance struct {
	PartnerID string
	Currency  money.Currency
	Available int64
	Held      int64
}

// Total is available plus held.
func (b Balance) Total() int64 { return b.Available + b.Held }

const balanceQuery = `
SELECT a.partner_id, a.currency, a.kind, COALESCE(SUM(e.amount), 0)::text
FROM ledger_accounts a
LEFT JOIN ledger_entries e ON e.account_id = a.id
WHERE a.partner_id IS NOT NULL AND a.kind IN ('available', 'held')`

// PartnerBalances returns a partner's balances, one per currency.
func (s *Store) PartnerBalances(ctx context.Context, partnerID string) ([]Balance, error) {
	return s.balances(ctx, balanceQuery+" AND a.partner_id = $1 GROUP BY a.partner_id, a.currency, a.kind", partnerID)
}

// AllBalances returns every partner's balances.
func (s *Store) AllBalances(ctx context.Context) ([]Balance, error) {
	return s.balances(ctx, balanceQuery+" GROUP BY a.partner_id, a.currency, a.kind")
}

func (s *Store) balances(ctx context.Context, q string, args ...any) ([]Balance, error) {
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type key struct{ partner, cur string }
	acc := map[key]*Balance{}
	for rows.Next() {
		var partner, cur, kind, sum string
		if err := rows.Scan(&partner, &cur, &kind, &sum); err != nil {
			return nil, err
		}
		c := money.Currency(cur)
		minor, err := money.DecimalToMinor(sum, c)
		if err != nil {
			return nil, err
		}
		k := key{partner, cur}
		b, ok := acc[k]
		if !ok {
			b = &Balance{PartnerID: partner, Currency: c}
			acc[k] = b
		}
		if kind == "available" {
			b.Available = minor
		} else {
			b.Held = minor
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Balance, 0, len(acc))
	for _, b := range acc {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PartnerID != out[j].PartnerID {
			return out[i].PartnerID < out[j].PartnerID
		}
		return out[i].Currency < out[j].Currency
	})
	return out, nil
}
