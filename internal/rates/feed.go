// Package rates prices payouts: mid-market rates from a feed, a spread and a
// fee per corridor, and quotes that hold a rate for a short time.
package rates

import (
	"context"
	"fmt"
	"math/big"

	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/money"
)

// Feed returns mid-market rates: units of to for one unit of from.
type Feed interface {
	Mid(ctx context.Context, from, to money.Currency) (*big.Rat, error)
}

// FixedFeed serves rates from configuration. It is the sandbox feed and the
// fallback when the market data vendor is unreachable.
type FixedFeed struct {
	usd map[money.Currency]*big.Rat
}

// NewFixedFeed parses the configured rates.
func NewFixedFeed(c config.RatesConfig) (*FixedFeed, error) {
	f := &FixedFeed{usd: map[money.Currency]*big.Rat{money.USD: big.NewRat(1, 1)}}
	for cur, s := range map[money.Currency]string{
		money.NGN: c.USDNGN, money.KES: c.USDKES, money.GHS: c.USDGHS, money.UGX: c.USDUGX,
	} {
		r, ok := new(big.Rat).SetString(s)
		if !ok || r.Sign() <= 0 {
			return nil, fmt.Errorf("rates: bad USD/%s rate %q", cur, s)
		}
		f.usd[cur] = r
	}
	return f, nil
}

// Mid implements Feed by crossing through USD.
func (f *FixedFeed) Mid(_ context.Context, from, to money.Currency) (*big.Rat, error) {
	a, ok := f.usd[from]
	b, ok2 := f.usd[to]
	if !ok || !ok2 {
		return nil, fmt.Errorf("rates: no rate for %s/%s", from, to)
	}
	return new(big.Rat).Quo(b, a), nil
}

// ToUSD converts minor units of any currency to USD cents at the mid rate,
// rounding half up. Used for limits and reporting, never for pricing.
func ToUSD(ctx context.Context, f Feed, a money.Amount) (int64, error) {
	if a.Currency == money.USD {
		return a.Minor, nil
	}
	mid, err := f.Mid(ctx, money.USD, a.Currency)
	if err != nil {
		return 0, err
	}
	// cents = minor / 10^exp / mid * 100
	r := new(big.Rat).SetFrac(big.NewInt(a.Minor*100), big.NewInt(1))
	r.Quo(r, new(big.Rat).SetInt64(pow10(a.Currency.Exponent())))
	r.Quo(r, mid)
	return money.RoundRat(r, money.HalfUp), nil
}

func pow10(n int) int64 {
	p := int64(1)
	for range n {
		p *= 10
	}
	return p
}
