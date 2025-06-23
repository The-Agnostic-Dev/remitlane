package rates

import (
	"github.com/remitlane/remitlane/internal/money"
)

// Channel is how the recipient is paid.
type Channel string

// Payout channels.
const (
	Bank        Channel = "bank"
	MobileMoney Channel = "mobile_money"
)

// Valid reports whether the channel is known.
func (c Channel) Valid() bool { return c == Bank || c == MobileMoney }

// Corridor is a priced route from a source balance to a payout currency.
type Corridor struct {
	From      money.Currency
	To        money.Currency
	Channel   Channel
	SpreadBps int64 // taken off the mid rate
	FeeMinor  int64 // flat fee in the source currency
	MinMinor  int64 // minimum destination amount
	MaxMinor  int64 // maximum destination amount
}

// corridors is the price list. Changes go through finance (see docs/api.md).
var corridors = []Corridor{
	{money.USD, money.NGN, Bank, 150, 100, 100000, 2000000000},
	{money.USD, money.KES, Bank, 120, 150, 10000, 100000000},
	{money.USD, money.KES, MobileMoney, 100, 75, 1000, 25000000},
	{money.USD, money.GHS, Bank, 150, 150, 1000, 50000000},
	{money.USD, money.GHS, MobileMoney, 120, 75, 100, 1000000},
	{money.USD, money.UGX, Bank, 200, 200, 10000, 400000000},
	{money.USD, money.UGX, MobileMoney, 150, 100, 500, 7000000},
	{money.NGN, money.NGN, Bank, 0, 5000, 10000, 2000000000},
}

// Find looks up a corridor.
func Find(from, to money.Currency, ch Channel) (Corridor, bool) {
	for _, c := range corridors {
		if c.From == from && c.To == to && c.Channel == ch {
			return c, true
		}
	}
	return Corridor{}, false
}

// All returns every corridor, for the corridors endpoint.
func All() []Corridor {
	out := make([]Corridor, len(corridors))
	copy(out, corridors)
	return out
}
