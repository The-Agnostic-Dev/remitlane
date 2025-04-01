package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/recipients"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/wire"
)

type createPayoutRequest struct {
	QuoteID   string               `json:"quote_id"`
	Reference string               `json:"reference"`
	Narration string               `json:"narration"`
	Recipient recipients.Recipient `json:"recipient"`
}

func (a *API) createPayout(w http.ResponseWriter, r *http.Request) {
	var req createPayoutRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if req.QuoteID == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "quote_id is required")
		return
	}
	p, err := a.payouts.Create(r.Context(), partnerFrom(r.Context()), payouts.CreateRequest{
		QuoteID: req.QuoteID, Reference: req.Reference, Narration: req.Narration, Recipient: req.Recipient,
	})
	if err != nil {
		a.writePayoutError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, wire.FromPayout(p))
}

func (a *API) getPayout(w http.ResponseWriter, r *http.Request) {
	p, err := a.store.Q.GetPayoutForPartner(r.Context(), db.GetPayoutForPartnerParams{
		ID: chi.URLParam(r, "id"), PartnerID: partnerFrom(r.Context()).ID,
	})
	if store.IsNotFound(err) {
		writeError(w, r, http.StatusNotFound, payouts.CodePayoutNotFound, "no such payout")
		return
	}
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, wire.FromPayout(p))
}
