package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/mail"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/queue"
	"github.com/remitlane/remitlane/internal/rates"
)

// lowBalance emails ops once per partner per business day when a balance
// falls below LOW_BALANCE_ALERT_USD.
func (h *Handlers) lowBalance(ctx context.Context, t *asynq.Task) error {
	var p queue.LowBalancePayload
	if err := decode(t, &p); err != nil {
		return err
	}
	partner, err := h.Store.Q.GetPartner(ctx, p.PartnerID)
	if err != nil {
		return err
	}
	balances, err := h.Store.PartnerBalances(ctx, p.PartnerID)
	if err != nil {
		return err
	}
	threshold := h.Config.Alerts.LowBalanceUSD * 100
	for _, b := range balances {
		usd, err := rates.ToUSD(ctx, h.Feed, money.New(b.Available, b.Currency))
		if err != nil {
			return err
		}
		if usd >= threshold {
			continue
		}
		day, _ := time.Parse("2006-01-02", p.Day)
		n, err := h.Store.Q.InsertAlert(ctx, db.InsertAlertParams{
			PartnerID: partner.ID, Kind: "low_balance:" + string(b.Currency), Day: pgtype.Date{Time: day, Valid: true},
		})
		if err != nil || n == 0 {
			return err
		}
		body, err := mail.Render("low_balance", map[string]string{
			"Partner": partner.Name, "Currency": string(b.Currency),
			"Available": money.Format(b.Available, b.Currency),
			"Threshold": money.Format(threshold, money.USD),
			"Link":      fmt.Sprintf("%s/ops/partners/%s", h.Config.BaseURL, partner.ID),
		})
		if err != nil {
			return err
		}
		if err := h.Mail.Send(ctx, mail.Message{
			To: h.Config.Mail.OpsTo, Subject: fmt.Sprintf("Low balance: %s %s", partner.Name, b.Currency), Body: body,
		}); err != nil {
			return err
		}
	}
	return nil
}
