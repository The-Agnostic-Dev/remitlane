// Package recon builds the daily reconciliation report finance uses to match
// our payouts against provider statements. A report covers the payouts
// created on one business date (Africa/Lagos).
package recon

import (
	"context"
	"errors"
	"sort"

	"github.com/remitlane/remitlane/internal/clock"
	"github.com/remitlane/remitlane/internal/money"
	"github.com/remitlane/remitlane/internal/store"
)

// CurrencyTotal sums a report's payouts in one destination currency.
type CurrencyTotal struct {
	Currency    money.Currency
	Count       int
	Completed   int
	DestMinor   int64
	USDMinor    int64
	FeeUSDMinor int64
}

// Report is the reconciliation for one date.
type Report struct {
	Date      string
	Rows      []store.ReconRow
	Totals    []CurrencyTotal
	ByStatus  map[string]int
	USDMinor  int64
	FeesMinor int64
}

// ErrFutureDate is returned for dates after today.
var ErrFutureDate = errors.New("recon: date is in the future")

// Builder builds reports.
type Builder struct {
	store *store.Store
	clock *clock.Clock
}

// NewBuilder returns a Builder.
func NewBuilder(s *store.Store, c *clock.Clock) *Builder { return &Builder{store: s, clock: c} }

// Build builds the report for a "2006-01-02" business date.
func (b *Builder) Build(ctx context.Context, date string) (Report, error) {
	if date > b.clock.Today() {
		return Report{}, ErrFutureDate
	}
	rows, err := b.store.ReconRows(ctx, date)
	if err != nil {
		return Report{}, err
	}
	return summarise(date, rows), nil
}

func summarise(date string, rows []store.ReconRow) Report {
	r := Report{Date: date, Rows: rows, ByStatus: map[string]int{}}
	totals := map[money.Currency]*CurrencyTotal{}
	for _, row := range rows {
		r.ByStatus[row.Status]++
		if row.Status == "failed" {
			continue
		}
		c := money.Currency(row.DestCurrency)
		t, ok := totals[c]
		if !ok {
			t = &CurrencyTotal{Currency: c}
			totals[c] = t
		}
		t.Count++
		if row.Status == "completed" {
			t.Completed++
		}
		t.DestMinor += row.DestMinor
		t.USDMinor += row.USDMinor
		if row.SourceCur == string(money.USD) {
			t.FeeUSDMinor += row.FeeMinor
			r.FeesMinor += row.FeeMinor
		}
		r.USDMinor += row.USDMinor
	}
	for _, t := range totals {
		r.Totals = append(r.Totals, *t)
	}
	sort.Slice(r.Totals, func(i, j int) bool { return r.Totals[i].Currency < r.Totals[j].Currency })
	return r
}
