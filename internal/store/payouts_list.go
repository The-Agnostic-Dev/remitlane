package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/db"
)

// PartnerPayoutFilter selects a partner's payouts for the API list endpoint.
type PartnerPayoutFilter struct {
	PartnerID string
	Status    string
	Reference string
	BatchID   string
	// Before is the created_at of the last payout on the previous page.
	Before time.Time
	Limit  int
}

// ListPartnerPayouts returns one page of payouts, newest first, and whether
// another page follows.
func (s *Store) ListPartnerPayouts(ctx context.Context, f PartnerPayoutFilter) ([]db.Payout, bool, error) {
	var b strings.Builder
	args := []any{f.PartnerID}
	b.WriteString("SELECT " + payoutColumns + " FROM payouts p WHERE p.partner_id = $1")
	add := func(cond string, v any) {
		args = append(args, v)
		fmt.Fprintf(&b, " AND "+cond, len(args))
	}
	if f.Status != "" {
		add("p.status = $%d", f.Status)
	}
	if f.Reference != "" {
		add("p.reference = $%d", f.Reference)
	}
	if f.BatchID != "" {
		add("p.batch_id = $%d", f.BatchID)
	}
	if !f.Before.IsZero() {
		add("p.created_at < $%d", f.Before)
	}
	args = append(args, f.Limit+1)
	fmt.Fprintf(&b, " ORDER BY p.created_at DESC, p.id DESC LIMIT $%d", len(args))

	rows, err := s.Pool.Query(ctx, b.String(), args...)
	if err != nil {
		return nil, false, err
	}
	payouts, err := pgx.CollectRows(rows, pgx.RowToStructByPos[db.Payout])
	if err != nil {
		return nil, false, err
	}
	more := len(payouts) > f.Limit
	if more {
		payouts = payouts[:f.Limit]
	}
	return payouts, more, nil
}
