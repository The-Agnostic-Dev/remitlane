package rates

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/money"
)

// AmountType says which side of the quote the partner fixed.
type AmountType string

// Amount types.
const (
	BySource      AmountType = "source"
	ByDestination AmountType = "destination"
)

// Request asks for a price.
type Request struct {
	From        money.Currency
	To          money.Currency
	Channel     Channel
	AmountMinor int64
	AmountType  AmountType
}

// Price is a calculated quote, before it is stored.
type Price struct {
	Corridor    Corridor
	SourceMinor int64
	DestMinor   int64
	FeeMinor    int64
	Rate        *big.Rat
	Mid         *big.Rat
	ExpiresAt   time.Time
}

// Errors returned by Calculate.
var (
	ErrNoCorridor   = errors.New("rates: corridor not supported")
	ErrAmountRange  = errors.New("rates: amount outside corridor limits")
	ErrBadAmount    = errors.New("rates: amount must be positive")
	ErrBadAmountTyp = errors.New("rates: amount_type must be source or destination")
)

// Quoter prices requests.
type Quoter struct {
	feed  Feed
	clock *clock.Clock
	ttl   time.Duration
}

// NewQuoter returns a Quoter.
func NewQuoter(feed Feed, c *clock.Clock, ttl time.Duration) *Quoter {
	return &Quoter{feed: feed, clock: c, ttl: ttl}
}

// Calculate prices a request at the current rate.
func (q *Quoter) Calculate(ctx context.Context, req Request) (Price, error) {
	if req.AmountMinor <= 0 {
		return Price{}, ErrBadAmount
	}
	c, ok := Find(req.From, req.To, req.Channel)
	if !ok {
		return Price{}, fmt.Errorf("%w: %s to %s by %s", ErrNoCorridor, req.From, req.To, req.Channel)
	}
	mid, err := q.feed.Mid(ctx, req.From, req.To)
	if err != nil {
		return Price{}, err
	}
	rate := applySpread(mid, c.SpreadBps)

	var src, dst int64
	switch req.AmountType {
	case BySource, "":
		src = req.AmountMinor
		dst = convert(src, req.From, req.To, rate, money.Down)
	case ByDestination:
		dst = req.AmountMinor
		src = convertBack(dst, req.From, req.To, rate)
	default:
		return Price{}, ErrBadAmountTyp
	}
	if dst < c.MinMinor || dst > c.MaxMinor {
		return Price{}, fmt.Errorf("%w: %s to %s", ErrAmountRange,
			money.Format(c.MinMinor, c.To), money.Format(c.MaxMinor, c.To))
	}
	return Price{
		Corridor:    c,
		SourceMinor: src,
		DestMinor:   dst,
		FeeMinor:    c.FeeMinor,
		Rate:        rate,
		Mid:         mid,
		ExpiresAt:   q.clock.Now().Add(q.ttl),
	}, nil
}

// applySpread takes spread basis points off the mid rate.
func applySpread(mid *big.Rat, bps int64) *big.Rat {
	f := new(big.Rat).SetFrac64(10000-bps, 10000)
	return new(big.Rat).Mul(mid, f)
}

// convert turns source minor units into destination minor units at rate,
// rounded to a whole destination minor unit.
func convert(src int64, from, to money.Currency, rate *big.Rat, mode money.RoundMode) int64 {
	r := new(big.Rat).SetInt64(src)
	r.Mul(r, rate)
	r.Mul(r, new(big.Rat).SetFrac64(pow10(to.Exponent()), pow10(from.Exponent())))
	return money.RoundRat(r, mode)
}

// convertBack is the source amount needed to deliver dst, rounded up so the
// recipient is never short.
func convertBack(dst int64, from, to money.Currency, rate *big.Rat) int64 {
	r := new(big.Rat).SetInt64(dst)
	r.Quo(r, rate)
	r.Mul(r, new(big.Rat).SetFrac64(pow10(from.Exponent()), pow10(to.Exponent())))
	return money.RoundRat(r, money.Up)
}

// RateString renders a rate with six decimal places, as the API shows it.
func RateString(r *big.Rat) string { return r.FloatString(6) }
