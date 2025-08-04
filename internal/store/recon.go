package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ReconRow is one payout on the reconciliation report.
type ReconRow struct {
	PayoutID     string
	PartnerName  string
	Reference    string
	Provider     string
	ProviderRef  string
	Status       string
	Channel      string
	DestCurrency string
	DestMinor    int64
	SourceCur    string
	SourceMinor  int64
	FeeMinor     int64
	USDMinor     int64
	CreatedAt    time.Time
	CompletedAt  pgtype.Timestamptz
}

// ReconRows returns the payouts created in [from, to).
func (s *Store) ReconRows(ctx context.Context, from, to time.Time) ([]ReconRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT p.id, pr.name, p.reference, p.provider, p.provider_ref, p.status, p.channel,
		       p.dest_currency, p.dest_amount_minor, p.source_currency, p.source_amount_minor,
		       p.fee_minor, p.usd_amount_minor, p.created_at, p.completed_at
		FROM payouts p
		JOIN partners pr ON pr.id = p.partner_id
		WHERE p.created_at >= $1 AND p.created_at < $2
		ORDER BY p.created_at, p.id`, from, to)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[ReconRow])
}
