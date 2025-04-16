// Package outbound sends signed webhooks to partners. Events are written in
// the same transaction as the change they describe; delivery happens in the
// webhook:deliver job with retries.
package outbound

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/store"
	"github.com/remitlane/remitlane/internal/wire"
)

// Event is the body partners receive.
type Event struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	CreatedAt string      `json:"created_at"`
	Data      wire.Payout `json:"data"`
}

// Service writes and delivers partner webhooks.
type Service struct {
	store       *store.Store
	queue       queue.Enqueuer
	http        *http.Client
	log         *slog.Logger
	maxAttempts int
}

// New returns a Service.
func New(s *store.Store, q queue.Enqueuer, hc *http.Client, log *slog.Logger, maxAttempts int) *Service {
	return &Service{store: s, queue: q, http: hc, log: log.With("component", "outbound"), maxAttempts: maxAttempts}
}

// Emit records an event for a payout and a delivery per active endpoint, in
// the caller's transaction. It returns the delivery ids to enqueue after
// commit.
func (s *Service) Emit(ctx context.Context, q *db.Queries, eventType string, p db.Payout) ([]int64, error) {
	ev := Event{ID: newEventID(), Type: eventType, CreatedAt: wire.Time(time.Now()), Data: wire.FromPayout(p)}
	payload, err := json.Marshal(ev)
	if err != nil {
		return nil, err
	}
	if err := q.CreateEvent(ctx, db.CreateEventParams{
		ID: ev.ID, PartnerID: p.PartnerID, Type: eventType, PayoutID: text(p.ID), Payload: payload,
	}); err != nil {
		return nil, err
	}
	endpoints, err := q.ListActiveEndpoints(ctx, p.PartnerID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(endpoints))
	for _, ep := range endpoints {
		id, err := q.CreateDelivery(ctx, db.CreateDeliveryParams{EventID: ev.ID, EndpointID: ep.ID})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// Enqueue schedules deliveries. Errors are logged: the poller of last
// resort is an ops retry from the console.
func (s *Service) Enqueue(ctx context.Context, ids []int64) {
	for _, id := range ids {
		if err := s.queue.Deliver(ctx, id, false); err != nil {
			s.log.Error("enqueue delivery", "delivery_id", id, "err", err)
		}
	}
}

func newEventID() string {
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	return "evt_" + hex.EncodeToString(b)
}
