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
	if _, err := s.store.Q.GetDeliveryForSend(ctx, deliveryID); err != nil {
		return 0, err
	}
	ok, err := s.store.RequeueDelivery(ctx, deliveryID)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNotFailed
	}
	if err := s.queue.Deliver(ctx, deliveryID, true); err != nil {
		return 0, err
	}
	s.log.Info("requeued delivery", "delivery_id", deliveryID)
	return 1, nil
}

// RetryEndpoint gives every failed delivery to an endpoint one more attempt,
// for when a partner tells us their receiver is back up.
func (s *Service) RetryEndpoint(ctx context.Context, endpointID int64) (int, error) {
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
