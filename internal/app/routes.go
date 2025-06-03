package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/api"
	"github.com/remitlane/remitlane/internal/console"
	"github.com/remitlane/remitlane/internal/devtools"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/inbound"
	"github.com/remitlane/remitlane/internal/obs"
)

func (a *App) routes(cons *console.Console, partnerAPI *api.API, hooks *inbound.Handler, prov providerSet) http.Handler {
	r := chi.NewRouter()
	r.Use(obs.Middleware(a.log))
	r.Get("/healthz", a.health)
	cons.Routes(r)
	r.Mount("/v1", partnerAPI.Routes())
	hooks.Routes(r)
	if prov.swiftpaySandbox != nil {
		r.Mount("/_sandbox/swiftpay", prov.swiftpaySandbox.Routes())
	}
	if prov.momoSandbox != nil {
		r.Mount("/_sandbox/momobridge", prov.momoSandbox.Routes())
	}
	if a.cfg.DevTools {
		devtools.New(a.store, a.clock, a.log, prov.swiftpaySandbox, prov.momoSandbox).Routes(r)
	}
	return r
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status, code := "ok", http.StatusOK
	if err := a.store.Pool.Ping(ctx); err != nil {
		status, code = "database unavailable", http.StatusServiceUnavailable
	}
	httpx.WriteJSON(w, code, map[string]string{"status": status, "time": time.Now().UTC().Format(time.RFC3339)})
}
