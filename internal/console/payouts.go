package console

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/domain"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/store"
)

const payoutsPerPage = 25

type payoutsPage struct {
	Rows       []store.ConsolePayout
	Total      int
	Filter     store.ConsoleFilter
	Query      url.Values
	Pages      int
	From, To   int
	Partners   []db.Partner
	Statuses   []domain.Status
	Currencies []money.Currency
	Providers  []string
}

func (c *Console) payouts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	f := store.ConsoleFilter{
		PartnerID: q.Get("partner"), Status: q.Get("status"), Currency: q.Get("currency"),
		Provider: q.Get("provider"), Search: q.Get("q"), Page: page, PerPage: payoutsPerPage,
	}
	rows, total, err := c.store.ListConsolePayouts(r.Context(), f)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	partners, err := c.store.Q.ListPartners(r.Context())
	if err != nil {
		c.fail(w, r, err)
		return
	}
	data := payoutsPage{
		Rows: rows, Total: total, Filter: f, Query: q, Partners: partners,
		Pages:      (total + payoutsPerPage - 1) / payoutsPerPage,
		Statuses:   domain.AllStatuses(),
		Currencies: []money.Currency{money.NGN, money.KES, money.GHS, money.UGX},
		Providers:  []string{"swiftpay", "momobridge"},
	}
	if len(rows) > 0 {
		data.From = (page-1)*payoutsPerPage + 1
		data.To = data.From + len(rows) - 1
	}
	c.renderTitled(w, r, "payouts", "Payouts", data)
}

type sandboxAttempt struct {
	ID        string
	Status    string
	CreatedAt string
	Callbacks int
}

type payoutPage struct {
	Payout     db.Payout
	Partner    db.Partner
	Events     []db.PayoutEvent
	Ledger     []db.ListTxnsForPayoutRow
	Deliveries []db.ListDeliveriesForPayoutRow
	Sandbox    []sandboxAttempt
}

func (c *Console) payout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := c.store.Q.GetPayout(ctx, chi.URLParam(r, "id"))
	if store.IsNotFound(err) {
		c.notFound(w, r, "payout")
		return
	}
	if err != nil {
		c.fail(w, r, err)
		return
	}
	data := payoutPage{Payout: p}
	if data.Partner, err = c.store.Q.GetPartner(ctx, p.PartnerID); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Events, err = c.store.Q.ListPayoutEvents(ctx, p.ID); err != nil {
		c.fail(w, r, err)
		return
	}
	pid := pgtype.Text{String: p.ID, Valid: true}
	if data.Ledger, err = c.store.Q.ListTxnsForPayout(ctx, pid); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Deliveries, err = c.store.Q.ListDeliveriesForPayout(ctx, pid); err != nil {
		c.fail(w, r, err)
		return
	}
	c.renderTitled(w, r, "payout", "Payout "+p.ID, data)
}

