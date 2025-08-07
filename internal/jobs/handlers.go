package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/config"
	"github.com/remitlane/remitlane/internal/notify"
	"github.com/remitlane/remitlane/internal/outbound"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/rates"
	"github.com/remitlane/remitlane/internal/recon"
	"github.com/remitlane/remitlane/internal/store"
)

// Handlers holds what the jobs need.
type Handlers struct {
	Store    *store.Store
	Payouts  *payouts.Service
	Webhooks *outbound.Service
	Recon    *recon.Builder
	Mail     notify.Sender
	Feed     rates.Feed
	Clock    *clock.Clock
	Config   config.Config
	Log      *slog.Logger
}

// Mux routes task types to handlers.
func (h *Handlers) Mux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.HandleFunc(queue.TypeDispatch, h.dispatch)
	mux.HandleFunc(queue.TypePoll, h.poll)
	mux.HandleFunc(queue.TypeDeliver, h.deliver)
	mux.HandleFunc(queue.TypeRecon, h.recon)
	mux.HandleFunc(queue.TypeLowBalance, h.lowBalance)
	return mux
}

func decode(t *asynq.Task, v any) error {
	if err := json.Unmarshal(t.Payload(), v); err != nil {
		return fmt.Errorf("%s: bad payload: %w: %w", t.Type(), err, asynq.SkipRetry)
	}
	return nil
}

// lastAttempt reports whether the queue will not retry this task again.
func lastAttempt(ctx context.Context) (attempt int, last bool) {
	retried, _ := asynq.GetRetryCount(ctx)
	max, _ := asynq.GetMaxRetry(ctx)
	return retried + 1, retried >= max
}

func (h *Handlers) dispatch(ctx context.Context, t *asynq.Task) error {
	var p queue.DispatchPayload
	if err := decode(t, &p); err != nil {
		return err
	}
	err := h.Payouts.Dispatch(ctx, p.PayoutID)
	if store.IsNotFound(err) {
		return fmt.Errorf("payout %s: %w", p.PayoutID, asynq.SkipRetry)
	}
	return err
}

func (h *Handlers) poll(ctx context.Context, _ *asynq.Task) error {
	n, err := h.Payouts.PollStuck(ctx, h.Config.Jobs.PollStuckAfter, 50)
	if n > 0 {
		h.Log.Info("poller updated payouts", "count", n)
	}
	return err
}

func (h *Handlers) deliver(ctx context.Context, t *asynq.Task) error {
	var p queue.DeliverPayload
	if err := decode(t, &p); err != nil {
		return err
	}
	attempt, last := lastAttempt(ctx)
	err := h.Webhooks.Deliver(ctx, p.DeliveryID, attempt, last)
	if store.IsNotFound(err) {
		return fmt.Errorf("delivery %d: %w", p.DeliveryID, asynq.SkipRetry)
	}
	var ae *outbound.AttemptError
	if errors.As(err, &ae) && !ae.Retryable() {
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}
