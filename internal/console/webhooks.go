package console

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/store"
)

type webhooksPage struct {
	Rows      []store.FailedDelivery
	Partners  []db.Partner
	PartnerID string
	Endpoints map[int64]string
}

func (c *Console) failedWebhooks(w http.ResponseWriter, r *http.Request) {
	partnerID := r.URL.Query().Get("partner")
	rows, err := c.store.ListFailedDeliveries(r.Context(), partnerID, 200)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	partners, err := c.store.Q.ListPartners(r.Context())
	if err != nil {
		c.fail(w, r, err)
		return
	}
	endpoints := map[int64]string{}
	for _, d := range rows {
		endpoints[d.EndpointID] = d.PartnerName
	}
	c.renderTitled(w, r, "webhooks", "Failed webhooks", webhooksPage{Rows: rows, Partners: partners, PartnerID: partnerID, Endpoints: endpoints})
}

func (c *Console) retryDelivery(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.notFound(w, r, "delivery")
		return
	}
	n, err := c.webhooks.Retry(r.Context(), id)
	switch {
	case errors.Is(err, outbound.ErrNotFailed):
		setFlash(w, "error", fmt.Sprintf("Delivery %d is not in the failed queue any more.", id))
	case store.IsNotFound(err):
		c.notFound(w, r, "delivery")
		return
	case err != nil:
		c.fail(w, r, err)
		return
	default:
		c.log.Info("webhook retry", "delivery_id", id, "by", userFrom(r.Context()).Email)
		setFlash(w, "ok", fmt.Sprintf("Requeued %d %s. The result shows here in a few seconds.", n, plural(n, "delivery", "deliveries")))
	}
	http.Redirect(w, r, "/ops/webhooks/failed", http.StatusSeeOther)
}

func (c *Console) retryEndpoint(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.notFound(w, r, "endpoint")
		return
	}
	n, err := c.webhooks.RetryEndpoint(r.Context(), id)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	c.log.Info("webhook retry all", "endpoint_id", id, "by", userFrom(r.Context()).Email)
	setFlash(w, "ok", fmt.Sprintf("Requeued %d %s.", n, plural(n, "delivery", "deliveries")))
	http.Redirect(w, r, "/ops/webhooks/failed", http.StatusSeeOther)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
