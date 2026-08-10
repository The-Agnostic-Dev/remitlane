package outbound

import (
	"context"
	"errors"
)

// ErrNotFailed is returned when retrying a delivery that has not failed.
var ErrNotFailed = errors.New("outbound: delivery has not failed")

// Retry gives a failed delivery one more attempt. It returns how many
// deliveries were requeued.
func (s *Service) Retry(ctx context.Context, deliveryID int64) (int, error) {
	d, err := s.store.Q.GetDeliveryForSend(ctx, deliveryID)
	if err != nil {
		return 0, err
	}
	if d.Status != "failed" {
		return 0, ErrNotFailed
	}
	return s.requeue(ctx, d.EndpointID)
}

// RetryEndpoint gives every failed delivery to an endpoint one more attempt,
// for when a partner tells us their receiver is back up.
func (s *Service) RetryEndpoint(ctx context.Context, endpointID int64) (int, error) {
	return s.requeue(ctx, endpointID)
}

func (s *Service) requeue(ctx context.Context, endpointID int64) (int, error) {
	ids, err := s.store.RequeueFailed(ctx, endpointID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := s.queue.Deliver(ctx, id, true); err != nil {
			return 0, err
		}
	}
	s.log.Info("requeued deliveries", "endpoint_id", endpointID, "count", len(ids))
	return len(ids), nil
}
