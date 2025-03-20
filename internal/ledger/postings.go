package ledger

import (
	"github.com/remitlane/remitlane/internal/money"
)

// Payout is what the posting recipes need to know about a payout.
type Payout struct {
	ID        string
	PartnerID string
	Provider  string
	Currency  money.Currency // source currency: the partner balance it is paid from
	Amount    int64          // source amount, minor units
	Fee       int64          // fee, minor units of the source currency
}

func (p Payout) total() int64 { return p.Amount + p.Fee }

// Hold reserves amount and fee when a payout is created.
func Hold(p Payout) []Entry {
	return []Entry{
		{Available(p.PartnerID, p.Currency), -p.total()},
		{Held(p.PartnerID, p.Currency), p.total()},
	}
}

// Settle moves held funds to the provider and books the fee when a payout
// completes.
func Settle(p Payout) []Entry {
	return []Entry{
		{Held(p.PartnerID, p.Currency), -p.total()},
		{Clearing(p.Provider, p.Currency), p.Amount},
		{Fees(p.Currency), p.Fee},
	}
}

// Release gives held funds back when a payout fails.
func Release(p Payout) []Entry {
	return []Entry{
		{Held(p.PartnerID, p.Currency), -p.total()},
		{Available(p.PartnerID, p.Currency), p.total()},
	}
}

// Reverse refunds a completed payout that the recipient's bank sent back,
// fee included.
func Reverse(p Payout) []Entry {
	return []Entry{
		{Clearing(p.Provider, p.Currency), -p.Amount},
		{Fees(p.Currency), -p.Fee},
		{Available(p.PartnerID, p.Currency), p.total()},
	}
}

// Fund credits a partner top-up.
func Fund(partnerID string, a money.Amount) []Entry {
	return []Entry{
		{Funding(a.Currency), -a.Minor},
		{Available(partnerID, a.Currency), a.Minor},
	}
}
