// Package app wires Remitlane together: one process serves the partner API,
// the ops console and provider webhooks, and runs the background worker and
// scheduler.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"

	"github.com/remitlane/remitlane/internal/api"
	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/console"
	"github.com/remitlane/remitlane/internal/inbound"
	"github.com/remitlane/remitlane/internal/jobs"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/notify"
	"github.com/remitlane/remitlane/internal/migrate"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/quotes"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recon"
	"github.com/remitlane/remitlane/internal/seed"
	"github.com/remitlane/remitlane/internal/store"
)

// App is a running Remitlane.
type App struct {
	cfg       config.Config
	log       *slog.Logger
	store     *store.Store
	queue     *queue.Client
	redis     asynq.RedisClientOpt
	clock     *clock.Clock
	server    *http.Server
	worker    *asynq.Server
	scheduler *asynq.Scheduler
	handlers  *jobs.Handlers
}

// New builds the app: connects, migrates, seeds and wires every component.
func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	clk, err := clock.New(cfg.BusinessTZ)
	if err != nil {
		return nil, err
	}
	st, err := store.Open(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, err
	}
	if cfg.MigrateOnStart {
		if err := migrate.Up(ctx, st.Pool, log); err != nil {
			st.Close()
			return nil, err
		}
	}
	a := &App{
		cfg: cfg, log: log, store: st, clock: clk,
		redis: asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB},
	}
	a.queue = queue.NewClient(a.redis, queue.Options{DispatchRetries: cfg.Jobs.DispatchRetries, WebhookAttempts: cfg.Webhooks.MaxAttempts})

	feed, err := rates.NewFixedFeed(cfg.Rates)
	if err != nil {
		return nil, err
	}
	quoter := rates.NewQuoter(feed, clk, cfg.QuoteTTL)
	prov := buildProviders(cfg, log)
	events := outbound.New(st, a.queue, newClient(cfg.Webhooks.Timeout), log, cfg.Webhooks.MaxAttempts)
	lim := limits.New(st, clk)
	payoutSvc := payouts.New(payouts.Deps{
		Store: st, Quoter: quoter, Feed: feed, Limits: lim, Events: events, Queue: a.queue,
		Providers: prov.registry, Clock: clk, Log: log, MaxBulk: cfg.MaxBulkItems,
	})
	mailer, err := notify.New(cfg.Mail.Provider, st.Q, log)
	if err != nil {
		return nil, err
	}
	builder := recon.NewBuilder(st, clk)

	cons, err := console.New(console.Deps{
		Store: st, Limits: lim, Webhooks: events, Recon: builder, Queue: a.queue, Clock: clk, Config: cfg, Log: log,
		Swiftpay: prov.swiftpaySandbox, MoMo: prov.momoSandbox,
	})
	if err != nil {
		return nil, err
	}
	partnerAPI := api.New(api.Deps{
		Store: st, Quotes: quotes.New(st, quoter, clk), Payouts: payoutSvc, Limits: lim, Clock: clk, Log: log,
		RatePerMinute: cfg.APIRatePerMin,
	})
	hooks := inbound.New(st, payoutSvc, log, cfg.Swiftpay.WebhookSecret, cfg.MoMo.WebhookSecret)

	a.handlers = &jobs.Handlers{
		Store: st, Payouts: payoutSvc, Webhooks: events, Recon: builder, Mail: mailer, Feed: feed,
		Clock: clk, Config: cfg, Log: log,
	}
	a.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           a.routes(cons, partnerAPI, hooks, prov),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}

	if cfg.SeedOnStart {
		if err := seed.Run(ctx, st, seed.Options{
			BaseURL: cfg.BaseURL, Clock: clk, Quoter: quoter, Feed: feed, Hash: console.HashPassword, Log: log,
		}); err != nil {
			return nil, fmt.Errorf("seed: %w", err)
		}
		a.resetQueues()
	}
	return a, nil
}

// resetQueues drops jobs left in Redis from before the reseed; they point at
// payouts that no longer exist.
func (a *App) resetQueues() {
	insp := asynq.NewInspector(a.redis)
	defer insp.Close()
	for _, q := range []string{queue.Critical, queue.Default, queue.Low} {
		_, _ = insp.DeleteAllPendingTasks(q)
		_, _ = insp.DeleteAllScheduledTasks(q)
		_, _ = insp.DeleteAllRetryTasks(q)
		_, _ = insp.DeleteAllArchivedTasks(q)
	}
}

// Run serves until ctx is cancelled, then shuts down gracefully.
func (a *App) Run(ctx context.Context) error {
	a.worker = jobs.NewServer(a.redis, a.cfg, a.log)
	if err := a.worker.Start(a.handlers.Mux()); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	sched, err := jobs.NewScheduler(a.redis, a.cfg, a.clock.Location(), a.log)
	if err != nil {
		return err
	}
	a.scheduler = sched
	if err := a.scheduler.Start(); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}

	errc := make(chan error, 1)
	go func() {
		a.log.Info("remitlane listening", "addr", a.server.Addr, "env", a.cfg.Env, "base_url", a.cfg.BaseURL)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case <-ctx.Done():
	case err := <-errc:
		if err != nil {
			return err
		}
	}
	a.log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.cfg.ShutdownTimeout)
	defer cancel()
	err = a.server.Shutdown(shutdownCtx)
	a.scheduler.Shutdown()
	a.worker.Shutdown()
	_ = a.queue.Close()
	a.store.Close()
	return err
}
