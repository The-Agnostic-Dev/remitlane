// Package api is the partner REST API under /v1. Partners authenticate with
// a bearer API key; amounts are integers in minor units.
package api

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"golang.org/x/time/rate"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/store"
)

// API serves the partner endpoints.
type API struct {
	store   *store.Store
	quotes  *quotes.Service
	payouts *payouts.Service
	limits  *limits.Service
	clock   *clock.Clock
	log     *slog.Logger

	perMinute int
	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
}

// Deps are the API's collaborators.
type Deps struct {
	Store         *store.Store
	Quotes        *quotes.Service
	Payouts       *payouts.Service
	Limits        *limits.Service
	Clock         *clock.Clock
	Log           *slog.Logger
	RatePerMinute int
}

// New returns the API.
func New(d Deps) *API {
	return &API{
		store: d.Store, quotes: d.Quotes, payouts: d.Payouts, limits: d.Limits, clock: d.Clock,
		log: d.Log.With("component", "api"), perMinute: d.RatePerMinute, limiters: map[string]*rate.Limiter{},
	}
}

// Routes returns the /v1 router.
func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(a.authenticate, a.rateLimit)
	r.Get("/corridors", a.listCorridors)
	r.Post("/quotes", a.createQuote)
	r.Get("/quotes/{id}", a.getQuote)
	r.With(a.idempotent).Post("/payouts", a.createPayout)
	r.With(a.idempotent).Post("/payouts/bulk", a.createBulk)
	r.Get("/payouts", a.listPayouts)
	r.Get("/payouts/{id}", a.getPayout)
	r.Get("/batches/{id}", a.getBatch)
	r.Get("/balances", a.listBalances)
	r.Get("/limits", a.getLimits)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	return r
}

func (a *API) limiter(partnerID string) *rate.Limiter {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.limiters[partnerID]
	if !ok {
		per := float64(a.perMinute) / 60
		l = rate.NewLimiter(rate.Limit(per), max(a.perMinute/10, 10))
		a.limiters[partnerID] = l
	}
	return l
}

func (a *API) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.perMinute > 0 && !a.limiter(partnerFrom(r.Context()).ID).Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests; slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}
