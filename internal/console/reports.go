package console

import (
	"net/http"
	"time"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/db"
	"github.com/remitlane/remitlane/internal/limits"
	"github.com/remitlane/remitlane/internal/recon"
	"github.com/remitlane/remitlane/internal/store"
)

type dashboardPage struct {
	Counts    map[string]int
	Total     int
	Recent    []store.ConsolePayout
	Health    []store.ProviderStats
	Failed    map[string]int
	FailedAll int
	DayStart  time.Time
}

func (c *Console) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start, _ := c.clock.DayBounds(c.clock.Now())
	data := dashboardPage{DayStart: start}
	var err error
	if data.Counts, err = c.store.StatusCounts(ctx, start); err != nil {
		c.fail(w, r, err)
		return
	}
	for _, n := range data.Counts {
		data.Total += n
	}
	if data.Recent, err = c.store.RecentPayouts(ctx, 8); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Health, err = c.store.ProviderHealth(ctx, time.Now().Add(-24*time.Hour)); err != nil {
		c.fail(w, r, err)
		return
	}
	if data.Failed, err = c.store.CountFailedDeliveries(ctx); err != nil {
		c.fail(w, r, err)
		return
	}
	for _, n := range data.Failed {
		data.FailedAll += n
	}
	c.renderTitled(w, r, "dashboard", "Dashboard", data)
}

type limitRow struct {
	Partner db.Partner
	Usage   limits.Usage
	Percent int
}

func (c *Console) limitsPage(w http.ResponseWriter, r *http.Request) {
	ps, err := c.store.Q.ListPartners(r.Context())
	if err != nil {
		c.fail(w, r, err)
		return
	}
	rows := make([]limitRow, 0, len(ps))
	for _, p := range ps {
		u, err := c.limits.Usage(r.Context(), p)
		if err != nil {
			c.fail(w, r, err)
			return
		}
		pct := 0
		if u.LimitMinor > 0 {
			pct = int(u.UsedMinor * 100 / u.LimitMinor)
		}
		rows = append(rows, limitRow{Partner: p, Usage: u, Percent: pct})
	}
	c.renderTitled(w, r, "limits", "Daily limits", rows)
}

type reconPage struct {
	Date   string
	Report recon.Report
	Runs   []db.ReconRun
	Shown  int
}

func (c *Console) reconDate(r *http.Request) string {
	d := r.FormValue("date")
	if !clock.ValidDate(d) || d > c.clock.Today() {
		d = c.clock.Yesterday()
	}
	return d
}

func (c *Console) reconPage(w http.ResponseWriter, r *http.Request) {
	date := c.reconDate(r)
	rep, err := c.recon.Build(r.Context(), date)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	runs, err := c.store.Q.ListReconRuns(r.Context(), 10)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	shown := min(len(rep.Rows), 200)
	c.renderTitled(w, r, "recon", "Reconciliation", reconPage{Date: date, Report: rep, Runs: runs, Shown: shown})
}

func (c *Console) reconCSV(w http.ResponseWriter, r *http.Request) {
	date := c.reconDate(r)
	rep, err := c.recon.Build(r.Context(), date)
	if err != nil {
		c.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+recon.Filename(date)+`"`)
	if err := recon.WriteCSV(w, rep, c.clock.Location()); err != nil {
		c.log.Error("write csv", "err", err)
	}
}

func (c *Console) reconRun(w http.ResponseWriter, r *http.Request) {
	date := c.reconDate(r)
	if err := c.queue.Recon(r.Context(), date, "console:"+userFrom(r.Context()).Email); err != nil {
		c.fail(w, r, err)
		return
	}
	setFlash(w, "ok", "Reconciliation for "+clock.DisplayDate(date)+" queued. Finance gets the email in a minute.")
	http.Redirect(w, r, "/ops/reconciliation?date="+date, http.StatusSeeOther)
}
