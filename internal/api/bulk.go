package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recipients"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/wire"
)

type bulkItem struct {
	Reference           string               `json:"reference"`
	SourceCurrency      string               `json:"source_currency"`
	DestinationCurrency string               `json:"destination_currency"`
	Channel             string               `json:"channel"`
	AmountMinor         int64                `json:"amount_minor"`
	Narration           string               `json:"narration"`
	Recipient           recipients.Recipient `json:"recipient"`
}

type bulkRequest struct {
	Reference string     `json:"reference"`
	Items     []bulkItem `json:"items"`
}

type batchResponse struct {
	ID        string        `json:"id"`
	Object    string        `json:"object"`
	Reference string        `json:"reference"`
	ItemCount int32         `json:"item_count"`
	CreatedAt string        `json:"created_at"`
	Payouts   []wire.Payout `json:"payouts"`
}

func (a *API) createBulk(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	in := payouts.BulkRequest{Reference: req.Reference}
	for i, it := range req.Items {
		src, err := money.ParseCurrency(it.SourceCurrency)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", fmt.Sprintf("items[%d].source_currency: %v", i, err))
			return
		}
		dst, err := money.ParseCurrency(it.DestinationCurrency)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", fmt.Sprintf("items[%d].destination_currency: %v", i, err))
			return
		}
		in.Items = append(in.Items, payouts.BulkItem{
			Reference: it.Reference, SourceCurrency: src, DestCurrency: dst, Channel: rates.Channel(it.Channel),
			AmountMinor: it.AmountMinor, Narration: it.Narration, Recipient: it.Recipient,
		})
	}
	batch, rows, err := a.payouts.Bulk(r.Context(), partnerFrom(r.Context()), in)
	if err != nil {
		a.writePayoutError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, presentBatch(batch, rows))
}

func presentBatch(b db.PayoutBatch, rows []db.Payout) batchResponse {
	out := batchResponse{
		ID: b.ID, Object: "batch", Reference: b.Reference, ItemCount: b.ItemCount, CreatedAt: wire.Time(b.CreatedAt),
		Payouts: make([]wire.Payout, 0, len(rows)),
	}
	for _, p := range rows {
		out.Payouts = append(out.Payouts, wire.FromPayout(p))
	}
	return out
}

func (a *API) getBatch(w http.ResponseWriter, r *http.Request) {
	partner := partnerFrom(r.Context())
	b, err := a.store.Q.GetBatchForPartner(r.Context(), db.GetBatchForPartnerParams{ID: chi.URLParam(r, "id"), PartnerID: partner.ID})
	if store.IsNotFound(err) {
		writeError(w, r, http.StatusNotFound, "batch_not_found", "no such batch")
		return
	}
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	rows, _, err := a.store.ListPartnerPayouts(r.Context(), store.PartnerPayoutFilter{PartnerID: partner.ID, BatchID: b.ID, Limit: 1000})
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, presentBatch(b, rows))
}
