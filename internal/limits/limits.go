// Package limits enforces each partner's daily payout limit. The limit is in
// USD and the day is the business day in Lagos (see internal/clock).
package limits

import (
	"context"
	"fmt"
	"time"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/store"
)

// Usage is a partner's position against its limit today.
type Usage struct {
	LimitMinor     int64
	UsedMinor      int64
	RemainingMinor int64
	WindowStart    time.Time
	WindowEnd      time.Time
}

// ExceededError is returned when a payout would go over the limit.
type ExceededError struct {
	Usage
	Requested int64
}

func (e *ExceededError) Error() string {
	return fmt.Sprintf("daily limit of %s would be exceeded: %s used, %s requested",
		money.Format(e.LimitMinor, money.USD), money.Format(e.UsedMinor, money.USD), money.Format(e.Requested, money.USD))
}

// Service reads usage and checks new payouts.
type Service struct {
	store *store.Store
	clock *clock.Clock
}

// New returns a Service.
func New(s *store.Store, c *clock.Clock) *Service { return &Service{store: s, clock: c} }

// Usage returns today's usage for a partner.
func (s *Service) Usage(ctx context.Context, p db.Partner) (Usage, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	used, err := s.store.UsedUSD(ctx, p.ID, start, end)
	if err != nil {
		return Usage{}, err
	}
	rem := p.DailyLimitUsdMinor - used
	if rem < 0 {
		rem = 0
	}
	return Usage{LimitMinor: p.DailyLimitUsdMinor, UsedMinor: used, RemainingMinor: rem, WindowStart: start, WindowEnd: end}, nil
}

// Check returns an *ExceededError if adding usdMinor would pass the limit.
func (s *Service) Check(ctx context.Context, p db.Partner, usdMinor int64) error {
	u, err := s.Usage(ctx, p)
	if err != nil {
		return err
	}
	if u.UsedMinor+usdMinor > u.LimitMinor {
		return &ExceededError{Usage: u, Requested: usdMinor}
	}
	return nil
}
