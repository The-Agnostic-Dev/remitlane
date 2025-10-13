// Package console is the ops console: server-rendered pages for Remitlane's
// operations and finance teams.
package console

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/providers/momobridge/sandbox"
	swiftsandbox "github.com/remitlane/remitlane/internal/providers/swiftpay/sandbox"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/recon"
	"github.com/remitlane/remitlane/internal/store"
)

// Console serves /ops and /login.
type Console struct {
	store    *store.Store
	limits   *limits.Service
	webhooks *outbound.Service
	recon    *recon.Builder
	queue    queue.Enqueuer
	clock    *clock.Clock
	cfg      config.Config
	log      *slog.Logger
	swiftpay *swiftsandbox.Server
	momo     *sandbox.Server
	pages    map[string]*template.Template
}

// Deps are the console's collaborators. The sandbox servers are nil when the
// providers run in live mode.
type Deps struct {
	Store    *store.Store
	Limits   *limits.Service
	Webhooks *outbound.Service
	Recon    *recon.Builder
	Queue    queue.Enqueuer
	Clock    *clock.Clock
	Config   config.Config
	Log      *slog.Logger
	Swiftpay *swiftsandbox.Server
	MoMo     *sandbox.Server
}

// New parses the templates and returns the console.
func New(d Deps) (*Console, error) {
	c := &Console{
		store: d.Store, limits: d.Limits, webhooks: d.Webhooks, recon: d.Recon, queue: d.Queue,
		clock: d.Clock, cfg: d.Config, log: d.Log.With("component", "console"), swiftpay: d.Swiftpay, momo: d.MoMo,
	}
	pages, err := c.parse()
	if err != nil {
		return nil, err
	}
	c.pages = pages
	return c, nil
}

// Routes mounts the console on r.
func (c *Console) Routes(r chi.Router) {
	static, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ops", http.StatusFound) })
	r.Get("/login", c.loginPage)
	r.Post("/login", c.login)
	r.Post("/logout", c.logout)

	r.Route("/ops", func(r chi.Router) {
		r.Use(c.requireSession, c.checkCSRF)
		r.Get("/", c.dashboard)
		r.Get("/payouts", c.payouts)
		r.Get("/payouts/{id}", c.payout)
		r.Get("/partners", c.partners)
		r.Get("/partners/{id}", c.partner)
		r.Post("/partners/{id}/limit", c.updateLimit)
		r.Get("/ledger", c.ledger)
		r.Get("/webhooks/failed", c.failedWebhooks)
		r.Post("/webhooks/deliveries/{id}/retry", c.retryDelivery)
		r.Post("/webhooks/endpoints/{id}/retry", c.retryEndpoint)
		r.Get("/limits", c.limitsPage)
		r.Get("/reconciliation", c.reconPage)
		r.Get("/reconciliation.csv", c.reconCSV)
		r.Post("/reconciliation/run", c.reconRun)
		r.Get("/providers", c.providersPage)
		r.Get("/sandbox", c.sandboxPage)
		r.Post("/sandbox/force", c.sandboxForce)
	})
}
