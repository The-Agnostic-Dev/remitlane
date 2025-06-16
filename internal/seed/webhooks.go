package seed

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/wire"
)

// webhooks seeds recent partner webhook history: Kobopay's were delivered,
// Accra Freight's failed because their staging endpoint is down.
func (g *generator) webhooks(ctx context.Context, q *db.Queries, tx pgx.Tx) error {
	type plan struct {
		partner  string
		n        int
		status   string
		code     int32
		attempts int
	}
	for _, pl := range []plan{
		{"prt_kobopay", 8, "delivered", 200, 1},
		{"prt_accrafreight", 5, "failed", 500, 5},
	} {
		eps, err := q.ListActiveEndpoints(ctx, pl.partner)
		if err != nil || len(eps) == 0 {
			return err
		}
		recent := g.recent[pl.partner]
		if len(recent) > pl.n {
			recent = recent[len(recent)-pl.n:]
		}
		for _, p := range recent {
			at := p.CompletedAt.Time
			evID := "evt_" + randHex(10)
			payload, _ := json.Marshal(map[string]any{"id": evID, "type": "payout.completed", "created_at": wire.Time(at), "data": wire.FromPayout(p)})
			if err := q.CreateEvent(ctx, db.CreateEventParams{ID: evID, PartnerID: p.PartnerID, Type: "payout.completed", PayoutID: text(p.ID), Payload: payload}); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE webhook_events SET created_at = $2 WHERE id = $1`, evID, at); err != nil {
				return err
			}
			var delivered any
			last := at.Add(time.Duration(pl.attempts*pl.attempts) * 2 * time.Second)
			lastErr := ""
			if pl.status == "delivered" {
				delivered = last
			} else {
				lastErr = "webhook endpoint answered 500"
			}
			var id int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO webhook_deliveries (event_id, endpoint_id, status, attempts, last_status_code, last_error, last_attempt_at, delivered_at, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
				evID, eps[0].ID, pl.status, pl.attempts, pl.code, lastErr, last, delivered, at).Scan(&id); err != nil {
				return err
			}
			for a := 1; a <= pl.attempts; a++ {
				when := at.Add(time.Duration(a*a) * 2 * time.Second)
				if _, err := tx.Exec(ctx, `INSERT INTO webhook_attempts (delivery_id, status_code, duration_ms, error, created_at) VALUES ($1, $2, $3, $4, $5)`,
					id, pl.code, 40+a*7, lastErr, when); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
