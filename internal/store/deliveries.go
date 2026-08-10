package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// FailedDelivery is a partner webhook that ran out of attempts.
type FailedDelivery struct {
	ID            int64
	EndpointID    int64
	PartnerID     string
	PartnerName   string
	URL           string
	EventID       string
	EventType     string
	PayoutID      string
	Attempts      int32
	LastStatus    int32
	LastError     string
	LastAttemptAt time.Time
}

// ListFailedDeliveries returns failed deliveries, newest first. An empty
// partnerID means all partners.
func (s *Store) ListFailedDeliveries(ctx context.Context, partnerID string, limit int) ([]FailedDelivery, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.id, d.endpoint_id, pr.id, pr.name, ep.url, e.id, e.type, COALESCE(e.payout_id, ''),
		       d.attempts, d.last_status_code, d.last_error, COALESCE(d.last_attempt_at, d.created_at)
		FROM webhook_deliveries d
		JOIN webhook_events e ON e.id = d.event_id
		JOIN webhook_endpoints ep ON ep.id = d.endpoint_id
		JOIN partners pr ON pr.id = ep.partner_id
		WHERE d.status = 'failed' AND ($1 = '' OR pr.id = $1)
		ORDER BY d.id DESC
		LIMIT $2`, partnerID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[FailedDelivery])
}

// RequeueFailed moves an endpoint's failed deliveries back to queued and
// returns their ids.
func (s *Store) RequeueFailed(ctx context.Context, endpointID int64) ([]int64, error) {
	rows, err := s.Pool.Query(ctx, `
		UPDATE webhook_deliveries SET status = 'queued'
		WHERE endpoint_id = $1 AND status = 'failed'
		RETURNING id`, endpointID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// CountFailedDeliveries counts failed deliveries per partner id.
func (s *Store) CountFailedDeliveries(ctx context.Context) (map[string]int, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT ep.partner_id, count(*)
		FROM webhook_deliveries d JOIN webhook_endpoints ep ON ep.id = d.endpoint_id
		WHERE d.status = 'failed' GROUP BY ep.partner_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}
