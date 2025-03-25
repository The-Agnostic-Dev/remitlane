package rates

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/money"
)

func testQuoter(t *testing.T) *Quoter {
	t.Helper()
	feed, err := NewFixedFeed(config.RatesConfig{USDNGN: "1540.00", USDKES: "129.37", USDGHS: "15.82", USDUGX: "3698.40"})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Africa/Lagos")
	return NewQuoter(feed, clock.Fixed(time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC), loc), 15*time.Minute)
}

func TestQuoteBySourceNGN(t *testing.T) {
	q := testQuoter(t)
	p, err := q.Calculate(context.Background(), Request{From: money.USD, To: money.NGN, Channel: Bank, AmountMinor: 100000, AmountType: BySource})
	if err != nil {
		t.Fatal(err)
	}
	// $1,000 at 1540 less 1.5% = 1516.90 per dollar.
	if p.DestMinor != 151690000 {
		t.Errorf("dest = %d", p.DestMinor)
	}
	if RateString(p.Rate) != "1516.900000" {
		t.Errorf("rate = %s", RateString(p.Rate))
	}
	if p.FeeMinor != 100 {
		t.Errorf("fee = %d", p.FeeMinor)
	}
}

func TestQuoteByDestinationRoundsSourceUp(t *testing.T) {
	q := testQuoter(t)
	p, err := q.Calculate(context.Background(), Request{From: money.USD, To: money.UGX, Channel: Bank, AmountMinor: 1850000, AmountType: ByDestination})
	if err != nil {
		t.Fatal(err)
	}
	// 1,850,000 UGX at 3698.40 less 2% = 3624.432 per dollar: $510.42 (rounded up).
	if p.SourceMinor != 51043 {
		t.Errorf("source = %d", p.SourceMinor)
	}
	back := convert(p.SourceMinor, money.USD, money.UGX, p.Rate, money.Down)
	if back < p.DestMinor {
		t.Errorf("source %d only buys %d", p.SourceMinor, back)
	}
}

func TestQuoteExpiry(t *testing.T) {
	q := testQuoter(t)
	p, err := q.Calculate(context.Background(), Request{From: money.USD, To: money.GHS, Channel: Bank, AmountMinor: 5000, AmountType: BySource})
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 6, 9, 15, 0, 0, time.UTC); !p.ExpiresAt.Equal(want) {
		t.Errorf("expires %s, want %s", p.ExpiresAt, want)
	}
}

func TestQuoteErrors(t *testing.T) {
	q := testQuoter(t)
	ctx := context.Background()
	if _, err := q.Calculate(ctx, Request{From: money.NGN, To: money.KES, Channel: Bank, AmountMinor: 100}); !errors.Is(err, ErrNoCorridor) {
		t.Errorf("want ErrNoCorridor, got %v", err)
	}
	if _, err := q.Calculate(ctx, Request{From: money.USD, To: money.NGN, Channel: Bank, AmountMinor: 1, AmountType: BySource}); !errors.Is(err, ErrAmountRange) {
		t.Errorf("want ErrAmountRange, got %v", err)
	}
	if _, err := q.Calculate(ctx, Request{From: money.USD, To: money.NGN, Channel: Bank, AmountMinor: 0}); !errors.Is(err, ErrBadAmount) {
		t.Errorf("want ErrBadAmount, got %v", err)
	}
}

func TestToUSD(t *testing.T) {
	feed, _ := NewFixedFeed(config.RatesConfig{USDNGN: "1540.00", USDKES: "129.37", USDGHS: "15.82", USDUGX: "3698.40"})
	got, err := ToUSD(context.Background(), feed, money.New(1540000, money.NGN))
	if err != nil || got != 1000 {
		t.Errorf("NGN 15,400 = %d cents, %v", got, err)
	}
	got, _ = ToUSD(context.Background(), feed, money.New(3698400, money.UGX))
	if got != 100000 {
		t.Errorf("UGX 3,698,400 = %d cents", got)
	}
}
