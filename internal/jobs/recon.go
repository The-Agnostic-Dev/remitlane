package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/mail"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/queue"
)

func (h *Handlers) recon(ctx context.Context, t *asynq.Task) error {
	var p queue.ReconPayload
	if err := decode(t, &p); err != nil {
		return err
	}
	date := p.Date
	if date == "" {
		date = h.Clock.Yesterday()
	}
	report, err := h.Recon.Build(ctx, date)
	if err != nil {
		return err
	}
	day, _ := time.Parse("2006-01-02", date)
	if err := h.Store.Q.InsertReconRun(ctx, db.InsertReconRunParams{
		BusinessDate: pgtype.Date{Time: day, Valid: true}, PayoutCount: int32(len(report.Rows)),
		UsdTotalMinor: report.USDMinor, TriggeredBy: p.Triggered,
	}); err != nil {
		return err
	}

	type cur struct{ Currency, Count, Amount, Fees string }
	data := struct {
		Date, USD, Link          string
		Count, Completed, Failed int
		Currencies               []cur
	}{
		Date: clock.DisplayDate(date), USD: money.Format(report.USDMinor, money.USD),
		Link:  fmt.Sprintf("%s/ops/reconciliation?date=%s", h.Config.BaseURL, date),
		Count: len(report.Rows), Completed: report.ByStatus["completed"], Failed: report.ByStatus["failed"],
	}
	for _, tot := range report.Totals {
		data.Currencies = append(data.Currencies, cur{
			Currency: string(tot.Currency), Count: fmt.Sprint(tot.Count),
			Amount: money.Format(tot.DestMinor, tot.Currency), Fees: money.Format(tot.FeeUSDMinor, money.USD),
		})
	}
	body, err := mail.Render("recon", data)
	if err != nil {
		return err
	}
	return h.Mail.Send(ctx, mail.Message{
		To: h.Config.Mail.ReconTo, Subject: "Reconciliation ready: " + clock.DisplayDate(date), Body: body,
	})
}
