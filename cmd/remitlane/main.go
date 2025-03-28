package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/api"
	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/migrate"
	"github.com/remitlane/remitlane/internal/obs"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	log := obs.NewLogger(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	ctx := context.Background()

	clk, err := clock.New(cfg.BusinessTZ)
	if err != nil {
		log.Error("clock", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	if err := migrate.Up(ctx, st.Pool, log); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	feed, err := rates.NewFixedFeed(cfg.Rates)
	if err != nil {
		log.Error("rates", "err", err)
		os.Exit(1)
	}
	quoter := rates.NewQuoter(feed, clk, cfg.QuoteTTL)

	// TODO: payouts service, worker
	partnerAPI := api.New(api.Deps{Store: st, Quotes: quotes.New(st, quoter, clk), Clock: clk, Log: log, RatePerMinute: cfg.APIRatePerMin})

	r := chi.NewRouter()
	r.Use(obs.Middleware(log))
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	r.Mount("/v1", partnerAPI.Routes())

	log.Info("listening", "port", cfg.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), r); err != nil {
		log.Error("serve", "err", err)
	}
}
