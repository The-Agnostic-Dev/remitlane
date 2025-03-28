package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/wire"
)

type quoteRequest struct {
	SourceCurrency      string `json:"source_currency"`
	DestinationCurrency string `json:"destination_currency"`
	Channel             string `json:"channel"`
	AmountMinor         int64  `json:"amount_minor"`
	AmountType          string `json:"amount_type"`
}

type quoteResponse struct {
	ID                     string `json:"id"`
	Object                 string `json:"object"`
	SourceCurrency         string `json:"source_currency"`
	DestinationCurrency    string `json:"destination_currency"`
	Channel                string `json:"channel"`
	SourceAmountMinor      int64  `json:"source_amount_minor"`
	DestinationAmountMinor int64  `json:"destination_amount_minor"`
	FeeMinor               int64  `json:"fee_minor"`
	TotalDebitMinor        int64  `json:"total_debit_minor"`
	Rate                   string `json:"rate"`
	MidRate                string `json:"mid_rate"`
	SpreadBps              int32  `json:"spread_bps"`
	ExpiresAt              string `json:"expires_at"`
	CreatedAt              string `json:"created_at"`
	Used                   bool   `json:"used"`
}

func presentQuote(q quotes.Quote) quoteResponse {
	return quoteResponse{
		ID: q.ID, Object: "quote", SourceCurrency: string(q.From), DestinationCurrency: string(q.To),
		Channel: string(q.Channel), SourceAmountMinor: q.SourceMinor, DestinationAmountMinor: q.DestMinor,
		FeeMinor: q.FeeMinor, TotalDebitMinor: q.SourceMinor + q.FeeMinor,
		Rate: q.Rate, MidRate: q.MidRate, SpreadBps: q.SpreadBps,
		ExpiresAt: q.ExpiresAt.Format(wire.TimeLayout),
		CreatedAt: wire.Time(q.CreatedAt),
		Used:      q.UsedAt != nil,
	}
}

func (a *API) createQuote(w http.ResponseWriter, r *http.Request) {
	var req quoteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	from, err := money.ParseCurrency(req.SourceCurrency)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "source_currency: "+err.Error())
		return
	}
	to, err := money.ParseCurrency(req.DestinationCurrency)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "destination_currency: "+err.Error())
		return
	}
	ch := rates.Channel(req.Channel)
	if !ch.Valid() {
		writeError(w, r, http.StatusBadRequest, "validation_error", "channel must be bank or mobile_money")
		return
	}
	partner := partnerFrom(r.Context())
	q, err := a.quotes.Create(r.Context(), partner.ID, rates.Request{
		From: from, To: to, Channel: ch, AmountMinor: req.AmountMinor, AmountType: rates.AmountType(req.AmountType),
	})
	if err != nil {
		perr := payouts.PriceError(err)
		var pe *payouts.Error
		if errors.As(perr, &pe) {
			a.writePayoutError(w, r, perr)
			return
		}
		internalError(w, r, a.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, presentQuote(q))
}

func (a *API) getQuote(w http.ResponseWriter, r *http.Request) {
	q, err := a.quotes.Get(r.Context(), partnerFrom(r.Context()).ID, chi.URLParam(r, "id"))
	if store.IsNotFound(err) {
		writeError(w, r, http.StatusNotFound, payouts.CodeQuoteNotFound, "no such quote")
		return
	}
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, presentQuote(q))
}
