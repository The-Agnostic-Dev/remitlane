// Package jobs runs Remitlane's background work on asynq: payout dispatch,
// status polling, partner webhook delivery, the nightly reconciliation and
// balance alerts.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/queue"
)

// NewServer returns the asynq worker server.
func NewServer(r asynq.RedisConnOpt, cfg config.Config, log *slog.Logger) *asynq.Server {
	return asynq.NewServer(r, asynq.Config{
		Concurrency:              cfg.Jobs.Concurrency,
		Queues:                   map[string]int{queue.Critical: 6, queue.Default: 3, queue.Low: 1},
		RetryDelayFunc:           retryDelay(cfg),
		DelayedTaskCheckInterval: cfg.Jobs.DelayedCheck,
		ShutdownTimeout:          cfg.ShutdownTimeout,
		Logger:                   asynqLogger{log.With("component", "asynq")},
		LogLevel:                 asynq.WarnLevel,
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, t *asynq.Task, err error) {
			log.Warn("job failed", "type", t.Type(), "err", err)
		}),
	})
}

// retryDelay backs dispatch off exponentially and webhooks linearly. Base
// delays are short in development so the sandbox feels quick.
func retryDelay(cfg config.Config) asynq.RetryDelayFunc {
	return func(n int, err error, t *asynq.Task) time.Duration {
		switch t.Type() {
		case queue.TypeDispatch:
			return cfg.Jobs.DispatchBase * time.Duration(1<<min(n, 6))
		case queue.TypeDeliver:
			return cfg.Webhooks.RetryBase * time.Duration(n+1)
		}
		return asynq.DefaultRetryDelayFunc(n, err, t)
	}
}

// NewScheduler registers the periodic jobs. Cron specs run in the business
// time zone.
func NewScheduler(r asynq.RedisConnOpt, cfg config.Config, loc *time.Location, log *slog.Logger) (*asynq.Scheduler, error) {
	s := asynq.NewScheduler(r, &asynq.SchedulerOpts{
		Location: loc,
		Logger:   asynqLogger{log.With("component", "scheduler")},
		LogLevel: asynq.WarnLevel,
	})
	if _, err := s.Register(fmt.Sprintf("@every %s", cfg.Jobs.PollInterval), asynq.NewTask(queue.TypePoll, nil),
		asynq.Queue(queue.Low), asynq.Unique(cfg.Jobs.PollInterval)); err != nil {
		return nil, fmt.Errorf("jobs: register poller: %w", err)
	}
	if _, err := s.Register(cfg.Jobs.ReconCron, asynq.NewTask(queue.TypeRecon, []byte(`{"triggered_by":"schedule"}`)),
		asynq.Queue(queue.Low), asynq.Unique(time.Hour)); err != nil {
		return nil, fmt.Errorf("jobs: register reconciliation (RECON_CRON=%q): %w", cfg.Jobs.ReconCron, err)
	}
	return s, nil
}

type asynqLogger struct{ l *slog.Logger }

func (a asynqLogger) Debug(args ...interface{}) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...interface{})  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...interface{})  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...interface{}) { a.l.Error(fmt.Sprint(args...)) }
func (a asynqLogger) Fatal(args ...interface{}) { a.l.Error(fmt.Sprint(args...)) }
