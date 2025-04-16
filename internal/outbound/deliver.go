package outbound

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
)

// AttemptError is a delivery attempt the partner did not accept.
type AttemptError struct {
	Status int
	Err    error
}

func (e *AttemptError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("webhook delivery failed: %v", e.Err)
	}
	return fmt.Sprintf("webhook endpoint answered %d", e.Status)
}

// Retryable reports whether another attempt may succeed. Client errors such
// as 400, 404 or 410 mean the endpoint will never take this event.
func (e *AttemptError) Retryable() bool {
	return e.Status < 400 || e.Status >= 500 || e.Status == http.StatusTooManyRequests || e.Status == http.StatusRequestTimeout
}

// Deliver makes one attempt at a delivery. final says whether this is the
// last attempt the queue will make, so a failure marks the delivery failed.
func (s *Service) Deliver(ctx context.Context, deliveryID int64, attempt int, final bool) error {
	d, err := s.store.Q.GetDeliveryForSend(ctx, deliveryID)
	if err != nil {
		return err
	}
	if d.Status == "delivered" {
		return nil
	}

	start := time.Now()
	status, sendErr := s.send(ctx, d, attempt)
	ms := int32(time.Since(start).Milliseconds())
	if err := s.store.Q.RecordAttempt(ctx, db.RecordAttemptParams{
		DeliveryID: d.ID, StatusCode: int32(status), DurationMs: ms, Error: errString(sendErr),
	}); err != nil {
		s.log.Error("record attempt", "delivery_id", d.ID, "err", err)
	}

	if sendErr == nil && status == http.StatusOK {
		return s.store.Q.MarkDelivered(ctx, db.MarkDeliveredParams{ID: d.ID, LastStatusCode: int32(status)})
	}

	aerr := &AttemptError{Status: status, Err: sendErr}
	next := "retrying"
	if final || !aerr.Retryable() {
		next = "failed"
	}
	if err := s.store.Q.MarkAttemptFailed(ctx, db.MarkAttemptFailedParams{
		ID: d.ID, Status: next, LastStatusCode: int32(status), LastError: aerr.Error(),
	}); err != nil {
		return err
	}
	s.log.Warn("webhook attempt failed", "delivery_id", d.ID, "event", d.EventID, "status", status, "attempt", attempt, "next", next)
	return aerr
}

func (s *Service) send(ctx context.Context, d db.GetDeliveryForSendRow, attempt int) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Url, bytes.NewReader(d.Payload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Remitlane-Webhooks/1.4")
	req.Header.Set("Remitlane-Event-Id", d.EventID)
	req.Header.Set("Remitlane-Event-Type", d.Type)
	req.Header.Set(SignatureHeader, Sign(d.Secret, time.Now(), d.Payload))
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
