package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/db"
)

// ConsoleFilter selects payouts for the ops console list.
type ConsoleFilter struct {
	PartnerID string
	Status    string
	Currency  string
	Provider  string
	Search    string
	Page      int
	PerPage   int
}

// ConsolePayout is a payout row with its partner's name.
type ConsolePayout struct {
	db.Payout
	PartnerName string
}

// ListConsolePayouts returns a page of payouts across partners and the
// total number matching the filter.
func (s *Store) ListConsolePayouts(ctx context.Context, f ConsoleFilter) ([]ConsolePayout, int, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.PartnerID != "" {
		add("p.partner_id = $%d", f.PartnerID)
	}
	if f.Status != "" {
		add("p.status = $%d", f.Status)
	}
	if f.Currency != "" {
		add("p.dest_currency = $%d", f.Currency)
	}
	if f.Provider != "" {
		add("p.provider = $%d", f.Provider)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		args = append(args, q, "%"+strings.ToLower(q)+"%")
		where = append(where, fmt.Sprintf("(p.id = $%d OR lower(p.reference) LIKE $%d OR lower(p.recipient_name) LIKE $%d)",
			len(args)-1, len(args), len(args)))
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM payouts p"+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if f.PerPage <= 0 {
		f.PerPage = 25
	}
	if f.Page < 1 {
		f.Page = 1
	}
	args = append(args, f.PerPage, (f.Page-1)*f.PerPage)
	q := fmt.Sprintf("SELECT %s, pr.name FROM payouts p JOIN partners pr ON pr.id = p.partner_id%s"+
		" ORDER BY p.created_at DESC, p.id DESC LIMIT $%d OFFSET $%d", payoutColumns, cond, len(args)-1, len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []ConsolePayout
	for rows.Next() {
		var c ConsolePayout
		p := &c.Payout
		if err := rows.Scan(&p.ID, &p.PartnerID, &p.QuoteID, &p.Reference, &p.Status, &p.Channel, &p.Provider,
			&p.ProviderRef, &p.SourceCurrency, &p.SourceAmountMinor, &p.FeeMinor, &p.DestCurrency,
			&p.DestAmountMinor, &p.UsdAmountMinor, &p.Rate, &p.RecipientName, &p.RecipientCountry,
			&p.BankCode, &p.AccountNumber, &p.Msisdn, &p.Narration, &p.FailureCode, &p.FailureMessage,
			&p.CreatedAt, &p.UpdatedAt, &p.SubmittedAt, &p.CompletedAt, &p.BatchID, &c.PartnerName); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// RecentPayouts returns the newest payouts across all partners.
func (s *Store) RecentPayouts(ctx context.Context, n int) ([]ConsolePayout, error) {
	rows, _, err := s.ListConsolePayouts(ctx, ConsoleFilter{Page: 1, PerPage: n})
	return rows, err
}

// PayoutsByIDs loads payouts by id, in no particular order.
func (s *Store) PayoutsByIDs(ctx context.Context, ids []string) ([]db.Payout, error) {
	rows, err := s.Pool.Query(ctx, "SELECT "+payoutColumns+" FROM payouts p WHERE p.id = ANY($1)", ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[db.Payout])
}
