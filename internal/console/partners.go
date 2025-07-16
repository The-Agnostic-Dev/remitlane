package console

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/store"
)

type partnerRow struct {
	Partner        db.Partner
	Balances       []store.Balance
	FailedWebhooks int
}

func (c *Console) partners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ps, err := c.store.Q.ListPartners(ctx)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	balances, err := c.store.AllBalances(ctx)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	failed, err := c.store.CountFailedDeliveries(ctx)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	rows := make([]partnerRow, 0, len(ps))
	for _, p := range ps {
		row := partnerRow{Partner: p, FailedWebhooks: failed[p.ID]}
		for _, b := range balances {
			if b.PartnerID == p.ID {
				row.Balances = append(row.Balances, b)
			}
		}
		rows = append(rows, row)
	}
	c.renderTitled(w, r, "partners", "Partners", rows)
}

type partnerPage struct {
	Partner   db.Partner
	Balances  []store.Balance
	Keys      []db.ListAPIKeysRow
	Endpoints []db.WebhookEndpoint
	Recent    []store.ConsolePayout
	Usage     limits.Usage
}

func (c *Console) partner(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := c.store.Q.GetPartner(ctx, chi.URLParam(r, "id"))
	if store.IsNotFound(err) {
		c.notFound(w, r, "partner")
		return
	}
	if err != nil {
		c.fail(w, r, err)
		return
	}
	data := partnerPage{Partner: p}
	if data.Balances, err = c.store.PartnerBalances(ctx, p.ID); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Keys, err = c.store.Q.ListAPIKeys(ctx, p.ID); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Endpoints, err = c.store.Q.ListActiveEndpoints(ctx, p.ID); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Recent, _, err = c.store.ListConsolePayouts(ctx, store.ConsoleFilter{PartnerID: p.ID, Page: 1, PerPage: 10}); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Usage, err = c.limits.Usage(ctx, p); err != nil {
		c.fail(w, r, err)
		return
	}
	c.renderTitled(w, r, "partner", p.Name, data)
}

