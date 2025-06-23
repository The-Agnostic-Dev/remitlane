package api

import (
	"net/http"

	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/wire"
)

type balance struct {
	Currency       string `json:"currency"`
	AvailableMinor int64  `json:"available_minor"`
	HeldMinor      int64  `json:"held_minor"`
}

func (a *API) listBalances(w http.ResponseWriter, r *http.Request) {
	rows, err := a.store.PartnerBalances(r.Context(), partnerFrom(r.Context()).ID)
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	data := make([]balance, 0, len(rows))
	for _, b := range rows {
		data = append(data, balance{Currency: string(b.Currency), AvailableMinor: b.Available, HeldMinor: b.Held})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

type limitsResponse struct {
	Object         string `json:"object"`
	Currency       string `json:"currency"`
	DailyLimit     int64  `json:"daily_limit_minor"`
	UsedToday      int64  `json:"used_today_minor"`
	RemainingToday int64  `json:"remaining_today_minor"`
	WindowStart    string `json:"window_start"`
	WindowEnd      string `json:"window_end"`
	TimeZone       string `json:"time_zone"`
}

func (a *API) getLimits(w http.ResponseWriter, r *http.Request) {
	u, err := a.limits.Usage(r.Context(), partnerFrom(r.Context()))
	if err != nil {
		internalError(w, r, a.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, limitsResponse{
		Object: "limits", Currency: string(money.USD), DailyLimit: u.LimitMinor, UsedToday: u.UsedMinor,
		RemainingToday: u.RemainingMinor, WindowStart: wire.Time(u.WindowStart), WindowEnd: wire.Time(u.WindowEnd),
		TimeZone: a.clock.Location().String(),
	})
}

type corridor struct {
	SourceCurrency      string `json:"source_currency"`
	DestinationCurrency string `json:"destination_currency"`
	Channel             string `json:"channel"`
	SpreadBps           int64  `json:"spread_bps"`
	FeeMinor            int64  `json:"fee_minor"`
	MinAmountMinor      int64  `json:"min_destination_amount_minor"`
	MaxAmountMinor      int64  `json:"max_destination_amount_minor"`
}

func (a *API) listCorridors(w http.ResponseWriter, r *http.Request) {
	all := rates.All()
	data := make([]corridor, 0, len(all))
	for _, c := range all {
		data = append(data, corridor{
			SourceCurrency: string(c.From), DestinationCurrency: string(c.To), Channel: string(c.Channel),
			SpreadBps: c.SpreadBps, FeeMinor: c.FeeMinor, MinAmountMinor: c.MinMinor, MaxAmountMinor: c.MaxMinor,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}
