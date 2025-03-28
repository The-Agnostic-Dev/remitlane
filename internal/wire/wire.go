// Package wire is the JSON shape of objects that leave Remitlane: API
// responses and partner webhooks share it so the two never drift apart.
package wire

import (
	"time"

	"github.com/remitlane/remitlane/internal/db"
)

// TimeLayout is how timestamps leave the API: RFC 3339 in UTC.
const TimeLayout = "2006-01-02T15:04:05Z"

// Time formats t for the API.
func Time(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(TimeLayout)
}

// Recipient is the payee as shown in the API.
type Recipient struct {
	Name          string `json:"name"`
	Country       string `json:"country"`
	BankCode      string `json:"bank_code,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	MSISDN        string `json:"msisdn,omitempty"`
}

// Failure explains why a payout failed.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Payout is a payout as partners see it.
type Payout struct {
	ID                     string    `json:"id"`
	Object                 string    `json:"object"`
	Reference              string    `json:"reference"`
	Status                 string    `json:"status"`
	Channel                string    `json:"channel"`
	QuoteID                string    `json:"quote_id,omitempty"`
	BatchID                string    `json:"batch_id,omitempty"`
	SourceCurrency         string    `json:"source_currency"`
	SourceAmountMinor      int64     `json:"source_amount_minor"`
	FeeMinor               int64     `json:"fee_minor"`
	DestinationCurrency    string    `json:"destination_currency"`
	DestinationAmountMinor int64     `json:"destination_amount_minor"`
	Rate                   string    `json:"rate"`
	Recipient              Recipient `json:"recipient"`
	Narration              string    `json:"narration,omitempty"`
	Failure                *Failure  `json:"failure,omitempty"`
	CreatedAt              string    `json:"created_at"`
	UpdatedAt              string    `json:"updated_at"`
	CompletedAt            string    `json:"completed_at,omitempty"`
}

// FromPayout converts a stored payout.
func FromPayout(p db.Payout) Payout {
	out := Payout{
		ID: p.ID, Object: "payout", Reference: p.Reference, Status: p.Status, Channel: p.Channel,
		QuoteID: p.QuoteID.String, BatchID: p.BatchID.String,
		SourceCurrency: p.SourceCurrency, SourceAmountMinor: p.SourceAmountMinor, FeeMinor: p.FeeMinor,
		DestinationCurrency: p.DestCurrency, DestinationAmountMinor: p.DestAmountMinor,
		Rate: p.Rate,
		Recipient: Recipient{
			Name: p.RecipientName, Country: p.RecipientCountry, BankCode: p.BankCode,
			AccountNumber: p.AccountNumber, MSISDN: p.Msisdn,
		},
		Narration: p.Narration,
		CreatedAt: Time(p.CreatedAt), UpdatedAt: Time(p.UpdatedAt),
	}
	if p.CompletedAt.Valid {
		out.CompletedAt = Time(p.CompletedAt.Time)
	}
	if p.FailureCode != "" {
		out.Failure = &Failure{Code: p.FailureCode, Message: p.FailureMessage}
	}
	return out
}

