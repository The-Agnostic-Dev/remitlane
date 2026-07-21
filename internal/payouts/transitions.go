package payouts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/ledger"
)

// ErrIgnored is returned when an update does not apply to the payout's
// current status (for example a late "processing" after "completed").
var ErrIgnored = errors.New("payouts: update ignored")

// Update is a status change reported by a provider, the poller or ops.
type Update struct {
	Status         domain.Status
	ProviderRef    string
	FailureCode    string
	FailureMessage string
	Source         string
	Detail         string
}

// Apply moves a payout to a new status, posting the ledger entries and
// partner webhook that go with it.
func (s *Service) Apply(ctx context.Context, payoutID string, u Update) (db.Payout, error) {
	var out db.Payout
	var deliveries []int64
	err := s.store.InTx(ctx, func(q *db.Queries, _ pgx.Tx) error {
		p, err := q.LockPayout(ctx, payoutID)
		if err != nil {
			return err
		}
		if p.ProviderRef == "" && u.ProviderRef != "" {
			if err := q.SetProviderRef(ctx, db.SetProviderRefParams{ID: p.ID, ProviderRef: u.ProviderRef}); err != nil {
				return err
			}
		}
		from := domain.Status(p.Status)
		if from == u.Status {
			out = p
			return nil
		}
		if !domain.CanMove(from, u.Status) {
			s.log.Warn("ignoring status update", "payout_id", p.ID, "from", from, "to", u.Status, "source", u.Source)
			_ = q.InsertPayoutEvent(ctx, db.InsertPayoutEventParams{
				PayoutID: p.ID, Status: string(u.Status), Source: u.Source, Detail: "ignored: payout is " + string(from),
			})
			out = p
			return ErrIgnored
		}
		if err := q.SetPayoutStatus(ctx, db.SetPayoutStatusParams{
			ID: p.ID, Status: string(u.Status), FailureCode: u.FailureCode, FailureMessage: u.FailureMessage,
		}); err != nil {
			return err
		}
		if err := q.InsertPayoutEvent(ctx, db.InsertPayoutEventParams{
			PayoutID: p.ID, Status: string(u.Status), Source: u.Source, Detail: u.Detail,
		}); err != nil {
			return err
		}
		if err := postFor(ctx, q, p, u.Status); err != nil {
			return err
		}
		if out, err = q.GetPayout(ctx, p.ID); err != nil {
			return err
		}
		if et := u.Status.EventType(); et != "" {
			deliveries, err = s.events.Emit(ctx, q, et, out)
			return err
		}
		return nil
	})
	if errors.Is(err, ErrIgnored) {
		return out, err
	}
	if err != nil {
		return db.Payout{}, err
	}
	s.events.Enqueue(ctx, deliveries)
	return out, nil
}

// postFor books the ledger movement for reaching status.
func postFor(ctx context.Context, q *db.Queries, p db.Payout, to domain.Status) error {
	lp := ledgerPayout(p)
	var kind string
	var entries []ledger.Entry
	switch to {
	case domain.Completed:
		kind, entries = "payout_settle", ledger.Settle(lp)
	case domain.Failed:
		kind, entries = "payout_release", ledger.Release(lp)
	case domain.Reversed:
		kind, entries = "payout_reversal", ledger.Reverse(lp)
	default:
		return nil
	}
	_, err := ledger.Post(ctx, q, kind, p.ID, "", entries...)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
