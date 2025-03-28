// Package quotes stores priced quotes. A quote fixes a rate for QUOTE_TTL
// (15 minutes by default) and can be used for one payout.
package quotes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/store"
)

// Quote is a stored price.
type Quote struct {
	ID          string
	PartnerID   string
	From        money.Currency
	To          money.Currency
	Channel     rates.Channel
	SourceMinor int64
	DestMinor   int64
	FeeMinor    int64
	Rate        string
	MidRate     string
	SpreadBps   int32
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UsedAt      *time.Time
}

// Expired reports whether the quote can no longer be used at t.
func (q Quote) Expired(t time.Time) bool { return !t.Before(q.ExpiresAt) }

// Service creates and reads quotes.
type Service struct {
	store  *store.Store
	quoter *rates.Quoter
	clock  *clock.Clock
}

// New returns a Service.
func New(s *store.Store, q *rates.Quoter, c *clock.Clock) *Service {
	return &Service{store: s, quoter: q, clock: c}
}

// Create prices and stores a quote.
func (s *Service) Create(ctx context.Context, partnerID string, req rates.Request) (Quote, error) {
	p, err := s.quoter.Calculate(ctx, req)
	if err != nil {
		return Quote{}, err
	}
	row, err := s.store.Q.CreateQuote(ctx, db.CreateQuoteParams{
		ID: newID(), PartnerID: partnerID,
		SourceCurrency: string(req.From), DestCurrency: string(req.To), Channel: string(req.Channel),
		SourceAmountMinor: p.SourceMinor, DestAmountMinor: p.DestMinor, FeeMinor: p.FeeMinor,
		Rate: p.Rate.FloatString(8), MidRate: p.Mid.FloatString(8), SpreadBps: int32(p.Corridor.SpreadBps),
		ExpiresAt: p.ExpiresAt,
	})
	if err != nil {
		return Quote{}, err
	}
	return Quote{
		ID: row.ID, PartnerID: partnerID, From: req.From, To: req.To, Channel: req.Channel,
		SourceMinor: p.SourceMinor, DestMinor: p.DestMinor, FeeMinor: p.FeeMinor,
		Rate: rates.RateString(p.Rate), MidRate: rates.RateString(p.Mid), SpreadBps: int32(p.Corridor.SpreadBps),
		ExpiresAt: p.ExpiresAt, CreatedAt: row.CreatedAt.In(s.clock.Location()),
	}, nil
}

// Get loads a partner's quote.
func (s *Service) Get(ctx context.Context, partnerID, id string) (Quote, error) {
	row, err := s.store.Q.GetQuoteForPartner(ctx, db.GetQuoteForPartnerParams{ID: id, PartnerID: partnerID})
	if err != nil {
		return Quote{}, err
	}
	return FromRow(row, s.clock.Location()), nil
}

// FromRow converts a stored quote, with times in the business zone.
func FromRow(row db.Quote, loc *time.Location) Quote {
	q := Quote{
		ID: row.ID, PartnerID: row.PartnerID,
		From: money.Currency(row.SourceCurrency), To: money.Currency(row.DestCurrency), Channel: rates.Channel(row.Channel),
		SourceMinor: row.SourceAmountMinor, DestMinor: row.DestAmountMinor, FeeMinor: row.FeeMinor,
		Rate: sixDP(row.Rate), MidRate: sixDP(row.MidRate), SpreadBps: row.SpreadBps,
		ExpiresAt: row.ExpiresAt.In(loc), CreatedAt: row.CreatedAt.In(loc),
	}
	q.UsedAt = timePtr(row.UsedAt, loc)
	return q
}

func sixDP(s string) string {
	if len(s) > 2 {
		return s[:len(s)-2]
	}
	return s
}

func timePtr(t pgtype.Timestamptz, loc *time.Location) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.In(loc)
	return &v
}

func newID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return "qt_" + hex.EncodeToString(b)
}
