// Package devtools serves development helpers under /_dev: the email outbox,
// a fake partner webhook receiver and a JSON view of the provider sandboxes.
// Mounted only when DEV_TOOLS_ENABLED=true.
package devtools

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/httpx"
	momosandbox "github.com/remitlane/remitlane/internal/providers/momobridge/sandbox"
	swiftsandbox "github.com/remitlane/remitlane/internal/providers/swiftpay/sandbox"
	"github.com/remitlane/remitlane/internal/store"
)

//go:embed templates/*.html
var templates embed.FS

// Tools serves /_dev.
type Tools struct {
	store    *store.Store
	clock    *clock.Clock
	log      *slog.Logger
	swiftpay *swiftsandbox.Server
	momo     *momosandbox.Server
	tmpl     *template.Template
}

// New returns the dev tools.
func New(s *store.Store, c *clock.Clock, log *slog.Logger, sp *swiftsandbox.Server, mm *momosandbox.Server) *Tools {
	t := template.Must(template.New("dev").Funcs(template.FuncMap{"when": c.Display}).ParseFS(templates, "templates/*.html"))
	return &Tools{store: s, clock: c, log: log.With("component", "devtools"), swiftpay: sp, momo: mm, tmpl: t}
}

// Routes mounts the tools on r.
func (t *Tools) Routes(r chi.Router) {
	r.Get("/_dev/outbox", t.outbox)
	r.Get("/_dev/outbox.json", t.outboxJSON)
	r.Get("/_dev/receiver", t.receiverPage)
	r.Get("/_dev/receiver.json", t.receiverJSON)
	r.Post("/_dev/receiver/{slug}", t.receive)
	r.Post("/_dev/receiver/{slug}/config", t.configure)
	r.Get("/_dev/sandbox/transfers.json", t.sandboxJSON)
}

func (t *Tools) page(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.tmpl.ExecuteTemplate(w, name, data); err != nil {
		t.log.Error("render dev page", "page", name, "err", err)
	}
}

func (t *Tools) outbox(w http.ResponseWriter, r *http.Request) {
	msgs, err := t.store.Q.ListOutbox(r.Context(), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.page(w, "outbox.html", msgs)
}

type outboxMessage struct {
	ID        int64  `json:"id"`
	Channel   string `json:"channel"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func (t *Tools) outboxJSON(w http.ResponseWriter, r *http.Request) {
	msgs, err := t.store.Q.ListOutbox(r.Context(), 500)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]outboxMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, outboxMessage{m.ID, m.Channel, m.Recipient, m.Subject, m.Body, m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": out})
}

func (t *Tools) sandboxJSON(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("reference")
	out := map[string]any{"swiftpay": []swiftsandbox.Transfer{}, "momobridge": []momosandbox.Disbursement{}}
	if t.swiftpay != nil {
		out["swiftpay"] = t.swiftpay.Transfers(ref)
	}
	if t.momo != nil {
		out["momobridge"] = t.momo.Disbursements(ref)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
