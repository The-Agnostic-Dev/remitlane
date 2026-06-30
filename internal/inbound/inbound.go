// Package inbound receives provider webhooks. Every body is recorded in
// provider_events before it is acted on, signed or not.
package inbound

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/httpx"
	"github.com/remitlane/remitlane/internal/payouts"
	"github.com/remitlane/remitlane/internal/store"
)

// Handler serves /webhooks/{provider}.
type Handler struct {
	store          *store.Store
	payouts        *payouts.Service
	log            *slog.Logger
	swiftpaySecret string
	momoSecret     string
}

// New returns a Handler.
func New(s *store.Store, p *payouts.Service, log *slog.Logger, swiftpaySecret, momoSecret string) *Handler {
	return &Handler{store: s, payouts: p, log: log.With("component", "inbound"), swiftpaySecret: swiftpaySecret, momoSecret: momoSecret}
}

// Routes mounts the webhook endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/webhooks/swiftpay", h.swiftpay)
	r.Post("/webhooks/momobridge", h.momobridge)
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, httpx.MaxBody))
}

func (h *Handler) record(ctx context.Context, provider, ref, status string, sigOK bool, outcome string, body []byte) {
	if err := h.store.Q.InsertProviderEvent(ctx, db.InsertProviderEventParams{
		Provider: provider, Reference: ref, Status: status, SignatureOk: sigOK, Outcome: outcome, Body: string(body),
	}); err != nil {
		h.log.Error("record provider event", "provider", provider, "err", err)
	}
}

// apply hands a provider update to the payout service and says what happened,
// for the provider_events log.
func (h *Handler) apply(ctx context.Context, payoutID string, u payouts.Update) string {
	_, err := h.payouts.Apply(ctx, payoutID, u)
	switch {
	case err == nil:
		return "applied"
	case errors.Is(err, payouts.ErrIgnored):
		return "ignored"
	case store.IsNotFound(err):
		return "unknown_reference"
	default:
		h.log.Error("apply provider update", "payout_id", payoutID, "err", err)
		return "error"
	}
}

func ack(w http.ResponseWriter, outcome string) {
	status := http.StatusOK
	if outcome == "error" {
		status = http.StatusInternalServerError
	}
	httpx.WriteJSON(w, status, map[string]string{"received": "true", "outcome": outcome})
}
